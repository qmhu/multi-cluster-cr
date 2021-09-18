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

// Server management controllers cross multiple clusters.
type Server interface {

	// AddReconcilerSetup allows you to add reconciler setup functions
	AddReconcilerSetup(fn func(mgr ctrl.Manager) error)

	// Add allows you to register a cluster for server,
	// server create a controller-runtime manager based on kubeconfig in NamedConfig,
	// save the manager,setup with reconciler and run it when server start up.
	Add(add *config.NamedConfig) error

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

	// GetSchema returns the first registered cluster's Schema.
	// Scheme should be symmetric for clusters when use multi-cluster cr.
	GetSchema() *runtime.Scheme

	// GetLogger returns this server's logger.
	GetLogger() logr.Logger
}

type controllerServer struct {
	// setupFns is the set of reconciler setup functions that server execution with manager and starts.
	setupFns []func(mgr ctrl.Manager) error

	// managerRunners is the map of managerRunner that hold all managed controller-runtime managers.
	managerRunners map[string]*managerRunner

	mu sync.Mutex

	// client is a client that can work for multi-cluster requests, use context to decide which cluster to interactive with
	client *mtclient.MultiClusterClient

	// options saves user's options and used for creating a new controller-runtime manager.
	options ctrl.Options

	// leaderElectionConfig is the kubeconfig to do leaderElection
	leaderElectionConfig *rest.Config

	// scheme is the first registered cluster's Schema.
	scheme *runtime.Scheme

	runnableManagers sync.WaitGroup

	errChan chan error
	isStart bool

	logger logr.Logger

	internalCtx context.Context
}

func NewServer(config *rest.Config, options ctrl.Options) (Server, error) {
	logger := log.Log.WithName("multi-cluster-server")
	if options.Logger != nil {
		logger = options.Logger
	}
	client := mtclient.NewMultiClusterClient()
	server := &controllerServer{
		setupFns:             make([]func(mgr ctrl.Manager) error, 0),
		managerRunners:       make(map[string]*managerRunner),
		client:               client,
		leaderElectionConfig: config,
		options:              options,
		logger:               logger,
	}
	return server, nil
}

func (s *controllerServer) AddReconcilerSetup(fn func(mgr ctrl.Manager) error) {
	if fn != nil {
		s.setupFns = append(s.setupFns, fn)
	}
}

func (s *controllerServer) Add(add *config.NamedConfig) error {
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
			return err
		}

		s.runManager(runner)
	}

	return nil
}

func (s *controllerServer) Delete(del string) error {
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

	return fmt.Errorf("cannot delete non existent cluster %s", del)
}

func (s *controllerServer) Start(ctx context.Context) error {
	s.internalCtx = ctx

	s.errChan = make(chan error)

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()

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
	return nil
}

func (s *controllerServer) GetClient() client.Client {
	return s.client
}

func (s *controllerServer) GetSchema() *runtime.Scheme {
	return s.scheme
}

func (s *controllerServer) GetLogger() logr.Logger {
	return s.logger
}

func (s *controllerServer) addConfig(config *config.NamedConfig) error {
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

func (s *controllerServer) runManager(runner *managerRunner) {
	s.logger.Info("Starting manager", "cluster", runner.config.Name)
	s.runnableManagers.Add(1)

	go func() {
		defer s.runnableManagers.Done()
		if err := runner.Start(s.internalCtx); err != nil {
			s.errChan <- err
		}

		runner.Done()
	}()
}

func (s *controllerServer) setupManager(mgr ctrl.Manager) error {
	for _, fn := range s.setupFns {
		err := fn(mgr)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *controllerServer) getManagerOptions() ctrl.Options {
	options := s.options
	options.MetricsBindAddress = "0"
	options.HealthProbeBindAddress = "0"
	options.LeaderElection = false
	options.WebhookServer = nil
	return options
}
