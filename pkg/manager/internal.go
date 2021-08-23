package manager

import (
	"qmhu/multi-cluster-cr/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ manager.Runnable = &multiClusterManager{}

type multiClusterManager struct {
	// cluster holds a variety of methods to interact with the admin cluster
	adminCluster cluster.Cluster
}

func (m *multiClusterManager) GetCluster(clusterName string) cluster.Cluster {
	return nil
}

func (m *multiClusterManager) Recv(e <-chan source.ClusterEvent) {

}

func (m *multiClusterManager) Add(r manager.Runnable) error {

}
