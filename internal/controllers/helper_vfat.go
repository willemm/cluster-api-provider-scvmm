package controllers

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

type vfatSector []byte

func (sector vfatSector) putU16(offset int, value uint16) {
	binary.LittleEndian.PutUint16(sector[offset:offset+2], value)
}

func (sector vfatSector) putU32(offset int, value uint32) {
	binary.LittleEndian.PutUint32(sector[offset:offset+4], value)
}

func (sector vfatSector) putString(offset, length int, text string) {
	const padString = "                                                                                                                                "
	copy(sector[offset:offset+length], []byte(text+padString))
}

func (sector vfatSector) putBytes(offset int, bytes ...byte) {
	copy(sector[offset:offset+len(bytes)], bytes)
}

func (sector vfatSector) putFATEnt(offset, entry, fatsize uint32) uint32 {
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

func (sector vfatSector) putFATChain(offset, sectorSize, flen, fatsize uint32) uint32 {
	for flen > sectorSize {
		offset = sector.putFATEnt(offset, offset+1, fatsize)
		flen = flen - sectorSize
	}
	return sector.putFATEnt(offset, 0xFFFFFFFF, fatsize)
}

func (sector vfatSector) putVfatDirent(offset int, name string, start, flen uint32, ctime time.Time) int {
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

	sector.putString(offset, 11, shortname)
	sector.putDateTime(offset+14, ctime)
	sector.putU16(offset+20, uint16(start>>16))
	sector.putDateTime(offset+22, ctime)
	sector.putU16(offset+26, uint16(start))
	sector.putU32(offset+28, uint32(flen))
	return offset + 32
}

func (sector vfatSector) putDirent(offset int, name string, attributes uint8, ctime time.Time) int {
	sector.putString(offset, 11, name)
	sector[offset+11] = attributes
	sector.putDateTime(offset+14, ctime)
	sector.putDateTime(offset+22, ctime)
	return offset + 32
}

func (sector vfatSector) putDateTime(offset int, datetime time.Time) {
	// fat time format: Hour (5 bits) Minute (6 bits) Second (5 bits)
	sector.putU16(offset,
		uint16((datetime.Hour()&0x1F)<<11|
			(datetime.Minute()&0x3F)<<5|
			(datetime.Second()&0x1F)))
	// fat date format: Year (7 bits) Month (4 bits) Day (5 bits), note that year 0 means 1980
	sector.putU16(offset+2,
		uint16(((datetime.Year()-1980)&0x7F)<<9|
			(int(datetime.Month())&0x0F)<<5|
			(datetime.Day()&0x1F)))
}

func writeVFAT(fh io.WriterAt, files []CloudInitFile, offset int) (int, error) {
	const sectorSize = 512
	now := time.Now()

	// Calculate number of sectors
	dirents := 1            // Volume identifier is first entry
	lastSector := uint32(1) // Boot sector
	for cif := range files {
		// Round up to sector size
		fsz := ((len(files[cif].Contents) - 1) / sectorSize) + 1
		lastSector = lastSector + uint32(fsz)
		// One dirent needed for every 13 characters in the filename
		nsz := ((len(files[cif].Filename) - 1) / 13) + 1 + 1
		dirents = dirents + nsz
	}
	// Round up to multiple of the sector size (times 32)
	dirents = (((dirents*32 - 1) / sectorSize) + 1) * sectorSize
	lastSector = lastSector + uint32(dirents)/sectorSize

	// Determine fat type and size
	fatSize := uint32(12)
	fatSectorCount := uint32(0)
	for extraSectors := uint32(1); extraSectors > fatSectorCount; {
		// Increase lastSector and recalculate
		lastSector = lastSector + (extraSectors - fatSectorCount)
		fatSectorCount = extraSectors
		if lastSector >= 4086 {
			fatSize = 16
		}
		if lastSector >= 65525 {
			panic("TODO: fat32")
		}
		// Size in bits to bytes, rounding up
		fatSectorSize := ((fatSize*lastSector-1)/8 + 1)
		// Size in sectors, rounding up
		extraSectors = (fatSectorSize-1)/sectorSize + 1
	}
	// Allow for querying the size
	if fh == nil {
		return int(sectorSize * lastSector), nil
	}

	sector := make(vfatSector, sectorSize)

	sector.putBytes(0, 0xeb, 0x3c, 0x90)                   // Jump instruction
	sector.putString(3, 8, "MSWIN4.1")                     // OEM Name
	sector.putU16(11, sectorSize)                          // Bytes per sector
	sector[13] = 1                                         // Sectors per cluster
	sector.putU16(14, 1)                                   // Reserved sectors
	sector[16] = 1                                         // Number of FATs
	sector.putU16(17, uint16(dirents/32))                  // Number of root entries
	sector.putU16(19, uint16(lastSector))                  // Total logical sectors
	sector[21] = 0xf8                                      // Media descriptor = Fixed disk
	sector.putU16(22, uint16(fatSectorCount))              // Sectors per FAT
	sector.putU16(24, 1)                                   // sectors per track
	sector.putU16(26, 1)                                   // number of heads
	sector.putU32(28, 0)                                   // Hidden sectors
	sector[38] = 0x29                                      // Extended boot signature
	sector.putU32(39, 0x00C1DA7A)                          // Serial number
	sector.putString(43, 11, "cidata")                     // Volume identifier
	sector.putString(54, 8, fmt.Sprintf("FAT%d", fatSize)) // File system type

	n, err := fh.WriteAt(sector, int64(offset))
	if err != nil {
		return 0, err
	}
	offset += n

	// Create FAT
	fileStarts := make([]uint32, len(files))

	sector = make(vfatSector, sectorSize*fatSectorCount)

	fatOff := uint32(0)
	fatOff = sector.putFATEnt(fatOff, 0xFFFFFFF8, fatSize) // FAT ID
	fatOff = sector.putFATEnt(fatOff, 0xFFFFFFFF, fatSize) // end of chain marker
	for cif := range files {
		fileStarts[cif] = fatOff
		fatOff = sector.putFATChain(fatOff, sectorSize, uint32(len(files[cif].Contents)), fatSize)
	}

	n, err = fh.WriteAt(sector, int64(offset))
	if err != nil {
		return 0, err
	}
	offset += n

	sector = make(vfatSector, dirents)
	curOff := 0
	curOff = sector.putDirent(curOff, "CIDATA", 0x08, now)

	for cif := range files {
		flen := uint32(len(files[cif].Contents))
		curOff = sector.putVfatDirent(curOff, files[cif].Filename, fileStarts[cif], flen, now)
	}

	n, err = fh.WriteAt(sector, int64(offset))
	if err != nil {
		return 0, err
	}
	offset += n

	sector = make(vfatSector, sectorSize)
	for cif := range files {
		contents := files[cif].Contents
		_, err = fh.WriteAt(contents, int64(offset))
		if err != nil {
			return 0, err
		}
		// Align offset to sector size by rounding up
		offset += (((len(contents) - 1) / sectorSize) + 1) * sectorSize
	}

	return int(sectorSize * lastSector), nil
}

func init() {
	FilesystemHandlers["vfat"] = writeVFAT
}
