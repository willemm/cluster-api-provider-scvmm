package controllers

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/google/uuid"
)

type vhdxSector []byte

func (sector vhdxSector) putU16(offset int, value uint16) {
	binary.LittleEndian.PutUint16(sector[offset:offset+2], value)
}

func (sector vhdxSector) putU32(offset int, value uint32) {
	binary.LittleEndian.PutUint32(sector[offset:offset+4], value)
}

func (sector vhdxSector) putU64(offset int, value uint64) {
	binary.LittleEndian.PutUint64(sector[offset:offset+8], value)
}

func (sector vhdxSector) putString(offset, length int, text string) {
	const padString = "                                                                                                                                "
	copy(sector[offset:offset+length], []byte(text+padString))
}

func (sector vhdxSector) putUUID(offset int, uuid uuid.UUID) {
	copy(sector[offset:offset+16], uuid[:])
}

func (sector vhdxSector) putDateTime(offset int, datetime time.Time) {
	// vhd time format: Seconds since Jan 1, 2000
	y2k := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	sector.putU32(offset, uint32(datetime.Sub(y2k).Seconds()))
}

func (sector vhdxSector) checksum() uint32 {
	chk := uint32(0)
	for _, b := range sector[0:84] {
		chk = chk + uint32(b)
	}
	return ^chk
}

func writeVHDX(fh io.WriterAt, handler CloudInitFilesystemHandler, files []CloudInitFile) error {
	_, err := handler(fh, files, 0)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	DeviceHandlers["ide"] = writeVHDX
	DeviceHandlers["scsi"] = writeVHDX
}
