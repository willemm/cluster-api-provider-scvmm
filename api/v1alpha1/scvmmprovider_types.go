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
)

type ScvmmProviderReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ScvmmProviderSpec defines the desired state of ScvmmProvider
type ScvmmProviderSpec struct {
	// Hostname of scvmm server
	ScvmmHost string `json:"scvmmHost"`
	// Reference to secret containing user and password for scvmm
	ScvmmSecret *corev1.SecretReference `json:"scvmmSecret,omitempty"`
	// Jumphost to run scvmm scripts on, instead of directly on the scvmm server
	// +optional
	ExecHost string `json:"execHost,omitempty"`
	// How long to keep winrm connections to scvmm alive
	// Default 20 seconds
	KeepAliveSeconds int `json:"keepAliveSeconds,omitempty"`
	// Settings that define how to pass cloud-init data
	// +optional
	CloudInit ScvmmCloudInitSpec `json:"cloudInit,omitempty"`
	// Active Directory server
	// +optional
	ADServer string `json:"adServer,omitempty"`
	// Reference to secret containing user and password for activediractory
	// +optional
	ADSecret *corev1.SecretReference `json:"adSecret,omitempty"`
	// Extra functions to run when provisioning machines
	// +optional
	ExtraFunctions map[string]string `json:"extraFunctions,omitempty"`

	// Scvmm/AD username and Password (not serialized)
	ScvmmUsername string `json:"-"`
	ScvmmPassword string `json:"-"`
	ADUsername    string `json:"-"`
	ADPassword    string `json:"-"`

	// Environment variables to set for scripts
	// Will be supplemented with scvmm and ad credentials
	// +optional
	Env map[string]string `json:"env,omitempty"`

	// Sensitive env variables
	SensitiveEnv map[string]string `json:"-"`
}

type ScvmmCloudInitSpec struct {
	// Library share where ISOs can be placed for cloud-init
	// Defaults to \\scvmmhost\MSSCVMMLibrary\ISOs\cloud-init
	// +optional
	LibraryShare string `json:"libraryShare,omitempty"`
	// Filesystem to use for cloud-init
	// vfat or iso9660
	// Defaults to vfat
	// +optional
	// +kubebuilder:validation:Enum=vfat;iso9660
	FileSystem string `json:"fileSystem,omitempty"`
	// Device type to use for cloud-init
	// dvd floppy scsi ide
	// Defaults to dvd
	// +optional
	// +kubebuilder:validation:Enum=dvd;floppy;scsi;ide
	DeviceType string `json:"deviceType,omitempty"`
}

// ScvmmProviderStatus defines the observed state of ScvmmProvider
type ScvmmProviderStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ScvmmProvider is the Schema for the scvmmproviders API
type ScvmmProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmProviderSpec   `json:"spec,omitempty"`
	Status ScvmmProviderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScvmmProviderList contains a list of ScvmmProvider
type ScvmmProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmProvider `json:"items"`
}

func init() {
	//SchemeBuilder.Register(&ScvmmProvider{}, &ScvmmProviderList{})
	objectTypes = append(objectTypes, &ScvmmProvider{}, &ScvmmProviderList{})
}
