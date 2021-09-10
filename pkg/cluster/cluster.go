package cluster

import (
	"qmhu/multi-cluster-cr/pkg/source"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

type MultiClusterMap struct {
	sync.Mutex
	clusterMap map[string]cluster.Cluster
}

func NewMultiClusterMap() MultiClusterMap {
	return MultiClusterMap{
		clusterMap: make(map[string]cluster.Cluster),
	}
}

func (c *MultiClusterMap) GetCluster(name string) cluster.Cluster{
	return c.clusterMap[name]
}

func (c *MultiClusterMap) AddCluster(name string, cluster cluster.Cluster) {
	c.Lock()
	defer c.Unlock()

	c.clusterMap[name] = cluster
}

func (c *MultiClusterMap) DeleteCluster(cluster source.Cluster) error {
	return nil
}
