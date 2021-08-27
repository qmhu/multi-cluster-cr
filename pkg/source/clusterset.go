package source

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"math/rand"
	"reflect"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	controllerruntimeapi "qmhu/multi-cluster-cr/pkg/apis/controllerruntime/v1alpha1"
	controllerruntimeClientset "qmhu/multi-cluster-cr/pkg/generated/clientset/versioned"
	clusterprovider "qmhu/multi-cluster-cr/pkg/source/provider"
)

var (
	watchTimeout = 5 * time.Minute
)

func NewClusterSetClusterSource(config *rest.Config, namespace string, name string) (ClusterSource, error) {
	controllerruntimeClient, err := controllerruntimeClientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	clusterSet, err := controllerruntimeClient.ControllerruntimeV1alpha1().ClusterSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error when get ClusterSet %s/%s : %v", namespace, name, err)
	}

	return &ClusterSetClusterSource{
		logger:                  log.Log.WithName("ClusterSetClusterSource"),
		config:                  config,
		clusterSet:              clusterSet,
		controllerruntimeClient: controllerruntimeClient,
		channels:                make([]chan ClusterEvent, 1),
	}, nil
}

type ClusterSetClusterSource struct {
	logger                  logr.Logger
	config                  *rest.Config
	clusterSet              *controllerruntimeapi.ClusterSet
	controllerruntimeClient *controllerruntimeClientset.Clientset
	channels                []chan ClusterEvent
	channelLock             sync.Mutex
	providers               []ClusterProvider
	providerLock            sync.Mutex
	clustersMap             map[Cluster]interface{}
	clusterLock             sync.Mutex
}

func (s *ClusterSetClusterSource) Start(ctx context.Context) error {
	go func() {
		for {
			var resourceVersion string
			timeoutSeconds := int64(watchTimeout.Seconds() * (rand.Float64() + 1.0))
			options := metav1.ListOptions{
				ResourceVersion: resourceVersion,
				TimeoutSeconds:  &timeoutSeconds,
				FieldSelector:   fields.OneTermEqualSelector("metadata.name", s.clusterSet.Name).String(),
			}

			watcher, err := s.controllerruntimeClient.ControllerruntimeV1alpha1().ClusterSets(s.clusterSet.Namespace).Watch(context.TODO(), options)
			if err != nil {
				s.logger.Error(err, fmt.Sprintf("failed to watch ClusterSet %s", klog.KObj(s.clusterSet)))
				continue
			}

		loop:
			for {
				select {
				case event, ok := <-watcher.ResultChan():
					if !ok {
						watcher.Stop()
						break loop
					}
					if event.Type == watch.Error {
						watcher.Stop()
						break loop
					}
					clusterset, ok := event.Object.(*controllerruntimeapi.ClusterSet)
					if err != nil {
						s.logger.Error(err, fmt.Sprintf("convert to ClusterSet failed"))
						continue
					}
					newResourceVersion := clusterset.ResourceVersion
					switch event.Type {
					case watch.Added:
						s.onChange(clusterset)
					case watch.Modified:
						s.onChange(clusterset)
					case watch.Deleted:
						s.logger.V(2).Info("Recv a delete event")
					default:
						s.logger.V(2).Info(fmt.Sprintf("unable to understand watch event %#v", event))
					}
					resourceVersion = newResourceVersion
				}
			}

		}
	}()

	select {
	case <-ctx.Done():
		// exit when recv done
		return nil
	}
}

func (s *ClusterSetClusterSource) Register(eventChan chan ClusterEvent) {
	s.channelLock.Lock()
	s.channels = append(s.channels, eventChan)
	s.channelLock.Unlock()
}

func (s *ClusterSetClusterSource) onChange(cs *controllerruntimeapi.ClusterSet) {
	if reflect.DeepEqual(s.clusterSet, cs) {
		return
	}

	var allClusters []Cluster
	for _, clusterAffinity := range s.clusterSet.Spec.ClusterAffinitys {

		s.providerLock.Lock()
		provider := s.getClusterProvider(clusterAffinity.Provider)
		if provider == nil {
			// lazy initialization
			provider, err := clusterprovider.NewClusternetProvider(s.config)
			if err != nil {
				s.logger.Error(err, fmt.Sprintf("parse object meta failed"))
				continue
			}
			s.providers = append(s.providers, provider)
		}
		s.providerLock.Unlock()

		// list clusters from provider
		clusters, err := provider.ListClusters(clusterAffinity)
		if err != nil {
			s.logger.Error(err, fmt.Sprintf("%s list cluster failed", provider.ProviderType()))
			return
		}

		allClusters = append(allClusters, clusters...)
	}

	var toDeleteClusterMap map[Cluster]interface{}
	var newClusterMap map[Cluster]interface{}

	for cluster, _ := range s.clustersMap {
		toDeleteClusterMap[cluster] = nil
	}

	for _, cluster := range allClusters {
		if _, exist := s.clustersMap[cluster]; !exist {
			s.Notify(ClusterEvent{
				eventType: ClusterEventAdd,
				cluster:   cluster,
			})
		}

		newClusterMap[cluster] = nil
		delete(toDeleteClusterMap, cluster)
	}

	for toDelete, _ := range toDeleteClusterMap {
		s.Notify(ClusterEvent{
			eventType: ClusterEventRemove,
			cluster:   toDelete,
		})
	}

	s.clusterLock.Lock()
	s.clusterSet = cs
	s.clustersMap = newClusterMap
	s.clusterLock.Unlock()
}

func (s *ClusterSetClusterSource) Notify(event ClusterEvent) {
	for _, channel := range s.channels {
		channel <- event
	}
}

func (s *ClusterSetClusterSource) getClusterProvider(provider controllerruntimeapi.Provider) ClusterProvider {
	for _, clusterProvider := range s.providers {
		if clusterProvider.ProviderType() == provider {
			return clusterProvider
		}
	}

	return nil
}
