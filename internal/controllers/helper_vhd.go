package controllers

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/google/uuid"
)

type vhdSector []byte

func (sector vhdSector) putU16(offset int, value uint16) {
	binary.BigEndian.PutUint16(sector[offset:offset+2], value)
}

func (sector vhdSector) putU32(offset int, value uint32) {
	binary.BigEndian.PutUint32(sector[offset:offset+4], value)
}

func (sector vhdSector) putU64(offset int, value uint64) {
	binary.BigEndian.PutUint64(sector[offset:offset+8], value)
}

func (sector vhdSector) putString(offset, length int, text string) {
	const padString = "                                                                                                                                "
	copy(sector[offset:offset+length], []byte(text+padString))
}

func (sector vhdSector) putUUID(offset int, uuid uuid.UUID) {
	copy(sector[offset:offset+16], uuid[:])
}

func (sector vhdSector) putDateTime(offset int, datetime time.Time) {
	// vhd time format: Seconds since Jan 1, 2000
	y2k := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	sector.putU32(offset, uint32(datetime.Sub(y2k).Seconds()))
}

func (sector vhdSector) checksum() uint32 {
	chk := uint32(0)
	for _, b := range sector[0:84] {
		chk = chk + uint32(b)
	}
	return ^chk
}

func writeVHD(fh io.WriterAt, handler CloudInitFilesystemHandler, files []CloudInitFile) error {
	size, err := handler(fh, files, 0)
	if err != nil {
		return err
	}
	const sectorSize = 512
	now := time.Now()
	sector := make(vhdSector, sectorSize)

	sector.putString(0, 8, "conectix")         // Cookie
	sector.putU32(8, 0x02)                     // Features
	sector.putU32(12, 0x00010000)              // File Format Version
	sector.putU64(16, 0xFFFFFFFFFFFFFFFF)      // Data Offset
	sector.putDateTime(24, now)                // Time Stamp
	sector.putString(28, 4, "capi")            // Creator Application
	sector.putU16(32, 0)                       // Creator Version Major
	sector.putU16(34, 3)                       // Creator Version Minor
	sector.putString(36, 4, "lnux")            // Creator Host OS
	sector.putU64(40, uint64(size))            // Original Size
	sector.putU64(48, uint64(size))            // Current Size
	sector.putU16(56, uint16(size/sectorSize)) // Disk Geometry Cylinder
	sector[58] = 1                             // Disk Geometry Heads
	sector[59] = 1                             // Disk Geometry Sectors/track
	sector.putU32(60, 2)                       // Disk Type (Fixed)
	sector.putUUID(68, uuid.New())             // Unique Id
	sector[84] = 0                             // Saved state

	sector.putU32(64, sector.checksum())

	offset := (((size - 1) / sectorSize) + 1) * sectorSize

	if _, err := fh.WriteAt(sector, int64(offset)); err != nil {
		return err
	}
	return nil
}

func init() {
	DeviceHandlers["vhd"] = writeVHD
}
