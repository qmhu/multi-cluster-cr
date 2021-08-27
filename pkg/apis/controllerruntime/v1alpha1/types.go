package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSetSpec `json:"spec,omitempty"`
}

type ClusterSetSpec struct {
	ClusterAffinitys []ClusterAffinity `json:"clusterAffinities"`
}

type Provider string

const (
	ClusternetProvider Provider = "clusternet"
)

type ClusterAffinity struct {
	*metav1.LabelSelector
	Provider Provider `json:"provider"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterSetList is a list of ClusterSet resources
type ClusterSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterSet `json:"items"`
}
