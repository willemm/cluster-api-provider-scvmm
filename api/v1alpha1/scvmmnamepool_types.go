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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ScvmmNamePoolSpec defines the desired state of ScvmmNamePool
type ScvmmNamePoolSpec struct {
	// VMNames is the list of VM name ranges in this pool
	VMNameRanges []VmNameRange `json:"vmNameRanges"`
}

type VmNameRange struct {
	// Start of name range
	Start string `json:"start"`
	// End of name range
	// +optional
	End string `json:"end,omitempty"`
}

// ScvmmNamePoolStatus defines the observed state of ScvmmNamePool
type ScvmmNamePoolStatus struct {
	// Conditions defines current service state of the ScvmmNamePool.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
	// List of vmnames in use by ScvmmMachines
	// +optional
	VMNames []VmPoolName `json:"vmNames,omitempty"`
}

type VmPoolName struct {
	// Name given to Scvmmmachine
	VMName string `json:"vmName"`
	// Scvmmmachine referencing the name
	Owner *corev1.TypedLocalObjectReference `json:"owner"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ScvmmNamePool is the Schema for the scvmmnamepools API
type ScvmmNamePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmNamePoolSpec   `json:"spec,omitempty"`
	Status ScvmmNamePoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScvmmNamePoolList contains a list of ScvmmNamePool
type ScvmmNamePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmNamePool `json:"items"`
}

func init() {
	//SchemeBuilder.Register(&ScvmmNamePool{}, &ScvmmNamePoolList{})
	objectTypes = append(objectTypes, &ScvmmNamePool{}, &ScvmmNamePoolList{})
}
