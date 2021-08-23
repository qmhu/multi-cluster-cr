package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type ClusterSetSpec struct {
	ClusterAffinitys *metav1.LabelSelector `json:"clusterAffinities"`
}

type Provider string

const (
	ClusternetProvider Provider = "clusternet"
)

type ClusterAffinity struct {
	*metav1.LabelSelector
	Provider Provider `json:"provider"`
}
