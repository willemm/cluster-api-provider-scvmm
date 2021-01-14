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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ScvmmMachineSpec defines the desired state of ScvmmMachine

type ScvmmMachineSpec struct {
	// ProviderID is scvmm plus bios-guid
	ProviderID string `json:"providerID,omitempty"`
	// VMM cloud to run VM on
	Cloud string `json:"cloud"`
	// Name of the VM
	VMName string `json:"vmName"`
	// Disk size in gigabytes
	DiskSize resource.Quantity `json:"diskSize"`
	// Number of CPU's
	CPUCount int `json:"cpuCount"`
	// Memory in Megabytes
	Memory resource.Quantity `json:"memory"`
	// Virtual Network identifier
	VMNetwork string `json:"vmNetwork"`
}

// ScvmmMachineStatus defines the observed state of ScvmmMachine
type ScvmmMachineStatus struct {
	// Mandatory field, is machine ready
	Ready bool `json:"ready,omitempty"`
	// Status string as given by SCVMM
	VMStatus string `json:"vmStatus,omitempty"`
	// BiosGuid as reposted by SVCMM
	BiosGuid string `json:"biosGuid,omitempty"`
	// Reason for failures
	FailureReason string `json:"failureReason,omitempty"`
	// Description of failure
	FailureMessage string `json:"failureMessage,omitempty"`
	// Creation as given by SCVMM
	CreationTime metav1.Time `json:"creationTime,omitempty"`
	// Modification as given by SCVMM
	ModifiedTime metav1.Time `json:"modifiedTime,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:JSONPath=".status.vmStatus",type="string",name="STATUS",description="Virtual Machine Status"
// +kubebuilder:printcolumn:JSONPath=".status.biosGuid",type="string",name="GUID",description="Virtual Machine BIS GUID"

// ScvmmMachine is the Schema for the scvmmmachines API
type ScvmmMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmMachineSpec   `json:"spec,omitempty"`
	Status ScvmmMachineStatus `json:"status,omitempty"`
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
