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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	// Extra disks (after the VHDisk) to connect to the VM
	// +optional
	Disks []VmDisk `json:"disks,omitEmpty"`
	// Number of CPU's
	CPUCount int `json:"cpuCount"`
	// Allocated memory
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
	// Active Directory entry
	// +optional
	ActiveDirectory *ActiveDirectory `json:"activeDirectory,omitempty"`
	// Cloud-Init data
	// This triggers the controller to create the machine without a (cluster-api) cluster
	// For testing purposes, or just for creating VMs
	// +optional
	CloudInit *CloudInit `json:"cloudInit,omitempty"`
	// ProviderRef points to an ScvmmProvider instance that defines the provider settings for this cluster.
	// Will be copied from scvmmcluster if not using cloudinit
	// +optional
	ProviderRef *corev1.ObjectReference `json:"providerRef,omitEmpty"`
}

type VmDisk struct {
	// Size of the virtual disk
	// +optional
	Size *resource.Quantity `json:"size,omitEmpty"`
	// Specify that the virtual disk can expand dynamically (default: true)
	// +optional
	Dynamic bool `json:"dynamic,omitEmpty"`
	// Virtual Harddisk to couple
	// +optional
	VHDisk string `json:"vhDisk,omitempty"`
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

type ActiveDirectory struct {
	// Domain Controller
	// +optional
	DomainController string `json:"domainController,omitempty"`
	// OU Path
	OUPath string `json:"ouPath"`
	// Description
	// +optional
	Description string `json:"description,omitempty"`
	// Group memberships
	// +option
	MemberOf []string `json:"memberOf,omitempty"`
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
// +kubebuilder:printcolumn:JSONPath=".spec.providerID",type="string",name="ID",description="Virtual Machine ProviderID",priority=1
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

func (in *ScvmmMachineSpec) CopyNonZeroTo(out *ScvmmMachineSpec) bool {
	changed := false
	if in.Cloud != "" && in.Cloud != out.Cloud {
		changed = true
		out.Cloud = in.Cloud
	}
	if in.HostGroup != "" && in.HostGroup != out.HostGroup {
		changed = true
		out.HostGroup = in.HostGroup
	}
	if in.VMName != "" && in.VMName != out.VMName {
		changed = true
		out.VMName = in.VMName
	}
	if in.VMTemplate != "" && in.VMTemplate != out.VMTemplate {
		changed = true
		out.VMTemplate = in.VMTemplate
	}
	if in.Disks != nil && VmDiskEquals(in.Disks, out.Disks) {
		changed = true
		out.Disks = in.Disks
	}
	if in.CPUCount != 0 && in.CPUCount != out.CPUCount {
		changed = true
		out.CPUCount = in.CPUCount
	}
	if in.Memory != nil && in.Memory != out.Memory {
		changed = true
		out.Memory = in.Memory
	}
	if in.VMNetwork != "" && in.VMNetwork != out.VMNetwork {
		changed = true
		out.VMNetwork = in.VMNetwork
	}
	if in.HardwareProfile != "" && in.HardwareProfile != out.HardwareProfile {
		changed = true
		out.HardwareProfile = in.HardwareProfile
	}
	if in.Description != "" && in.Description != out.Description {
		changed = true
		out.Description = in.Description
	}
	if in.StartAction != "" && in.StartAction != out.StartAction {
		changed = true
		out.StartAction = in.StartAction
	}
	if in.StopAction != "" && in.StopAction != out.StopAction {
		changed = true
		out.StopAction = in.StopAction
	}
	if in.Networking != nil && !in.Networking.Equals(out.Networking) {
		changed = true
		out.Networking = in.Networking
	}
	if in.CloudInit != nil && !in.CloudInit.Equals(out.CloudInit) {
		changed = true
		out.CloudInit = in.CloudInit
	}
	return changed
}

func VmDiskEquals(left []VmDisk, right []VmDisk) bool {
	if left == nil {
		return right == nil
	}
	if len(left) != len(right) {
		return false
	}
	for i, v := range left {
		if v != right[i] {
			return false
		}
	}
	return true
}

func (left *Networking) Equals(right *Networking) bool {
	if right == nil {
		return false
	}
	if left.IPAddress != right.IPAddress ||
		left.Gateway != right.Gateway ||
		left.Domain != right.Domain ||
		len(left.Nameservers) != len(right.Nameservers) {
		return false
	}
	for i, v := range left.Nameservers {
		if v != right.Nameservers[i] {
			return false
		}
	}
	return true
}

func (left *CloudInit) Equals(right *CloudInit) bool {
	if right == nil {
		return false
	}
	return left.MetaData == right.MetaData &&
		left.UserData == right.UserData &&
		left.NetworkConfig == right.NetworkConfig
}
