package manager

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"net/http"
	"qmhu/multi-cluster-cr/pkg/source"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	managerLog = ctrl.Log.WithName("manager")
)

type ControllerConfig struct {
	OnCreation func(mgr ctrl.Manager) interface{}
	OnSetup    func(controller interface{}, mgr ctrl.Manager) error
}

type Manager interface {
	ctrl.Manager

	EventChannel() chan source.ClusterEvent

	GetClusterClient() client.Client
}

func NewManager(config *rest.Config, options ctrl.Options) (Manager, error) {
	return &multiClusterManager{
		eventChannel: make(chan source.ClusterEvent),
	}, nil
}

type multiClusterManager struct {
	eventChannel chan source.ClusterEvent
}

func (m *multiClusterManager) EventChannel() chan source.ClusterEvent {
	return m.eventChannel
}

// todo
func (m *multiClusterManager) GetClusterClient() client.Client {
	return nil
}

// todo
func (m *multiClusterManager) Add(manager.Runnable) error {
	return nil
}

// todo
func (m *multiClusterManager) Elected() <-chan struct{} {
	return nil
}

// todo
func (m *multiClusterManager) AddMetricsExtraHandler(path string, handler http.Handler) error {
	return nil
}

// todo
func (m *multiClusterManager) AddHealthzCheck(name string, check healthz.Checker) error {
	return nil
}

// todo
func (m *multiClusterManager) AddReadyzCheck(name string, check healthz.Checker) error {
	return nil
}

// todo
func (m *multiClusterManager) Start(ctx context.Context) error {
	go func() {
		for {
			select {
			case e := <-m.eventChannel:
				managerLog.Info(fmt.Sprintf("EventType %s Cluster(%s %s)", e.EventType, e.Cluster.Namespace, e.Cluster.Name))
			}
		}
	}()

	select {
	case <-ctx.Done():
		// exit when recv done
		return nil
	}
}

// todo
func (m *multiClusterManager) GetWebhookServer() *webhook.Server {
	return nil
}

// todo
func (m *multiClusterManager) GetLogger() logr.Logger {
	return nil
}

// todo
func (m *multiClusterManager) GetControllerOptions() v1alpha1.ControllerConfigurationSpec {
	return v1alpha1.ControllerConfigurationSpec{}
}

// todo
func (m *multiClusterManager) SetFields(interface{}) error {
	return nil
}

// todo
func (m *multiClusterManager) GetConfig() *rest.Config {
	return nil
}

// todo
func (m *multiClusterManager) GetScheme() *runtime.Scheme {
	return nil
}

// todo
func (m *multiClusterManager) GetClient() client.Client {
	return nil
}

// todo
func (m *multiClusterManager) GetFieldIndexer() client.FieldIndexer {
	return nil
}

// todo
func (m *multiClusterManager) GetCache() cache.Cache {
	return nil
}

// todo
func (m *multiClusterManager) GetEventRecorderFor(name string) record.EventRecorder {
	return nil
}

// todo
func (m *multiClusterManager) GetRESTMapper() meta.RESTMapper {
	return nil
}

// todo
func (m *multiClusterManager) GetAPIReader() client.Reader {
	return nil
}
