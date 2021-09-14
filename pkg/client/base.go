package client

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MultiClusterClientBase interface {
	getRealClient(ctx context.Context) (client.Client, error)
	Scheme() *runtime.Scheme
	SetScheme(scheme *runtime.Scheme)
	RESTMapper() meta.RESTMapper
	SetRESTMapper(restMapper meta.RESTMapper)
	Add(clusterName string, client client.Client)
	Del(clusterName string)
}

type MultiClusterClientBaseImpl struct {
	mcMap      map[string]client.Client
	scheme     *runtime.Scheme
	restMapper meta.RESTMapper
}

func (base *MultiClusterClientBaseImpl) Scheme() *runtime.Scheme {
	return base.scheme
}

func (base *MultiClusterClientBaseImpl) SetScheme(scheme *runtime.Scheme) {
	base.scheme = scheme
}

func (base *MultiClusterClientBaseImpl) RESTMapper() meta.RESTMapper {
	return base.restMapper
}

func (base *MultiClusterClientBaseImpl) SetRESTMapper(restMapper meta.RESTMapper) {
	base.restMapper = restMapper
}

func (base *MultiClusterClientBaseImpl) getRealClient(ctx context.Context) (client.Client, error) {
	clusterName, err := ExtractClusterFromContext(ctx)
	if err != nil {
		return nil, err
	}

	client := base.mcMap[clusterName]
	if client == nil {
		return nil, fmt.Errorf(ErrClusterNotFoundInClientMap, clusterName)
	}

	return client, nil
}

func (base *MultiClusterClientBaseImpl) Add(clusterName string, client client.Client) {
	base.mcMap[clusterName] = client
}

func (base *MultiClusterClientBaseImpl) Del(clusterName string) {
	delete(base.mcMap, clusterName)
}
