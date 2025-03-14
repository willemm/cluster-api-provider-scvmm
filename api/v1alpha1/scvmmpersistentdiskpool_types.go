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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ScvmmPersistentDiskPoolReference struct {
	// Name of ScvmmPersistentDiskPool
	Name string `json:"name"`
}

// ScvmmPersistentDiskPoolSpec defines the desired state of ScvmmPersistentDiskPool
type ScvmmPersistentDiskPoolSpec struct {
	// Location (directory) where disks are stored
	Directory string `json:"directory"`
	// Maximum number of disk instances in pool (optional)
	// +optional
	MaxDisks int64 `json:"maxDisks,omitempty"`

	// Label selector for disks. Existing ScvmmPersistentDisks that are
	// selected by this will be the ones that are part of this pool.
	// Must match labels in template
	Selector metav1.LabelSelector `json:"selector"`

	// Template for persistent disk
	Template ScvmmPersistentDisk `json:"template"`
}

// ScvmmPersistentDiskPoolStatus defines the observed state of ScvmmPersistentDiskPool
type ScvmmPersistentDiskPoolStatus struct {
	// Current number of instances
	// +optional
	Instances int64 `json:"instances,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ScvmmPersistentDiskPool is the Schema for the scvmmpersistentdiskpools API
type ScvmmPersistentDiskPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmPersistentDiskPoolSpec   `json:"spec,omitempty"`
	Status ScvmmPersistentDiskPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScvmmPersistentDiskPoolList contains a list of ScvmmPersistentDiskPool
type ScvmmPersistentDiskPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmPersistentDiskPool `json:"items"`
}

func init() {
	//SchemeBuilder.Register(&ScvmmPersistentDiskPool{}, &ScvmmPersistentDiskPoolList{})
	objectTypes = append(objectTypes, &ScvmmPersistentDiskPool{}, &ScvmmPersistentDiskPoolList{})
}
