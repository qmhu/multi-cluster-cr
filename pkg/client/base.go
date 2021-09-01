package client

import (
	"context"
	"fmt"
	multicluster "qmhu/multi-cluster-cr/pkg/cluster"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MultiClusterClientBase interface {
	getRealClient(ctx context.Context) (client.Client, error)
	Scheme() *runtime.Scheme
	RESTMapper() meta.RESTMapper
}

type MultiClusterClientBaseImpl struct {
	mcMap              *multicluster.MultiClusterMap
	defaultClusterName string
}

func (base *MultiClusterClientBaseImpl) Scheme() *runtime.Scheme {
	cluster, err := base.mcMap.GetCluster(base.defaultClusterName)
	if err != nil {
		return nil
	}
	return cluster.GetScheme()
}

func (base *MultiClusterClientBaseImpl) RESTMapper() meta.RESTMapper {
	cluster, err := base.mcMap.GetCluster(base.defaultClusterName)
	if err != nil {
		return nil
	}
	return cluster.GetRESTMapper()
}

func (base *MultiClusterClientBaseImpl) getRealClient(ctx context.Context) (client.Client, error) {
	clusterName, err := ExtractClusterFromContext(ctx)
	if err != nil {
		return nil, err
	}

	cluster, err := base.mcMap.GetCluster(clusterName)
	if err != nil {
		return nil, fmt.Errorf(ErrClusterNotFoundInClientMap, cluster)
	}

	return cluster.GetClient(), nil
}
