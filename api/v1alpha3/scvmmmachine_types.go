/*


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

package v1alpha3

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// ScvmmMachineSpec defines the desired state of ScvmmMachine
type ScvmmMachineSpec struct {
	// ProviderID is scvmm plus bios-guid
	// +optional
	ProviderID string `json:"providerID,omitempty"`
	// VMM cloud to run VM on
	Cloud string `json:"cloud"`
	// Host Group to run VM in
	HostGroup string `json:"hostGroup"`
	// Name of the VM
	// +optional
	VMName string `json:"vmName,omitempty"`
	// VM template to use
	// +optional
	VMTemplate string `json:"vmTemplate,omitempty"`
	// Virtual Harddisk to use
	// +optional
	VHDisk string `json:"vhDisk,omitempty"`
	// Disk size in gigabytes
	DiskSize *resource.Quantity `json:"diskSize"`
	// Number of CPU's
	CPUCount int `json:"cpuCount"`
	// Memory in Megabytes
	Memory *resource.Quantity `json:"memory"`
	// Virtual Network identifier
	VMNetwork string `json:"vmNetwork"`
	// Hardware profile
	HardwareProfile string `json:"hardwareProfile"`
	// Description
	// +optional
	Description string `json:"description,omitempty"`
	// Start Action
	// +optional
	// +kubebuilder:validation:Enum=NeverAutoTurnOnVM;AlwaysAutoTurnOnVM;TurnOnVMIfRunningWhenVSStopped
	StartAction string `json:"startAction,omitempty"`
	// Stop Action
	// +optional
	// +kubebuilder:validation:Enum=ShutdownGuestOS;TurnOffVM;SaveVM
	StopAction string `json:"stopAction,omitempty"`
	// Network settings
	// +optional
	Networking *Networking `json:"networking,omitempty"`
	// Cloud-Init data
	// This triggers the controller to create the machine without a (cluster-api) cluster
	// For testing purposes, or just for creating VMs
	// +optional
	CloudInit *CloudInit `json:"cloudInit,omitempty"`
}

type Networking struct {
	// IP Address
	IPAddress string `json:"ipAddress"`
	// Gateway
	Gateway string `json:"gateway"`
	// Nameservers
	Nameservers []string `json:"nameservers"`
	// Domain
	Domain string `json:"domain"`
}

// NoCloud cloud-init data (user-data and meta-data file contents)
// This triggers the machine to be created without a cluster
type CloudInit struct {
	// Meta-data file contents
	// +optional
	MetaData string `json:"metaData,omitempty"`
	// User-data file contents
	// +optional
	UserData string `json:"userData,omitempty"`
	// Network-config file contents
	// +optional
	NetworkConfig string `json:"networkConfig,omitempty"`
}

// ScvmmMachineStatus defines the observed state of ScvmmMachine
type ScvmmMachineStatus struct {
	// Mandatory field, is machine ready
	// +optional
	Ready bool `json:"ready,omitempty"`
	// Status string as given by SCVMM
	// +optional
	VMStatus string `json:"vmStatus,omitempty"`
	// BiosGuid as reported by SVCMM
	// +optional
	BiosGuid string `json:"biosGuid,omitempty"`
	// Creation time as given by SCVMM
	// +optional
	CreationTime metav1.Time `json:"creationTime,omitempty"`
	// Modification time as given by SCVMM
	// +optional
	ModifiedTime metav1.Time `json:"modifiedTime,omitempty"`
	// Host name of the VM
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Addresses contains the associated addresses for the virtual machine
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`
	// Conditions defines current service state of the ScvmmMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:resource:path=scvmmmachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.vmStatus",type="string",name="STATUS",description="Virtual Machine Status"
// +kubebuilder:printcolumn:JSONPath=".status.hostname",type="string",name="HOST",description="Virtual Machine Hostname",priority=1
// +kubebuilder:printcolumn:JSONPath=".status.addresses[].address",type="string",name="IP",description="Virtual Machine IP Address"
// +kubebuilder:printcolumn:JSONPath=".status.biosGuid",type="string",name="GUID",description="Virtual Machine BIOS GUID",priority=1
// +kubebuilder:printcolumn:JSONPath=".status.creationTime",type="date",name="AGE",description="Virtual Machine Creation Timestamp"

// ScvmmMachine is the Schema for the scvmmmachines API
type ScvmmMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmMachineSpec   `json:"spec,omitempty"`
	Status ScvmmMachineStatus `json:"status,omitempty"`
}

func (c *ScvmmMachine) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

func (c *ScvmmMachine) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ScvmmMachineList contains a list of ScvmmMachine
type ScvmmMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScvmmMachine{}, &ScvmmMachineList{})
}

func (in *ScvmmMachineSpec) CopyNonZeroTo(out *ScvmmMachineSpec) {
	if in.Cloud != "" {
		out.Cloud = in.Cloud
	}
	if in.HostGroup != "" {
		out.HostGroup = in.HostGroup
	}
	if in.VMName != "" {
		out.VMName = in.VMName
	}
	if in.VMTemplate != "" {
		out.VMTemplate = in.VMTemplate
	}
	if in.VHDisk != "" {
		out.VHDisk = in.VHDisk
	}
	if in.DiskSize != nil {
		out.DiskSize = in.DiskSize
	}
	if in.CPUCount != 0 {
		out.CPUCount = in.CPUCount
	}
	if in.Memory != nil {
		out.Memory = in.Memory
	}
	if in.VMNetwork != "" {
		out.VMNetwork = in.VMNetwork
	}
	if in.HardwareProfile != "" {
		out.HardwareProfile = in.HardwareProfile
	}
	if in.Description != "" {
		out.Description = in.Description
	}
	if in.StartAction != "" {
		out.StartAction = in.StartAction
	}
	if in.StopAction != "" {
		out.StopAction = in.StopAction
	}
	if in.Networking != nil {
		out.Networking = in.Networking
	}
	if in.CloudInit != nil {
		out.CloudInit = in.CloudInit
	}
}
