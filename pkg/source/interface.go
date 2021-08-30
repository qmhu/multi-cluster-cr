package source

import (
	controllerruntimeapi "qmhu/multi-cluster-cr/pkg/apis/controllerruntime/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type ClusterEventType string

const (
	ClusterEventAdd    ClusterEventType = "Add"
	ClusterEventRemove ClusterEventType = "Remove"
)

type Cluster struct {
	Name       string
	Namespace  string
	KubeConfig string
}

type ClusterEvent struct {
	EventType ClusterEventType
	Cluster   Cluster
}

type ClusterSource interface {
	Register(eventChan chan ClusterEvent)
	manager.Runnable
}

type ClusterProvider interface {
	ListClusters(clusterAffinity controllerruntimeapi.ClusterAffinity) ([]Cluster, error)

	ProviderType() controllerruntimeapi.Provider
}
