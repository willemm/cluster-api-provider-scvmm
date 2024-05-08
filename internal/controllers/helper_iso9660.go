package controllers

import (
	"encoding/binary"
	"time"

	"github.com/hirochachacha/go-smb2"
)

type isoSector []byte

func (sector isoSector) putString(offset, length int, text string) {
	const padString = "                                                                                                                                "
	copy(sector[offset:offset+length], []byte(text+padString))
}

func (sector isoSector) putBytes(offset int, bytes ...byte) {
	copy(sector[offset:offset+len(bytes)], bytes)
}

func (sector isoSector) putU16(offset int, value uint16) {
	binary.LittleEndian.PutUint16(sector[offset+0:offset+2], value)
	binary.BigEndian.PutUint16(sector[offset+2:offset+4], value)
}

func (sector isoSector) putU32(offset int, value uint32) {
	binary.LittleEndian.PutUint32(sector[offset+0:offset+4], value)
	binary.BigEndian.PutUint32(sector[offset+4:offset+8], value)
}

func (sector isoSector) putDate(offset int, value time.Time) {
	if value.IsZero() {
		copy(sector[offset+0:offset+16], []byte("0000000000000000"))
	} else {
		copy(sector[offset+0:offset+16], []byte(value.UTC().Format("2006010215040500")))
	}
	sector[offset+16] = 0
}

type isoDirent struct {
	Location      int
	Length        int
	RecordingDate time.Time
	FileFlags     byte
	Identifier    string
}

func (sector isoSector) putDirent(offset int, dirent *isoDirent) int {
	identLen := len(dirent.Identifier)
	totlen := (33 + identLen | 1) + 1 // Pad to even length
	if offset+totlen > 2048 {
		return -1
	}
	buf := sector[offset : offset+totlen]
	buf[0] = byte(totlen)
	sector.putU32(offset+2, uint32(dirent.Location))
	sector.putU32(offset+10, uint32(dirent.Length))

	year, month, day := dirent.RecordingDate.UTC().Date()
	hour, minute, second := dirent.RecordingDate.UTC().Clock()
	sector.putBytes(offset+18,
		byte(year-1900),
		byte(month),
		byte(day),
		byte(hour),
		byte(minute),
		byte(second),
	)

	sector[offset+25] = dirent.FileFlags
	sector.putU16(offset+28, 1) // Volume sequence number
	sector[offset+32] = byte(identLen)
	if identLen > 0 {
		copy(sector[offset+33:], []byte(dirent.Identifier))
	}
	return offset + totlen
}

func writeISO9660(fh *smb2.File, files []CloudInitFile) error {
	sector := make(isoSector, 2048)
	now := time.Now()

	// Calculate the total size
	// NB: Assumes all files are in the root and the dirent will not exceed one sector
	// 16,17 = volume identifiers, 18 = directory
	lastSector := 19
	for cif := range files {
		// Round up to sector size
		fsz := ((len(files[cif].Contents) - 1) / 2048) + 1
		lastSector = lastSector + fsz
	}

	// Start with 32K of zeroes
	for i := 0; i < 16; i++ {
		if _, err := fh.Write(sector); err != nil {
			return err
		}
	}
	// Write Primary Volume Descriptor (sector 16)
	sector[0] = 1
	sector.putString(1, 5, "CD001")
	sector[7] = 1
	sector.putString(8, 32, "LINUX")                                       // System identifier
	sector.putString(40, 32, "cidata")                                     // Volume identifier
	sector.putU32(80, uint32(lastSector))                                  // Volume Space Size
	sector.putU16(120, 1)                                                  // Volume Set Size
	sector.putU16(124, 1)                                                  // Sequence Number
	sector.putU16(128, 2048)                                               // Logical Block Size
	sector.putDirent(156, &isoDirent{18, 2048, now, 2, string([]byte{0})}) // Root directory entry
	sector.putString(190, 128, "")                                         // Volume Set
	sector.putString(318, 128, "")                                         // Publisher
	sector.putString(446, 128, "")                                         // Data Preparer
	sector.putString(574, 128, "cluster-api-provider-scvmm")               // Application
	sector.putString(702, 38, "")                                          // Copyright File
	sector.putString(740, 36, "")                                          // Abstract File
	sector.putString(776, 37, "")                                          // Bibliographic File
	sector.putDate(813, now)                                               // Volume Creation
	sector.putDate(830, now)                                               // Volume Modification
	sector.putDate(847, time.Time{})                                       // Volume Expiration
	sector.putDate(864, now)                                               // Volume Effective
	sector[881] = 1                                                        // File Structure Version
	if _, err := fh.Write(sector); err != nil {
		return err
	}

	for i := range sector {
		sector[i] = 0
	}
	// Write Terminator VD (sector 17)
	sector[0] = 255                    // Type
	copy(sector[1:6], []byte("CD001")) // Identifier
	sector[6] = 1                      // Version
	if _, err := fh.Write(sector); err != nil {
		return err
	}
	for i := range sector {
		sector[i] = 0
	}

	// Write directory (sector 18)
	curOff := 0
	curOff = sector.putDirent(curOff, &isoDirent{18, 2048, now, 2, string([]byte{0})}) // Own directory entry
	curOff = sector.putDirent(curOff, &isoDirent{18, 2048, now, 2, string([]byte{1})}) // Parent directory entry

	// Write directory entries
	fileSector := 19
	for cif := range files {
		flen := len(files[cif].Contents)
		curOff = sector.putDirent(curOff, &isoDirent{fileSector, flen, now, 0, files[cif].Filename + ";1"})
		fileSector = fileSector + ((flen - 1) / 2048) + 1
	}
	if _, err := fh.Write(sector); err != nil {
		return err
	}
	for i := range sector {
		sector[i] = 0
	}
	for cif := range files {
		if _, err := fh.Write(files[cif].Contents); err != nil {
			return err
		}
		padlen := (2048 - (len(files[cif].Contents) % 2048)) % 2048
		if padlen > 0 {
			if _, err := fh.Write(sector[:padlen]); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	FilesystemHandlers["iso9660"] = CloudInitFilesystemHandler{
		FileExtension: "iso",
		Writer:        writeISO9660,
	}
	// Set default fs handler
	FilesystemHandlers[""] = FilesystemHandlers["iso9660"]
}
