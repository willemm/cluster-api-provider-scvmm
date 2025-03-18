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
	// +optional
	Path string `json:"path"`
	// Filename of stored disk
	// +optional
	Filename string `json:"filename"`
	// Is the disk already created
	// +optional
	Existing bool `json:"existing"`

	// Size of the virtual disk
	Size resource.Quantity `json:"size"`
	// Specify that the virtual disk can expand dynamically
	// +kubebuilder:default=true
	// +optional
	Dynamic bool `json:"dynamic,omitEmpty"`
}

// ScvmmPersistentDiskStatus defines the observed state of ScvmmPersistentDisk
type ScvmmPersistentDiskStatus struct {
	// Current usage (size) of virtual disk
	// +optional
	Size *resource.Quantity `json:"size,omitEmpty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".metadata.ownerReferences[?(@.kind=='ScvmmMachine')].name",type="string",name="OWNER",description="ScvmmMachine that owns disk"
// +kubebuilder:printcolumn:JSONPath=".spec.path",type="string",name="PATH",description="Storage Path",priority=1
// +kubebuilder:printcolumn:JSONPath=".spec.filename",type="string",name="FILENAME",description="vhdx Filename",priority=1
// +kubebuilder:printcolumn:JSONPath=".spec.size",type="string",name="SIZE",description="Maximum Disk Size"
// +kubebuilder:printcolumn:JSONPath=".status.size",type="string",name="USED",description="Current Disk Size"
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",type="date",name="AGE",description="Creation Timestamp"

// ScvmmPersistentDisk is the Schema for the scvmmpersistentdisks API
type ScvmmPersistentDisk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmPersistentDiskSpec   `json:"spec,omitempty"`
	Status ScvmmPersistentDiskStatus `json:"status,omitempty"`
	Index  int64                     `json:"-"`
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
