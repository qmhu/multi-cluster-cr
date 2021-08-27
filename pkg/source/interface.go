package source

import controllerruntimeapi "qmhu/multi-cluster-cr/pkg/apis/controllerruntime/v1alpha1"

type ClusterEventType string

const (
	ClusterEventAdd    ClusterEventType = "Add"
	ClusterEventRemove ClusterEventType = "Remove"
)

type ClusterEvent struct {
	eventType ClusterEventType
	cluster   Cluster
}

type Cluster struct {
	Name       string
	Namespace  string
	KubeConfig string
}

type ClusterSource interface {
	Register(eventChan chan ClusterEvent)
}

type ClusterProvider interface {
	ListClusters(clusterAffinity controllerruntimeapi.ClusterAffinity) ([]Cluster, error)

	ProviderType() controllerruntimeapi.Provider
}
