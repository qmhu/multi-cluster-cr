package configwatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	genericclient "sigs.k8s.io/kubefed/pkg/client/generic"
	"sigs.k8s.io/kubefed/pkg/controller/util"

	"qmhu/multi-cluster-cr/pkg/config"
)

const KubeFedDefaultNamespace = "kube-federation-system"

// KubeFedWatcher watches KubeFedCluster from a kubernetes apiserver, convert to kubeconfig and delivering events to a channel.
type KubeFedWatcher struct {
	logger     logr.Logger
	namespace  string
	controller cache.Controller
	clusterMap map[string]*fedv1b1.KubeFedCluster
	client     genericclient.Client
	eventChan  chan Event
	errorChan  chan error
	closeChan  chan struct{}
	mu         sync.Mutex
}

// NewKubeFedWatcher construct a KubeFedWatcher with a kubeconfig and default namespace
func NewKubeFedWatcher(config *rest.Config) (*KubeFedWatcher, error) {
	return NewKubeFedWatcherWithNamespace(config, KubeFedDefaultNamespace)
}

// NewKubeFedWatcherWithNamespace construct a KubeFedWatcher with a kubeconfig and the namespace that kubefed controll plane works on
func NewKubeFedWatcherWithNamespace(config *rest.Config, namespace string) (*KubeFedWatcher, error) {
	client, err := genericclient.New(config)
	if err != nil {
		return nil, err
	}

	w := &KubeFedWatcher{
		logger:     log.Log.WithName("kubefed-watcher"),
		namespace:  namespace,
		clusterMap: make(map[string]*fedv1b1.KubeFedCluster),
		client:     client,
		eventChan:  make(chan Event),
		errorChan:  make(chan error),
		closeChan:  make(chan struct{}),
	}

	_, w.controller, err = util.NewGenericInformerWithEventHandler(
		config,
		namespace,
		&fedv1b1.KubeFedCluster{},
		time.Hour*12,
		&cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj interface{}) {
				castObj, ok := obj.(*fedv1b1.KubeFedCluster)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						w.logger.Error(nil, fmt.Sprintf("Couldn't get object from tombstone %#v", obj))
						return
					}
					castObj, ok = tombstone.Obj.(*fedv1b1.KubeFedCluster)
					if !ok {
						w.logger.Error(nil, fmt.Sprintf("Tombstone contained object that is not expected %#v", obj))
						return
					}
				}
				w.onClusterDelete(castObj)
			},
			AddFunc: func(obj interface{}) {
				castObj := obj.(*fedv1b1.KubeFedCluster)
				w.onClusterUpdate(castObj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newCluster := newObj.(*fedv1b1.KubeFedCluster)
				oldCluster := oldObj.(*fedv1b1.KubeFedCluster)

				if newCluster.DeletionTimestamp != nil {
					w.onClusterDelete(newCluster)
				}

				if equality.Semantic.DeepEqual(oldCluster.Spec, newCluster.Spec) {
					return
				}

				w.onClusterUpdate(newCluster)
			},
		},
	)
	if err != nil {
		return nil, err
	}

	go w.watch()

	return w, nil
}

func (w *KubeFedWatcher) Events() <-chan Event {
	return w.eventChan
}

func (w *KubeFedWatcher) Errors() <-chan error {
	return w.errorChan
}

func (w *KubeFedWatcher) Stop() {
	close(w.closeChan)
}

func (w *KubeFedWatcher) watch() {
	defer close(w.eventChan)
	defer close(w.errorChan)
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// start the controller to watch KubeFedClusters
		w.controller.Run(ctx.Done())
	}()

	// wait close signal
	<-w.closeChan

	// stop the controller
	cancel()
}

func (w *KubeFedWatcher) onClusterDelete(c *fedv1b1.KubeFedCluster) {
	if c == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exist := w.clusterMap[c.Name]; exist {
		config := w.buildClusterConfig(c)
		if config != nil {
			w.eventChan <- Event{
				Type:   Deleted,
				Config: config,
			}
		}
	}

	delete(w.clusterMap, c.Name)
}

func (w *KubeFedWatcher) onClusterUpdate(c *fedv1b1.KubeFedCluster) {
	if c == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	config := w.buildClusterConfig(c)
	if config == nil {
		return
	}

	if saveCluster, exist := w.clusterMap[c.Name]; exist {
		if equality.Semantic.DeepEqual(saveCluster.Spec, c.Spec) {
			return
		} else {
			w.eventChan <- Event{
				Type:   Updated,
				Config: config,
			}
		}
	} else {
		w.eventChan <- Event{
			Type:   Added,
			Config: config,
		}
	}

	w.clusterMap[c.Name] = c
}

// buildClusterConfig construct a NamedConfig based on KubeFedCluster
func (w *KubeFedWatcher) buildClusterConfig(c *fedv1b1.KubeFedCluster) *config.NamedConfig {
	fedClusterConfig, err := util.BuildClusterConfig(c, w.client, w.namespace)
	if err != nil {
		w.errorChan <- err
		return nil
	}

	return config.NewNamedConfig(c.Name, fedClusterConfig)
}
