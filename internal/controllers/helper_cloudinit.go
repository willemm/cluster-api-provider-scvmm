/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"fmt"

	"github.com/go-logr/logr"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"

	"encoding/binary"
	"net"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

type CloudInitFile struct {
	Filename string
	Contents []byte
}

func writeCloudInit(log logr.Logger, scvmmMachine *infrav1.ScvmmMachine, provider *infrav1.ScvmmProviderSpec, machineid string, sharePath string, bootstrapData, metaData, networkConfig []byte) error {
	log.V(1).Info("Writing cloud-init", "sharePath", sharePath)
	// Parse share path into hostname, sharename, path
	shareParts := strings.Split(sharePath, "\\")
	host := shareParts[2]
	share := shareParts[3]
	path := strings.Join(shareParts[4:], "/")

	log.V(1).Info("smb2 Connecting", "host", host, "port", 445)
	conn, err := net.Dial("tcp", host+":445")
	if err != nil {
		return err
	}
	defer conn.Close()
	userParts := strings.Split(provider.ScvmmUsername, "\\")

	smbCreds := &smb2.NTLMInitiator{
		User:     userParts[0],
		Password: provider.ScvmmPassword,
	}
	if len(userParts) > 1 {
		smbCreds.Domain = userParts[0]
		smbCreds.User = userParts[1]
	}
	d := &smb2.Dialer{Initiator: smbCreds}

	log.V(1).Info("smb2 Dialing", "user", provider.ScvmmUsername)
	s, err := d.Dial(conn)
	if err != nil {
		return err
	}
	defer s.Logoff()

	log.V(1).Info("smb2 Mounting share", "share", share)
	fs, err := s.Mount(share)
	if err != nil {
		return err
	}
	defer fs.Umount()
	log.V(1).Info("smb2 Creating file", "path", path)
	fh, err := fs.Create(path)
	if err != nil {
		return err
	}
	networking := scvmmMachine.Spec.Networking
	if metaData == nil {
		hostname := scvmmMachine.Spec.VMName
		domainname := ""
		if networking != nil {
			domainname = "." + networking.Domain
		}
		data := "instance-id: " + machineid + "\n" +
			"hostname: " + hostname + domainname + "\n" +
			"local-hostname: " + hostname + domainname + "\n"
		metaData = []byte(data)
	}
	if networkConfig == nil {
		if networking != nil && networking.Devices != nil {
			var data bytes.Buffer
			data.WriteString("version: 2\n" +
				"ethernets:\n")
			for slot, nwd := range networking.Devices {
				data.WriteString("  eth" + fmt.Sprint(slot) + ":\n" +
					"    addresses:\n" +
					"    - " + nwd.IPAddress + "\n" +
					"    gateway4: " + nwd.Gateway + "\n" +
					"    nameservers:\n" +
					"      addresses:\n" +
					"      - " + strings.Join(nwd.Nameservers, "\n      - ") + "\n" +
					"      search:\n" +
					"      - " + strings.Join(nwd.SearchDomains, "\n      -") + "\n")
			}

			networkConfig = data.Bytes()
		}
	}
	numFiles := 2
	if networkConfig != nil {
		numFiles = 3
	}
	files := make([]CloudInitFile, numFiles)
	files[0] = CloudInitFile{
		"meta-data",
		metaData,
	}
	files[1] = CloudInitFile{
		"user-data",
		bootstrapData,
	}
	if networkConfig != nil {
		files[2] = CloudInitFile{
			"network-config",
			networkConfig,
		}
	}
	log.V(1).Info("smb2 Writing ISO", "path", path)
	if err := writeISO9660(fh, files); err != nil {
		log.Error(err, "Writing ISO file", "host", host, "share", share, "path", path)
		fh.Close()
		fs.Remove(path)
		return err
	}
	log.V(1).Info("smb2 Closing file")
	fh.Close()
	return nil
}

func putString(buf []byte, text string) {
	const padString = "                                                                                                                                "
	copy(buf, []byte(text+padString))
}

func putU16(buf []byte, value uint16) {
	binary.LittleEndian.PutUint16(buf[0:2], value)
	binary.BigEndian.PutUint16(buf[2:4], value)
}

func putU32(buf []byte, value uint32) {
	binary.LittleEndian.PutUint32(buf[0:4], value)
	binary.BigEndian.PutUint32(buf[4:8], value)
}

func putDate(buf []byte, value time.Time) {
	if value.IsZero() {
		copy(buf[0:16], []byte("0000000000000000"))
	} else {
		copy(buf[0:16], []byte(value.UTC().Format("2006010215040500")))
	}
	buf[16] = 0
}

type isoDirent struct {
	Location      int
	Length        int
	RecordingDate time.Time
	FileFlags     byte
	Identifier    string
}

func putDirent(sector []byte, offset int, dirent *isoDirent) int {
	identLen := len(dirent.Identifier)
	totlen := (33 + identLen | 1) + 1 // Pad to even length
	if offset+totlen > 2048 {
		return -1
	}
	buf := sector[offset : offset+totlen]
	buf[0] = byte(totlen)
	putU32(buf[2:10], uint32(dirent.Location))
	putU32(buf[10:18], uint32(dirent.Length))

	year, month, day := dirent.RecordingDate.UTC().Date()
	hour, minute, second := dirent.RecordingDate.UTC().Clock()
	buf[18] = byte(year - 1900)
	buf[19] = byte(month)
	buf[20] = byte(day)
	buf[21] = byte(hour)
	buf[22] = byte(minute)
	buf[23] = byte(second)

	buf[25] = dirent.FileFlags
	putU16(buf[28:32], 1) // Volume sequence number
	buf[32] = byte(identLen)
	if identLen > 0 {
		copy(buf[33:256], []byte(dirent.Identifier))
	}
	return offset + totlen
}

func writeISO9660(fh *smb2.File, files []CloudInitFile) error {
	sector := make([]byte, 2048)
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
	putString(sector[1:6], "CD001")
	sector[7] = 1
	putString(sector[8:40], "LINUX")                                        // System identifier
	putString(sector[40:72], "cidata")                                      // Volume identifier
	putU32(sector[80:88], uint32(lastSector))                               // Volume Space Size
	putU16(sector[120:124], 1)                                              // Volume Set Size
	putU16(sector[124:128], 1)                                              // Sequence Number
	putU16(sector[128:132], 2048)                                           // Logical Block Size
	putDirent(sector, 156, &isoDirent{18, 2048, now, 2, string([]byte{0})}) // Root directory entry
	putString(sector[190:318], "")                                          // Volume Set
	putString(sector[318:446], "")                                          // Publisher
	putString(sector[446:574], "")                                          // Data Preparer
	putString(sector[574:702], "cluster-api-provider-scvmm")                // Application
	putString(sector[702:740], "")                                          // Copyright File
	putString(sector[740:776], "")                                          // Abstract File
	putString(sector[776:813], "")                                          // Bibliographic File
	putDate(sector[813:830], now)                                           // Volume Creation
	putDate(sector[830:847], now)                                           // Volume Modification
	putDate(sector[847:864], time.Time{})                                   // Volume Expiration
	putDate(sector[864:881], now)                                           // Volume Effective
	sector[881] = 1                                                         // File Structure Version
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
	curOff = putDirent(sector, curOff, &isoDirent{18, 2048, now, 2, string([]byte{0})}) // Own directory entry
	curOff = putDirent(sector, curOff, &isoDirent{18, 2048, now, 2, string([]byte{1})}) // Parent directory entry

	// Write directory entries
	fileSector := 19
	for cif := range files {
		flen := len(files[cif].Contents)
		curOff = putDirent(sector, curOff, &isoDirent{fileSector, flen, now, 0, files[cif].Filename + ";1"})
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
