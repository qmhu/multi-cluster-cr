package cluster

import (
	"qmhu/multi-cluster-cr/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

type Manager struct {
	clusterMap map[string]cluster.Cluster
}

func (*Manager) GetCluster(name string) cluster.Cluster {
	return nil
}

func (*Manager) AddCluster(cluster source.Cluster) error {
	return nil
}

func (*Manager) DeleteCluster(cluster source.Cluster) error {
	return nil
}
