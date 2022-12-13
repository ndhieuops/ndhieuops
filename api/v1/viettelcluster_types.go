/*
Copyright 2022.

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

package v1

import (
	cloudapi "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ViettelClusterSpec defines the desired state of ViettelCluster
type ViettelClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	VpcID string `json:"vpc"`

	ProjectID string `json:"projectId"`

	OwnerID string `json:"owner,omitempty"`

	RegionID string `json:"region"`

	SubnetID string `json:"subnet"`
}

// ViettelClusterStatus defines the observed state of ViettelCluster
type ViettelClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Ready bool `json:"ready"`

	Vpc *cloudapi.VPC `json:"vpc,omitempty"`

	Subnet *cloudapi.Subnet `json:"subnet,omitempty"`

	LoadBalancer *cloudapi.LoadBalancer `json:"loadbalancer,omitempty"`
	// +optional
	FailureReason *capierrors.ClusterStatusError `json:"failureReason,omitempty"`

	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ViettelCluster is the Schema for the viettelclusters API
type ViettelCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ViettelClusterSpec   `json:"spec,omitempty"`
	Status ViettelClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ViettelClusterList contains a list of ViettelCluster
type ViettelClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ViettelCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ViettelCluster{}, &ViettelClusterList{})
}
