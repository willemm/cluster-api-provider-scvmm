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
	"context"
	"fmt"
	"io"

	"gopkg.in/yaml.v2"

	"github.com/go-logr/logr"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	"net"
	"strings"

	"github.com/hirochachacha/go-smb2"
)

type CloudInitFile struct {
	Filename string
	Contents []byte
}

type CloudInitFilesystemHandler struct {
	Writer func(io.Writer, []CloudInitFile) (int, error)
}

type CloudInitNetworkData struct {
	Version   int                          `json:"version"`
	Ethernets map[string]CloudInitEthernet `json:"ethernets"`
}

type CloudInitEthernet struct {
	Addresses   []string            `json:"addresses,omitempty"`
	Gateway4    string              `json:"gateway4,omitempty"`
	Nameservers CloudInitNameserver `json:"nameservers,omitempty"`
}

type CloudInitNameserver struct {
	Addresses []string `json:"addresses"`
	Search    []string `json:"search,omitempty"`
}

var cloudInitDeviceTypeExtensions = map[string]string{
	"":       "iso",
	"dvd":    "iso",
	"floppy": "vfd",
	"scsi":   "vhd",
	"ide":    "vhd",
}

var FilesystemHandlers = make(map[string]CloudInitFilesystemHandler)

func cloudInitPath(ctx context.Context, provider *infrav1.ScvmmProviderSpec, scvmmMachine *infrav1.ScvmmMachine) (string, error) {
	extension, ok := cloudInitDeviceTypeExtensions[provider.CloudInit.DeviceType]
	if !ok {
		return "", fmt.Errorf("Unknown devicetype " + provider.CloudInit.DeviceType)
	}
	share := provider.CloudInit.LibraryShare
	if !strings.HasPrefix(share, "\\\\") {
		res, err := sendWinrmCommand(ctrl.LoggerFrom(ctx), scvmmMachine.Spec.ProviderRef, "GetLibraryShare")
		if err != nil {
			return "", err
		}
		if res.Result == "" {
			return "", fmt.Errorf("GetLibraryShare returned nothing")
		}
		share = res.Result + "\\" + share
	}
	return share + "\\" + scvmmMachine.Spec.VMName + "_cloud-init." + extension, nil
}

func writeCloudInit(log logr.Logger, scvmmMachine *infrav1.ScvmmMachine, provider *infrav1.ScvmmProviderSpec, machineid string, sharePath string, bootstrapData, metaData, networkConfig []byte) error {
	log.V(1).Info("Writing cloud-init", "sharePath", sharePath)
	// Parse share path into hostname, sharename, path
	shareParts := strings.Split(sharePath, "\\")
	if len(shareParts) < 5 || shareParts[0] != "" || shareParts[1] != "" {
		return fmt.Errorf("malformed library share path " + sharePath)
	}
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
		if networking != nil {
			if networking.Domain == "" {
				return fmt.Errorf("missing required parameter networking.Domain")
			}
			hostname = hostname + "." + networking.Domain
		}

		metadataobj := map[string]string{}
		metadataobj["instance-id"] = machineid
		metadataobj["hostname"] = hostname
		metadataobj["local-hostname"] = hostname
		for k, v := range scvmmMachine.Spec.Metadata {
			metadataobj[k] = v
		}

		metaData, err = yaml.Marshal(metadataobj)
		if err != nil {
			return err
		}
	}
	if networkConfig == nil {
		if networking != nil && networking.Devices != nil {
			networkdata := CloudInitNetworkData{
				Version:   2,
				Ethernets: map[string]CloudInitEthernet{},
			}

			for slot, nwd := range networking.Devices {
				devicename := nwd.DeviceName
				if devicename == "" {
					devicename = fmt.Sprintf("eth%d", slot)
				}
				networkdata.Ethernets[devicename] = CloudInitEthernet{
					Addresses: nwd.IPAddresses,
					Gateway4:  nwd.Gateway,
					Nameservers: CloudInitNameserver{
						Addresses: nwd.Nameservers,
						Search:    nwd.SearchDomains,
					},
				}
			}

			networkConfig, err = yaml.Marshal(networkdata)
			if err != nil {
				return err
			}
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
		if err := WriteVHDFooter(fh, size); err != nil {
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
