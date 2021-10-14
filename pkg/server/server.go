package server

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mtclient "qmhu/multi-cluster-cr/pkg/client"
	"qmhu/multi-cluster-cr/pkg/config"
)

var (
	ClusterNotExistErr = fmt.Errorf("cannot delete non existent cluster")
)

// Server management controllers cross multiple clusters.
type Server interface {
	// AddReconcilerSetup allows you to add reconciler setup functions
	AddReconcilerSetup(fn func(mgr ctrl.Manager) error)

	// Add allows you to register a cluster for server,
	// server create a controller-runtime manager based on kubeconfig in NamedConfig,
	// save the manager, setup with reconciler and run it when server start up.
	Add(add *config.NamedConfig) error

	// Update allows you to update a cluster config for server,
	// it will execute Delete first to shut down controllers for the cluster
	// then execute Add to start controllers
	Update(update *config.NamedConfig) error

	// Delete allows you to unregister a cluster for server,
	// it will stop and delete controller-runtime manager which matches name,
	// return error if the cluster does not registered in server.
	Delete(del string) error

	// Start starts all registered controller-runtime managers and blocks until the context is cancelled.
	// Returns an error if there is an error starting any manager.
	Start(ctx context.Context) error

	// GetClient returns a multi-cluster controller-runtime client.
	// The client use context to decide which cluster to interactive with,
	// You can use the client in your controllers as shown below.
	//
	// func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//	var pod v1.Pod
	//	r.Client.Get(ctx, req.NamespacedName, &pod)
	//	return ctrl.Result{}, nil
	//}
	GetClient() client.Client

	// GetScheme returns the first registered cluster's Scheme.
	// Scheme should be symmetric for clusters when use multi-cluster cr.
	GetScheme() *runtime.Scheme

	// GetLogger returns this server's logger.
	GetLogger() logr.Logger
}

type clusterServer struct {
	// setupFns is the set of reconciler setup functions that server execution with manager and starts.
	setupFns []func(mgr ctrl.Manager) error

	// internalManager is the internal controller-runtime manager for basic functions, like leader election.
	internalManager ctrl.Manager

	// managerRunners is the map of managerRunner that hold all managed controller-runtime managers.
	managerRunners map[string]*managerRunner

	mu sync.Mutex

	// client is a client that can work for multi-cluster requests, use context to decide which cluster to interactive with
	client *mtclient.MultiClusterClient

	// options saves user's options and used for creating a new controller-runtime manager.
	options ctrl.Options

	// leaderElectionConfig is the kubeconfig to do leaderElection
	leaderElectionConfig *rest.Config

	// scheme is the first registered cluster's Scheme.
	scheme *runtime.Scheme

	runnableManagers sync.WaitGroup

	errChan chan error
	isStart bool

	logger logr.Logger

	internalCtx    context.Context
	internalCancel context.CancelFunc
}

func NewServer(config *rest.Config, options ctrl.Options) (Server, error) {
	internalManager, err := ctrl.NewManager(config, options)
	if err != nil {
		return nil, err
	}

	logger := log.Log.WithName("multi-cluster-server")
	if options.Logger != nil {
		logger = options.Logger
	}

	server := &clusterServer{
		setupFns:        make([]func(mgr ctrl.Manager) error, 0),
		managerRunners:  make(map[string]*managerRunner),
		client:          mtclient.NewMultiClusterClient(),
		options:         options,
		logger:          logger,
		internalManager: internalManager,
	}
	return server, nil
}

func (s *clusterServer) AddReconcilerSetup(fn func(mgr ctrl.Manager) error) {
	if fn != nil {
		s.setupFns = append(s.setupFns, fn)
	}
}

func (s *clusterServer) Add(add *config.NamedConfig) error {
	if len(strings.TrimSpace(add.Name)) == 0 || add.Config == nil {
		return fmt.Errorf("name or config is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if runner, exist := s.managerRunners[add.Name]; exist {
		if !reflect.DeepEqual(runner.config, add) {
			return fmt.Errorf("cannot added a cluster with same name and different kubeconfig")
		} else {
			// if config already exist and equal, just return
			return nil
		}
	}

	// add config to create managers
	err := s.addConfig(add)
	if err != nil {
		return err
	}

	runner := s.managerRunners[add.Name]

	// if server is started, start manager now
	if s.isStart {
		err := s.setupManager(runner.manager)
		if err != nil {
			// remove it if failed to setup
			delete(s.managerRunners, add.Name)
			return err
		}

		s.runManager(runner)
	}

	return nil
}

func (s *clusterServer) Update(update *config.NamedConfig) error {
	err := s.Delete(update.Name)
	if err != nil && err != ClusterNotExistErr {
		return fmt.Errorf("update cluster failed: %v ", err)
	}

	err = s.Add(update)
	if err != nil {
		return fmt.Errorf("update cluster failed: %v ", err)
	}

	return nil
}

func (s *clusterServer) Delete(del string) error {
	if len(strings.TrimSpace(del)) == 0 {
		return fmt.Errorf("cannot delete cluster with empty name")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if runner, exist := s.managerRunners[del]; exist {
		if s.isStart {
			// stop manager and wait manager graceful shutdown
			s.logger.Info("Stopping manager", "cluster", runner.config.Name)
			runner.Stop()
		}

		// delete cluster client for multi-cluster client
		s.client.Del(del)
		delete(s.managerRunners, del)
		return nil
	}

	return ClusterNotExistErr
}

func (s *clusterServer) Start(ctx context.Context) error {
	defer func() {
		// cancel all manager if error occurs
		s.internalCancel()
		// wait all managers to stop and complete
		s.runnableManagers.Wait()
	}()

	s.mu.Lock()
	if s.isStart {
		return fmt.Errorf("server already started, please don't start it twice! ")
	}

	s.internalCtx, s.internalCancel = context.WithCancel(ctx)
	s.errChan = make(chan error)
	s.mu.Unlock()

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		// run internalManager here
		s.runnableManagers.Add(1)
		go func() {
			defer s.runnableManagers.Done()

			if err := s.internalManager.Start(s.internalCtx); err != nil {
				s.errChan <- err
			}
		}()

		// wait internalManager to finish leader election
		select {
		case <-s.internalManager.Elected():
			break
		case <-ctx.Done():
			return
		}

		// setup and start
		for _, runner := range s.managerRunners {
			runner := runner
			err := s.setupManager(runner.manager)
			if err != nil {
				s.errChan <- err
				return
			}

			s.runManager(runner)
		}

		s.isStart = true
	}()

	select {
	case <-ctx.Done():
		// We are done
		return nil
	case err := <-s.errChan:
		// Error starting or running a manager
		return err
	}
}

func (s *clusterServer) GetClient() client.Client {
	return s.client
}

func (s *clusterServer) GetScheme() *runtime.Scheme {
	return s.scheme
}

func (s *clusterServer) GetLogger() logr.Logger {
	return s.logger
}

func (s *clusterServer) addConfig(config *config.NamedConfig) error {
	// create controller-runtime manager
	mgr, err := ctrl.NewManager(config.Config, s.getManagerOptions())
	if err != nil {
		s.logger.Error(err, "unable to create manager")
		return err
	}

	managerRunner := &managerRunner{
		manager: mgr,
		config:  config,
	}
	s.managerRunners[config.Name] = managerRunner

	// add cluster client for multi-cluster client
	s.client.Add(config.Name, mgr.GetClient())

	// Use first manager's scheme and restMapper to set manager and client.
	// scheme and restMapper should be symmetric for clusters.
	if s.scheme == nil {
		s.scheme = mgr.GetScheme()
		s.client.SetScheme(mgr.GetScheme())
		s.client.SetRESTMapper(mgr.GetRESTMapper())
	}

	return nil
}

func (s *clusterServer) runManager(runner *managerRunner) {
	s.logger.Info("Starting manager", "cluster", runner.config.Name)
	s.runnableManagers.Add(1)

	go func() {
		defer s.runnableManagers.Done()
		if err := runner.Start(s.internalCtx); err != nil {
			s.errChan <- err
		}
	}()
}

func (s *clusterServer) setupManager(mgr ctrl.Manager) error {
	for _, fn := range s.setupFns {
		err := fn(mgr)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *clusterServer) getManagerOptions() ctrl.Options {
	options := s.options
	options.MetricsBindAddress = "0"
	options.HealthProbeBindAddress = "0"
	options.LeaderElection = false
	options.WebhookServer = nil
	return options
}
