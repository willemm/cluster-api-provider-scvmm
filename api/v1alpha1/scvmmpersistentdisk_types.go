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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ScvmmPersistentDiskReference struct {
	// Name of ScvmmPersistentDisk
	Name string `json:"name"`

	// Fields grabbed from ScvmmPersistentDisk to pass through
	Path     string `json:"-"`
	Filename string `json:"-"`
	Existing bool   `json:"-"`
}

// ScvmmPersistentDiskSpec defines the desired state of ScvmmPersistentDisk
type ScvmmPersistentDiskSpec struct {
	// Location (share path) where disk is stored
	Path string `json:"path"`
	// Filename of stored disk
	// +optional
	Filename string `json:"filename"`
	// Is the disk already created
	// +optional
	Existing bool `json:"existing"`

	// Size of the virtual disk
	Size resource.Quantity `json:"size"`
	// Specify that the virtual disk can expand dynamically (default: true)
	// +optional
	Dynamic bool `json:"dynamic,omitEmpty"`
}

// ScvmmPersistentDiskStatus defines the observed state of ScvmmPersistentDisk
type ScvmmPersistentDiskStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ScvmmPersistentDisk is the Schema for the scvmmpersistentdisks API
type ScvmmPersistentDisk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmPersistentDiskSpec   `json:"spec,omitempty"`
	Status ScvmmPersistentDiskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScvmmPersistentDiskList contains a list of ScvmmPersistentDisk
type ScvmmPersistentDiskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmPersistentDisk `json:"items"`
}

func init() {
	//SchemeBuilder.Register(&ScvmmPersistentDisk{}, &ScvmmPersistentDiskList{})
	objectTypes = append(objectTypes, &ScvmmPersistentDisk{}, &ScvmmPersistentDiskList{})
}
