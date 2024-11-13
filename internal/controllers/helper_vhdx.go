package controllers

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/unicode"
)

var (
	GuidBAT            = uuid.UUID{0x66, 0x77, 0xC2, 0x2D, 0x23, 0xF6, 0x00, 0x42, 0x9D, 0x64, 0x11, 0x5E, 0x9B, 0xFD, 0x4A, 0x08}
	GuidMetadata       = uuid.UUID{0x06, 0xA2, 0x7C, 0x8B, 0x90, 0x47, 0x9A, 0x4B, 0xB8, 0xFE, 0x57, 0x5F, 0x05, 0x0F, 0x88, 0x6E}
	GuidFileParameters = uuid.UUID{0x37, 0x67, 0xA1, 0xCA, 0x36, 0xFA, 0x43, 0x4D, 0xB3, 0xB6, 0x33, 0xF0, 0xAA, 0x44, 0xE7, 0x6B}
	GuidDiskSize       = uuid.UUID{0x24, 0x42, 0xA5, 0x2F, 0x1B, 0xCD, 0x76, 0x48, 0xB2, 0x11, 0x5D, 0xBE, 0xD8, 0x3B, 0xF4, 0xB8}
	GuidDiskID         = uuid.UUID{0xAB, 0x12, 0xCA, 0xBE, 0xE6, 0xB2, 0x23, 0x45, 0x93, 0xEF, 0xC3, 0x09, 0xE0, 0x00, 0xC7, 0x46}
	GuidLogicalSector  = uuid.UUID{0x1D, 0xBF, 0x41, 0x81, 0x6F, 0xA9, 0x09, 0x47, 0xBA, 0x47, 0xF2, 0x33, 0xA8, 0xFA, 0xAB, 0x5F}
	GuidPhysicalSector = uuid.UUID{0xC7, 0x48, 0xA3, 0xCD, 0x5D, 0x44, 0x71, 0x44, 0x9C, 0xC9, 0xE9, 0x88, 0x52, 0x51, 0xC5, 0x56}
)

const sectorSize = 512
const filesystemStartMB = 4
const filesystemStart = filesystemStartMB * 1024 * 1024

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

func (sector vhdxSector) putBytes(offset int, bytes ...byte) {
	copy(sector[offset:offset+len(bytes)], bytes)
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

func encUTF16(text string) []byte {
	enc := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	bytes, _ := enc.Bytes([]byte(text))
	return bytes
}

func writeVHDXIdentifier(fh io.WriterAt, offset int64) error {
	sector := make(vhdxSector, 256)
	sector.putString(0, 8, "vhdxfile")
	sector.putBytes(8, encUTF16("cluster-api-provider-scvmm")...)
	_, err := fh.WriteAt(sector, offset)
	return err
}

func writeVHDXHeader(fh io.WriterAt, offset int64, seq uint64) error {
	sector := make(vhdxSector, 4096) // Need 4k bytes for the crc calculation
	sector.putString(0, 4, "head")   // Signature
	sector.putU64(8, seq)            // Sequence number (offset / 64Kb)
	sector.putUUID(16, uuid.New())   // FileWriteGuid
	sector.putUUID(32, uuid.New())   // DataWriteGuid
	// LogGuid = 0
	// LogVersion = 0
	sector.putU16(66, 1)           // Version = 1
	sector.putU32(68, 1*1024*1024) // LogLength = 1Mb
	sector.putU64(72, 1*1024*1024) // LogOffset = 1Mb
	checksum := crc32.Checksum(sector, crc32.MakeTable(crc32.Castagnoli))
	sector.putU32(4, checksum) // CRC32C checksum
	_, err := fh.WriteAt(sector[:256], offset)
	return err
}

func (sector vhdxSector) putRegionEntry(offset int, guid uuid.UUID, offMB uint64, lenMB uint32) {
	sector.putUUID(offset, guid)
	sector.putU64(offset+16, offMB*1024*1024)
	sector.putU32(offset+24, lenMB*1024*1024)
	// sector.putU32(offset+28, 1) // Required
}

func writeVHDXRegion(fh io.WriterAt, offset int64) error {
	sector := make(vhdxSector, 64*1024) // Need 64k bytes for the crc calculation
	// Region table header
	sector.putString(0, 4, "regi") // Signature
	sector.putU32(8, 2)            // EntryCount (BAT + metadata)
	// Entry for BAT
	sector.putRegionEntry(16, GuidBAT, 2, 1)
	sector.putRegionEntry(48, GuidMetadata, 3, 1)

	checksum := crc32.Checksum(sector, crc32.MakeTable(crc32.Castagnoli))
	sector.putU32(4, checksum) // CRC32C checksum
	_, err := fh.WriteAt(sector, offset)
	return err
}

func (sector vhdxSector) putMetadata(offset int, guid uuid.UUID, offs, length, flags int) {
	sector.putUUID(offset, guid)
	sector.putU32(offset+16, uint32(offs))
	sector.putU32(offset+20, uint32(length))
	sector.putU32(offset+24, uint32(flags))
}

func writeVHDXMetadata(fh io.WriterAt, offset int64, size int) error {
	sector := make(vhdxSector, 256) // Enough room for 5 entries

	mdoff := 64 * 1024 // Metadata entries start at 64kb

	sector.putString(0, 8, "metadata") // Signature
	sector.putU16(10, 5)               // EntryCount

	sector.putMetadata(32*1, GuidFileParameters, mdoff, 8, 4)
	sector.putMetadata(32*2, GuidDiskSize, mdoff+8, 8, 6)
	sector.putMetadata(32*3, GuidDiskID, mdoff+16, 16, 6)
	sector.putMetadata(32*4, GuidLogicalSector, mdoff+32, 4, 6)
	sector.putMetadata(32*5, GuidPhysicalSector, mdoff+36, 4, 6)
	_, err := fh.WriteAt(sector, offset)
	if err != nil {
		return err
	}
	sector = make(vhdxSector, 256)
	sector.putU32(0, 1*1024*1024)  // FileParameters (BlockSize)
	sector.putU32(4, 1)            // FileParameters (LeaveBlockAllocated|HasParent)
	sector.putU64(8, uint64(size)) // DiskSize
	sector.putUUID(16, uuid.New()) // Virtual Disk ID
	sector.putU32(32, sectorSize)  // Logical Sector Size
	sector.putU32(36, sectorSize)  // Physical Sector Size
	_, err = fh.WriteAt(sector, offset+int64(mdoff))
	return err
}

// Will most likely contain exactly just one block
func writeVHDXBAT(fh io.WriterAt, offset int64, size int) error {
	blockSize := 1 * 1024 * 1024
	numblocks := ((size - 1) / blockSize) + 1
	sector := make(vhdxSector, 8*numblocks)

	for b := 0; b < numblocks; b++ {
		// Blocks start at 4Mb
		batEntry := uint64(6) | (uint64((b + filesystemStartMB)) << 20)
		sector.putU64(b*8, batEntry)
	}
	_, err := fh.WriteAt(sector, offset)
	return err
}

func writeVHDX(fh io.WriterAt, handler CloudInitFilesystemHandler, files []CloudInitFile) error {
	// First, get the image size
	size, err := handler(nil, files, 0)
	// Round up to logical block size
	size = (((size - 1) / sectorSize) + 1) * sectorSize
	if err != nil {
		return err
	}
	if err := writeVHDXIdentifier(fh, 0); err != nil {
		return err
	}
	if err := writeVHDXHeader(fh, 64*1024, 1); err != nil {
		return err
	}
	if err := writeVHDXHeader(fh, 128*1024, 2); err != nil {
		return err
	}
	if err := writeVHDXRegion(fh, 192*1024); err != nil {
		return err
	}
	if err := writeVHDXRegion(fh, 256*1024); err != nil {
		return err
	}
	if err := writeVHDXBAT(fh, 2*1024*1024, size); err != nil {
		return err
	}
	if err := writeVHDXMetadata(fh, 3*1024*1024, size); err != nil {
		return err
	}

	hsize, err := handler(fh, files, filesystemStart)
	if err != nil {
		return err
	}
	if size > hsize {
		pad := make([]byte, (size - hsize))
		_, err = fh.WriteAt(pad, int64(filesystemStart+hsize))
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	DeviceHandlers["ide"] = writeVHDX
	DeviceHandlers["scsi"] = writeVHDX
}
