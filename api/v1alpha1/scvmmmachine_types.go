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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

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
	Ready bool `json:"ready"`
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
// +kubebuilder:printcolumn:JSONPath=".status.biosGuid",type="string",name="GUID",description="Virtual Machine BIS GUID"

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
