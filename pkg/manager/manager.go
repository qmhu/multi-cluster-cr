package manager

import (
	"context"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	clusterManager "qmhu/multi-cluster-cr/pkg/cluster"
	"qmhu/multi-cluster-cr/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sync"

	"k8s.io/client-go/rest"
	"qmhu/multi-cluster-cr/pkg/source"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	managerLog = ctrl.Log.WithName("manager")
)

type Controller struct {
	ctrlcontroller.Controller
	Name string
}

type Cluster struct {
	cluster.Cluster
	Name string
}

type Manager interface {
	ctrl.Manager

	GetCluster() cluster.Cluster

	EventChannel() chan source.ClusterEvent

	GetClusterClient() client.Client
	GetClusterMap() *clusterManager.MultiClusterMap
	GetMultiClusterController() *controller.MultiClusterController

	GetAdminClusterName() string

	GetSingeClusterManager(cluster source.Cluster) (ctrl.Manager, error)
}

func NewManager(config *rest.Config, options ctrl.Options) (Manager, error) {
	manager, err := ctrl.NewManager(config, options)
	if err != nil {
		return nil, err
	}
	m := &multiClusterManager{
		eventChannel: make(chan source.ClusterEvent),
		Manager:      manager,
		clusterMap:   clusterManager.NewMultiClusterMap(),
		managerMap:   make(map[string]ctrl.Manager),
	}
	// create cluster
	adminCluster, err := cluster.New(config, func(clusterOptions *cluster.Options) {
		clusterOptions.Scheme = m.options.Scheme
		clusterOptions.MapperProvider = m.options.MapperProvider
		clusterOptions.Logger = m.options.Logger
		clusterOptions.SyncPeriod = m.options.SyncPeriod
		clusterOptions.Namespace = m.options.Namespace
		clusterOptions.NewCache = m.options.NewCache
		clusterOptions.NewClient = m.options.NewClient
		clusterOptions.ClientDisableCacheFor = m.options.ClientDisableCacheFor
		clusterOptions.DryRunClient = m.options.DryRunClient
		clusterOptions.EventBroadcaster = m.options.EventBroadcaster
	})
	if err != nil {
		return nil, err
	}

	m.clusterMap.AddCluster("admin", adminCluster)

	return m, nil
}

type multiClusterManager struct {
	mu sync.Mutex
	ctrl.Manager
	options                ctrl.Options
	eventChannel           chan source.ClusterEvent
	clusterMap             clusterManager.MultiClusterMap
	multiClusterController controller.MultiClusterController
	managerMap             map[string]ctrl.Manager
	started                bool
	startedLeader          bool

	// leaderElectionRunnables is the set of Controllers that the controllerManager injects deps into and Starts.
	// These Runnables are managed by lead election.
	leaderElectionRunnables []manager.Runnable

	// nonLeaderElectionRunnables is the set of webhook servers that the controllerManager injects deps into and Starts.
	// These Runnables will not be blocked by lead election.
	nonLeaderElectionRunnables []manager.Runnable

	// waitForRunnable is holding the number of runnables currently running so that
	// we can wait for them to exit before quitting the manager
	waitForRunnable sync.WaitGroup

	internalCtx context.Context

	errChan chan error
}

func (m *multiClusterManager) GetCluster() cluster.Cluster {
	return m.clusterMap.GetCluster("admin")
}

func (m *multiClusterManager) SetFields(i interface{}) error {
	if c, isController := i.(Controller); isController {
		mgr, exist := m.managerMap[c.Name]
		if !exist {
			return fmt.Errorf("manager not found")
		}

		return mgr.SetFields(i)
	}

	return nil
}

func (m *multiClusterManager) GetSingeClusterManager(cluster source.Cluster) (ctrl.Manager, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mgr, ok := m.managerMap[cluster.Name]; ok {
		return mgr, nil
	} else {
		// Modify options to create a singe cluster manager with cluster inside
		opts := m.options
		opts.HealthProbeBindAddress = "0"
		opts.MetricsBindAddress = "0"
		opts.LeaderElection = false
		opts.Namespace = cluster.Namespace
		opts.WebhookServer = nil // disable webhook

		config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.KubeConfig))
		if err != nil {
			return nil, err
		}
		newMgr, err := ctrl.NewManager(config, opts)
		if err != nil {
			return nil, err
		}
		go newMgr.Start(m.internalCtx)
		m.managerMap[cluster.Name] = newMgr
		return newMgr, nil
	}
}

func (m *multiClusterManager) GetAdminClusterName() string {
	panic("implement me")
}

func (m *multiClusterManager) GetClusterMap() *clusterManager.MultiClusterMap {
	return &m.clusterMap
}

func (m *multiClusterManager) GetMultiClusterController() *controller.MultiClusterController {
	return &m.multiClusterController
}

func (m *multiClusterManager) EventChannel() chan source.ClusterEvent {
	return m.eventChannel
}

// todo
func (m *multiClusterManager) GetClusterClient() client.Client {
	return nil
}

// todo
func (m *multiClusterManager) Add(r manager.Runnable) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set dependencies on the object
	if err := m.SetFields(r); err != nil {
		return err
	}

	var shouldStart bool

	// Add the runnable to the leader election or the non-leaderelection list
	if leRunnable, ok := r.(manager.LeaderElectionRunnable); ok && !leRunnable.NeedLeaderElection() {
		shouldStart = m.started
		m.nonLeaderElectionRunnables = append(m.nonLeaderElectionRunnables, r)
	} else {
		shouldStart = m.startedLeader
		m.leaderElectionRunnables = append(m.leaderElectionRunnables, r)
	}

	if shouldStart {
		// If already started, start the controller
		m.startRunnable(r)
	}

	return nil
}

func (m *multiClusterManager) startRunnable(r manager.Runnable) {
	m.waitForRunnable.Add(1)
	go func() {
		defer m.waitForRunnable.Done()
		if err := r.Start(m.internalCtx); err != nil {
			m.errChan <- err
		}
	}()
}

func (m *multiClusterManager) Start(ctx context.Context) error {
	m.started = true
	m.startedLeader = true
	m.errChan = make(chan error)
	m.internalCtx, _ = context.WithCancel(ctx)

	go func() {
		for {
			select {
			case e := <-m.eventChannel:
				managerLog.Info(fmt.Sprintf("EventType %s Cluster(%s %s)", e.EventType, e.Cluster.Namespace, e.Cluster.Name))

				config, err := clientcmd.RESTConfigFromKubeConfig([]byte(e.Cluster.KubeConfig))
				if err != nil {
					managerLog.Error(err, fmt.Sprintf("EventType %s Cluster(%s %s)", e.EventType, e.Cluster.Namespace, e.Cluster.Name))
					continue
				}

				// create cluster
				cluster, err := cluster.New(config, func(clusterOptions *cluster.Options) {
					clusterOptions.Scheme = m.options.Scheme
					clusterOptions.MapperProvider = m.options.MapperProvider
					clusterOptions.Logger = m.options.Logger
					clusterOptions.SyncPeriod = m.options.SyncPeriod
					clusterOptions.Namespace = e.Cluster.Namespace // Event Cluster namespace
					clusterOptions.NewCache = m.options.NewCache
					clusterOptions.NewClient = m.options.NewClient
					clusterOptions.ClientDisableCacheFor = m.options.ClientDisableCacheFor
					clusterOptions.DryRunClient = m.options.DryRunClient
					clusterOptions.EventBroadcaster = m.options.EventBroadcaster
				})
				if err != nil {
					managerLog.Error(err, "New cluster failed")
					continue
				}

				// update ClusterManager
				m.clusterMap.AddCluster(e.Cluster.Name, cluster)

				m.multiClusterController.CreateController(e.Cluster)
			}
		}
	}()

	m.Manager.Start(ctx)

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
