package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

type CloudInitFile struct {
	Filename  string
	Contents  []byte
	Filestart uint32
}

func writeVFAT(fh *os.File, files []CloudInitFile) error {
	const sectorSize = 256
	sector := make([]byte, sectorSize)
	//now := time.Now()

	dirents := 1    // Volume identifier is first entry
	lastSector := 2 // Boot sector plus FAT
	for cif := range files {
		// Round up to sector size
		fsz := ((len(files[cif].Contents) - 1) / sectorSize) + 1
		lastSector = lastSector + fsz
		// One dirent needed for every 13 characters in the filename
		nsz := ((len(files[cif].Filename) - 1) / 13) + 1 + 1
		dirents = dirents + nsz
	}
	// Round up to multiple of the sector size (times 32)
	dirents = (((dirents*32 - 1) / sectorSize) + 1) * sectorSize
	lastSector = lastSector + dirents/sectorSize

	putBytes(sector[0:3], 0xeb, 0x3c, 0x90)   // Jump instruction
	putString(sector[3:11], "MSWIN4.1")       // OEM Name
	putU16(sector[11:13], 256)                // Bytes per sector
	sector[13] = 1                            // Sectors per cluster
	putU16(sector[14:16], 1)                  // Reserved sectors
	sector[16] = 1                            // Number of FATs
	putU16(sector[17:19], uint16(dirents/32)) // Number of root entries
	putU16(sector[19:21], uint16(lastSector)) // Total logical sectors
	sector[21] = 0xf8                         // Media descriptor = Fixed disk
	putU16(sector[22:24], 1)                  // Sectors per FAT
	putU16(sector[24:26], 1)                  // sectors per track
	putU16(sector[26:28], 1)                  // number of heads
	putU32(sector[28:32], 0)                  // Hidden sectors
	sector[38] = 0x29                         // Extended boot signature
	putU32(sector[39:43], 0x00C1DA7A)         // Serial number
	putString(sector[43:54], "cidata")        // Volume identifier
	putString(sector[54:62], "FAT12")         // File system type

	if _, err := fh.Write(sector); err != nil {
		return err
	}

	// Create FAT
	sector = make([]byte, sectorSize)
	fatOff := uint32(0)
	fatOff = putFATEnt(sector, fatOff, 0xFFFFFFF8, 12) // FAT ID
	fatOff = putFATEnt(sector, fatOff, 0xFFFFFFFF, 12) // end of chain marker
	// fatOff = putFATChain(sector, fatOff, sectorSize, uint32(dirents), 12)
	for cif := range files {
		files[cif].Filestart = fatOff
		fatOff = putFATChain(sector, fatOff, sectorSize, uint32(len(files[cif].Contents)), 12)
	}

	if _, err := fh.Write(sector); err != nil {
		return err
	}

	sector = make([]byte, dirents)
	curOff := 0
	curOff = putDirent(sector, curOff, "CIDATA", 0x08)

	for cif := range files {
		flen := uint32(len(files[cif].Contents))
		curOff = putVfatDirent(sector, curOff, files[cif].Filename, files[cif].Filestart, flen)
	}

	if _, err := fh.Write(sector); err != nil {
		return err
	}

	sector = make([]byte, sectorSize)
	for cif := range files {
		if _, err := fh.Write(files[cif].Contents); err != nil {
			return err
		}
		padlen := (sectorSize - (len(files[cif].Contents) % sectorSize)) % sectorSize
		if padlen > 0 {
			if _, err := fh.Write(sector[:padlen]); err != nil {
				return err
			}
		}
	}

	return nil
}

func putU16(buf []byte, value uint16) {
	binary.LittleEndian.PutUint16(buf[0:2], value)
}

func putU32(buf []byte, value uint32) {
	binary.LittleEndian.PutUint32(buf[0:4], value)
}

func putDate(buf []byte, value time.Time) {
	if value.IsZero() {
		copy(buf[0:16], []byte("0000000000000000"))
	} else {
		copy(buf[0:16], []byte(value.UTC().Format("2006010215040500")))
	}
	buf[16] = 0
}

func putString(buf []byte, text string) {
	const padString = "                                                                                                                                "
	copy(buf, []byte(text+padString))
}

func putBytes(buf []byte, bytes ...byte) {
	copy(buf, bytes)
}

func putFATEnt(sector []byte, offset, entry, fatsize uint32) uint32 {
	byteoff := (offset * fatsize) / 8
	bitoff := (offset * fatsize) % 8
	bitstodo := fatsize

	// fmt.Printf("Write FAT entry: offset = %d, byteoff = %d, bitoff = %d\n", offset, byteoff, bitoff)
	if bitoff > 0 {
		sector[byteoff] = sector[byteoff] | uint8((entry&((1<<(8-bitoff))-1))<<bitoff)
		// fmt.Printf("sector[%d], %d bits = %b, result %b\n", byteoff, bitoff, uint8(entry&(1<<byteoff-1)), sector[byteoff])
		byteoff = byteoff + 1
		entry = entry >> (8 - bitoff)
		bitstodo = bitstodo - (8 - bitoff)
	}
	for bitstodo >= 8 {
		sector[byteoff] = uint8(entry)
		// fmt.Printf("sector[%d] = %b, todo = %d\n", byteoff, sector[byteoff], bitstodo)
		byteoff = byteoff + 1
		entry = entry >> 8
		bitstodo = bitstodo - 8
	}
	if bitstodo > 0 {
		sector[byteoff] = uint8(entry & ((1 << bitstodo) - 1))
		// fmt.Printf("sector[%d] = %b, bits = %d\n", byteoff, sector[byteoff], bitstodo)
	}

	return offset + 1
}

func putFATChain(sector []byte, offset, sectorSize, flen, fatsize uint32) uint32 {
	for flen > sectorSize {
		offset = putFATEnt(sector, offset, offset+1, fatsize)
		flen = flen - sectorSize
	}
	return putFATEnt(sector, offset, 0xFFFFFFFF, fatsize)
}

func putVfatDirent(sector []byte, offset int, name string, start, flen uint32) int {
	// TODO: File extensions
	shortname := strings.ToUpper(name[0:6] + "~1")
	if len(name) <= 8 {
		shortname = strings.ToUpper(name)
	}
	namechecksum := uint8(0)
	for i := 0; i < 11; i++ {
		chr := uint8(0x20)
		if i < len(shortname) {
			chr = uint8(shortname[i])
		}
		namechecksum = ((namechecksum & 1) << 7) + (namechecksum >> 1) + chr
	}

	first := uint8(0x40)
	for nameseq := (len(name)-1)/13 + 1; nameseq > 0; nameseq = nameseq - 1 {
		sector[offset] = first | uint8(nameseq)
		chunklen := len(name) - 13*(nameseq-1)
		for i := 0; i < 5 && i < chunklen; i++ {
			sector[offset+1+i*2] = name[13*(nameseq-1)+i]
		}
		for i := 0; i < 6 && (i+5) < chunklen; i++ {
			sector[offset+14+i*2] = name[13*(nameseq-1)+i+5]
		}
		for i := 0; i < 2 && (i+11) < chunklen; i++ {
			sector[offset+28+i*2] = name[13*(nameseq-1)+i+11]
		}
		sector[offset+11] = 0x0f
		sector[offset+13] = namechecksum
		offset = offset + 32
		first = 0
	}

	putString(sector[offset:offset+11], shortname)
	putU16(sector[offset+20:offset+22], uint16(start>>16))
	putU16(sector[offset+26:offset+28], uint16(start))
	putU32(sector[offset+28:offset+32], uint32(flen))
	return offset + 32
}

func putDirent(sector []byte, offset int, name string, attributes uint8) int {
	putString(sector[offset:offset+11], name)
	sector[offset+11] = attributes
	return offset + 32
}

func main() {
	files := make([]CloudInitFile, 0)

	data := "instance-id: 0\n" +
		"hostname: localhost\n" +
		"local-hostname: localhost\n"
	files = append(files, CloudInitFile{
		Filename: "meta-data",
		Contents: []byte(data),
	})
	data = "version: 2\n" +
		"ethernets:\n" +
		"  eth0:\n" +
		"  addresses:\n" +
		"  - 127.0.0.1\n"
	files = append(files, CloudInitFile{
		Filename: "network-config",
		Contents: []byte(data),
	})
	userdata, err := os.ReadFile("user-data")
	if err != nil {
		panic(err)
	}
	files = append(files, CloudInitFile{
		Filename: "user-data",
		Contents: []byte(userdata),
	})

	fh, err := os.Create("test.vfat")
	if err != nil {
		panic(err)
	}
	defer fh.Close()
	writeVFAT(fh, files)
}

func testTimeout() {
	c1 := make(chan string, 1)
	go func() {
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
		c1 <- "Tock"
	}()
	select {
	case res := <-c1:
		fmt.Println(res)
	case <-time.After(2 * time.Second):
		fmt.Println("Timeout")
	}
}
