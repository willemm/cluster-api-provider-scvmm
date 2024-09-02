/*
Copyright 2024.

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

package v1alpha1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ScvmmMachineSpec defines the desired state of ScvmmMachine
type ScvmmMachineSpec struct {
	// ProviderID is scvmm plus vm-guid
	// +optional
	ProviderID string `json:"providerID,omitempty"`
	// ID is scvmm object ID, will be filled in by controller
	// +optional
	Id string `json:"id,omitempty"`
	// VMM cloud to run VM on
	// +optional
	Cloud string `json:"cloud,omitEmpty"`
	// Host Group to run VM in
	// +optional
	HostGroup string `json:"hostGroup,omitEmpty"`
	// Name of the VM
	// +optional
	VMName string `json:"vmName,omitempty"`
	// Pool to get VM name from
	// +optional
	VMNameFromPool *corev1.LocalObjectReference `json:"vmNameFromPool,omitempty"`
	// VM template to use
	// +optional
	VMTemplate string `json:"vmTemplate,omitempty"`
	// Extra disks (after the VHDisk) to connect to the VM
	// +optional
	Disks []VmDisk `json:"disks,omitEmpty"`
	// Virtual Fibrechannel device
	// +optional
	FibreChannel []FibreChannel `json:"fibreChannel,omitEmpty"`
	// Number of CPU's
	CPUCount int `json:"cpuCount"`
	// Allocated memory
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`
	// Dynamic Memory
	// +optional
	DynamicMemory *DynamicMemory `json:"dynamicMemory,omitempty"`
	// Hardware profile
	HardwareProfile string `json:"hardwareProfile"`
	// OperatingSystem
	// +optional
	OperatingSystem string `json:"operatingSystem,omitempty"`
	// Network settings
	// +optional
	Networking *Networking `json:"networking,omitempty"`
	// Active Directory entry
	// +optional
	ActiveDirectory *ActiveDirectory `json:"activeDirectory,omitempty"`
	// AvailabilitySet
	// +optional
	AvailabilitySet string `json:"availabilitySet,omitempty"`
	// Options for New-SCVirtualMachine
	// +optional
	VMOptions *VmOptions `json:"vmOptions,omitempty"`
	// Custom VirtualMachine Properties
	// Named CustomProperty because that's what it's named in SCVMM virtual machines
	// +optional
	CustomProperty map[string]string `json:"customProperty,omitempty"`
	// VirtualMachine tag
	// +optional
	Tag string `json:"tag,omitempty"`
	// ProviderRef points to an ScvmmProvider instance that defines the provider settings for this cluster.
	// Will be copied from scvmmcluster if not using local bootstrap
	// +optional
	ProviderRef *ScvmmProviderReference `json:"providerRef,omitEmpty"`
	// Custom bootstrap secret ref
	// This triggers the controller to create the machine without a (cluster-api) cluster
	// For testing purposes, or just for creating VMs
	// +optional
	Bootstrap *clusterv1.Bootstrap `json:"bootstrap,omitempty"`
}

type VmOptions struct {
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
	// CPULimitForMigration
	// +optional
	CPULimitForMigration *bool `json:"cpuLimitForMigration,omitempty"`
	// CPULimitFunctionality
	// +optional
	CPULimitFunctionality *bool `json:"cpuLimitFunctionality,omitempty"`
	// EnableNestedVirtualization
	// +optional
	EnableNestedVirtualization *bool `json:"enableNestedVirtualization,omitempty"`
	// CheckpointType
	// +kubebuilder:default:=Standard
	CheckpointType *bool `json:"checkpointType,omitempty"`
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

type NetworkDevice struct {
	// Network device name
	// +kubebuilder:default:=eth0
	DeviceName string `json:"deviceName,omitempty"`
	// Virtual Network identifier
	VMNetwork string `json:"vmNetwork"`
	// IP Address
	// +optional
	IPAddresses []string `json:"ipAddresses,omitempty"`
	// Gateway
	// +optional
	Gateway string `json:"gateway,omitempty"`
	// Nameservers
	// +optional
	Nameservers []string `json:"nameservers,omitempty"`
	// List of search domains used when resolving with DNS
	// +optional
	SearchDomains []string `json:"searchDomains,omitempty"`
	// List of IPAddressPools that should be assigned
	// to IPAddressClaims. The machine's cloud-init metadata will be populated
	// with IPAddresses fulfilled by an IPAM provider.
	// +optional
	AddressesFromPools []corev1.TypedLocalObjectReference `json:"addressesFromPools,omitempty"`
}

type Networking struct {
	// Network devices
	// +optional
	// +listType=map
	// +listMapKey=deviceName
	Devices []NetworkDevice `json:"devices,omitempty"`
	// Host domain
	// +optional
	Domain string `json:"domain,omitempty"`
}

type FibreChannel struct {
	// Storage Fabric Classification
	// +optional
	StorageFabricClassification string `json:"storageFabricClassification,omitEmpty"`
	// Virtual SAN
	// +optional
	VirtualSAN string `json:"virtualSAN,omitEmpty"`
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
	// +optional
	MemberOf []string `json:"memberOf,omitempty"`
}

type DynamicMemory struct {
	// Minimum
	Minimum *resource.Quantity `json:"minimum"`
	// Maximum
	Maximum *resource.Quantity `json:"maximum"`
	// BufferPercentage
	// +optional
	BufferPercentage *int `json:"bufferPercentage,omitempty"`
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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
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

//+kubebuilder:object:root=true

// ScvmmMachineList contains a list of ScvmmMachine
type ScvmmMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmMachine `json:"items"`
}

func init() {
	//SchemeBuilder.Register(&ScvmmMachine{}, &ScvmmMachineList{})
	objectTypes = append(objectTypes, &ScvmmMachine{}, &ScvmmMachineList{})
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
	if in.HardwareProfile != "" && in.HardwareProfile != out.HardwareProfile {
		changed = true
		out.HardwareProfile = in.HardwareProfile
	}
	if in.Networking != nil && !reflect.DeepEqual(in.Networking, out.Networking) {
		changed = true
		out.Networking = in.Networking
	}
	if in.AvailabilitySet != "" && in.AvailabilitySet != out.AvailabilitySet {
		changed = true
		out.AvailabilitySet = in.AvailabilitySet
	}
	if in.VMOptions != nil && !reflect.DeepEqual(in.VMOptions, out.VMOptions) {
		changed = true
		out.VMOptions = in.VMOptions
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
