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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// ScvmmClusterSpec defines the desired state of ScvmmCluster
type ScvmmClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`
}

// ScvmmClusterStatus defines the observed state of ScvmmCluster
type ScvmmClusterStatus struct {
	// Ready denotes that the docker cluster (infrastructure) is ready.
	Ready bool `json:"ready"`

	// Conditions defines current service state of the DockerCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int `json:"port"`
}

// +kubebuilder:resource:path=scvmmclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// ScvmmCluster is the Schema for the scvmmclusters API
type ScvmmCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScvmmClusterSpec   `json:"spec,omitempty"`
	Status ScvmmClusterStatus `json:"status,omitempty"`
}

func (c *ScvmmCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

func (c *ScvmmCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ScvmmClusterList contains a list of ScvmmCluster
type ScvmmClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScvmmCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScvmmCluster{}, &ScvmmClusterList{})
}
