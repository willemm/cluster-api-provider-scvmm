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

	"net"
	"strings"

	"github.com/hirochachacha/go-smb2"
)

type CloudInitFile struct {
	Filename string
	Contents []byte
}

type CloudInitFilesystemHandler struct {
	FileExtension string
	Writer        func(*smb2.File, []CloudInitFile) (int, error)
}

var FilesystemHandlers = make(map[string]CloudInitFilesystemHandler)

func cloudInitPath(provider *infrav1.ScvmmProviderSpec, scvmmMachine *infrav1.ScvmmMachine) (string, error) {
	handler, ok := FilesystemHandlers[provider.CloudInit.FileSystem]
	if !ok {
		return "", fmt.Errorf("Unknown filesystem " + provider.CloudInit.FileSystem)
	}
	return provider.CloudInit.LibraryShare + "\\" + scvmmMachine.Spec.VMName + "_cloud-init." + handler.FileExtension, nil
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
			if networking.Domain == "" {
				return fmt.Errorf("missing required parameter networking.Domain")
			}
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
				devicename := nwd.DeviceName
				if devicename == "" {
					devicename = fmt.Sprintf("eth%d", slot)
				}
				data.WriteString("  " + devicename + ":\n")
				if len(nwd.IPAddresses) > 0 {
					data.WriteString("    addresses:\n" +
						"    - " + strings.Join(nwd.IPAddresses, "\n    - ") + "\n")
				}
				if nwd.Gateway != "" {
					data.WriteString("    gateway4: " + nwd.Gateway + "\n")
				}
				if len(nwd.Nameservers) > 0 {
					data.WriteString("    nameservers:\n" +
						"      addresses:\n" +
						"      - " + strings.Join(nwd.Nameservers, "\n      - ") + "\n")
					if len(nwd.SearchDomains) > 0 {
						data.WriteString("      search:\n" +
							"      - " + strings.Join(nwd.SearchDomains, "\n      -") + "\n")
					}
				}
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
	log.V(1).Info("smb2 Writing cloud-init", "path", path)
	handler, ok := FilesystemHandlers[provider.CloudInit.FileSystem]
	if !ok {
		return fmt.Errorf("Unknown filesystem " + provider.CloudInit.FileSystem)
	}
	size, err := handler.Writer(fh, files)
	if err != nil {
		log.Error(err, "Writing cloud-init file", "host", host, "share", share, "path", path)
		fh.Close()
		fs.Remove(path)
		return err
	}
	if provider.CloudInit.DeviceType == "scsi" || provider.CloudInit.DeviceType == "ide" {
		log.V(1).Info("smb2 add vhd footer")
		if err := writeVHDFooter(fh, size); err != nil {
			log.Error(err, "Writing cloud-init file", "host", host, "share", share, "path", path)
			fh.Close()
			fs.Remove(path)
			return err
		}
	}
	log.V(1).Info("smb2 Closing file")
	fh.Close()
	return nil
}
