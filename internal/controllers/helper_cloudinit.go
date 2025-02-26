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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/Masterminds/sprig/v3"

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

// Params: filehandle interface, file list, offset
// Returns: size of created filesystem, error
type CloudInitFilesystemHandler func(io.WriterAt, []CloudInitFile, int) (int, error)

type CloudInitDeviceHandler func(io.WriterAt, CloudInitFilesystemHandler, []CloudInitFile) error

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
	"scsi":   "vhdx",
	"ide":    "vhdx",
}

var FilesystemHandlers = make(map[string]CloudInitFilesystemHandler)
var DeviceHandlers = make(map[string]CloudInitDeviceHandler)

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

func renderHostname(scvmmMachine *infrav1.ScvmmMachine) (string, error) {
	tmplstring := scvmmMachine.Spec.Hostname
	if tmplstring == "" {
		tmplstring = "{{ .spec.vmName }}.{{ .spec.networking.domain }}"
	}
	//   "github.com/Masterminds/sprig/v3"
	hostnametmpl, err := template.New("hostname").Funcs(sprig.FuncMap()).Parse(tmplstring)
	if err != nil {
		return "", fmt.Errorf("Failed to parse hostname template: %w", err)
	}
	// Convert to json and back to get lowercase fields like in the yaml
	svmjson, err := json.Marshal(scvmmMachine)
	if err != nil {
		return "", err
	}
	svmdata := map[string]interface{}{}
	if err := json.Unmarshal(svmjson, &svmdata); err != nil {
		return "", err
	}
	hostbytes := bytes.Buffer{}
	if err := hostnametmpl.Execute(&hostbytes, svmdata); err != nil {
		return "", fmt.Errorf("Failed to execute hostname template: %w", err)
	}
	return hostbytes.String(), nil
}

func writeCloudInit(log logr.Logger, scvmmMachine *infrav1.ScvmmMachine, provider *infrav1.ScvmmProviderSpec, machineid string, sharePath string, bootstrapData, metaData, networkConfig []byte) (reterr error) {
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
	defer func() {
		fh.Close()
		if reterr != nil {
			fs.Remove(path)
		}
	}()
	networking := scvmmMachine.Spec.Networking
	if metaData == nil {
		hostname, err := renderHostname(scvmmMachine)
		if err != nil {
			return err
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
	devHandler, ok := DeviceHandlers[provider.CloudInit.DeviceType]
	if !ok {
		return fmt.Errorf("Unknown device type " + provider.CloudInit.DeviceType)
	}
	if err := devHandler(fh, handler, files); err != nil {
		log.Error(err, "Writing cloud-init file", "host", host, "share", share, "path", path)
		return err
	}
	log.V(1).Info("smb2 Done")
	return nil
}
