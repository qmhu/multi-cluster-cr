package cluster

import (
	"qmhu/multi-cluster-cr/pkg/source"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

type MultiClusterMap struct {
	clusterMap map[string]cluster.Cluster
}

func (*MultiClusterMap) GetCluster(name string) (cluster.Cluster, error) {
	return nil, nil
}

func (*MultiClusterMap) AddCluster(cluster source.Cluster) error {
	return nil
}

func (*MultiClusterMap) DeleteCluster(cluster source.Cluster) error {
	return nil
}
