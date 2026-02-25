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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// ScvmmClusterSpec defines the desired state of ScvmmCluster
type ScvmmClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`
	// ProviderRef points to an ScvmmProvider instance that defines the provider settings for this cluster.
	// +optional
	ProviderRef *ScvmmProviderReference `json:"providerRef,omitempty"`
	// FailureDomains is a slice of failure domain objects which will be copied to the status field
	// +optional
	FailureDomains map[string]ScvmmFailureDomainSpec `json:"failureDomains,omitempty"`
}

type ScvmmFailureDomainSpec struct {
	// ControlPlane determines if this failure domain is suitable for use by control plane machines.
	// +optional
	ControlPlane bool `json:"controlPlane,omitempty"`

	// Cloud for this failure domain
	Cloud string `json:"cloud"`

	// Host Group for this failure domain
	HostGroup string `json:"hostGroup"`

	// Networking settings for this failure domain
	// +optional
	Networking *Networking `json:"networking,omitempty"`
}

// ScvmmClusterStatus defines the observed state of ScvmmCluster
type ScvmmClusterStatus struct {
	// Initialization.Provisioned denotes that the scvmm cluster (infrastructure) is ready.
	// +optional
	Initialization struct {
		// +optional
		Provisioned bool `json:"provisioned,omitempty"`
	} `json:"initialization,omitempty,omitzero"`

	// Conditions defines current service state of the ScvmmCluster.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// FailureDomains is a slice of failure domain objects copied from the spec
	// +optional
	FailureDomains []clusterv1.FailureDomain `json:"failureDomains,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ScvmmCluster is the Schema for the scvmmclusters API
type ScvmmCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmClusterSpec   `json:"spec,omitempty"`
	Status ScvmmClusterStatus `json:"status,omitempty"`
}

func (c *ScvmmCluster) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

func (c *ScvmmCluster) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// ScvmmClusterList contains a list of ScvmmCluster
type ScvmmClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmCluster `json:"items"`
}

func init() {
	//SchemeBuilder.Register(&ScvmmCluster{}, &ScvmmClusterList{})
	objectTypes = append(objectTypes, &ScvmmCluster{}, &ScvmmClusterList{})
}
