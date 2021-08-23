package manager

import (
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"qmhu/multi-cluster-cr/pkg/source"
)

type ControllerConfig struct {
	OnCreation func(mgr ctrl.Manager) interface{}
	OnSetup    func(controller interface{}, mgr ctrl.Manager) error
}

type Manager interface {
	ctrl.Manager

	GetCluster(clusterName string) cluster.Cluster

	Recv(e <-chan source.ClusterEvent)

	AddControllerSetup(c ControllerConfig)
}

func NewManager(config *rest.Config, options ctrl.Options) (Manager, error) {
	return nil, nil
}

/*
// returns a new Manager for Handling Multi-cluster Controllers.
// copied from sigs.k8s.io/controller-runtime/pkg/manager/manager.go and modified
func NewManager(config *rest.Config, options ctrl.Options) (Manager, error) {
	// Set default values for options fields
	options = setOptionsDefaults(options)

	cluster, err := cluster.New(config, func(clusterOptions *cluster.Options) {
		clusterOptions.Scheme = options.Scheme
		clusterOptions.MapperProvider = options.MapperProvider
		clusterOptions.Logger = options.Logger
		clusterOptions.SyncPeriod = options.SyncPeriod
		clusterOptions.Namespace = options.Namespace
		clusterOptions.NewCache = options.NewCache
		clusterOptions.NewClient = options.NewClient
		clusterOptions.ClientDisableCacheFor = options.ClientDisableCacheFor
		clusterOptions.DryRunClient = options.DryRunClient
		clusterOptions.EventBroadcaster = options.EventBroadcaster //nolint:staticcheck
	})
	if err != nil {
		return nil, err
	}

	makeBroadcaster := func() (record.EventBroadcaster, bool) {
		return record.NewBroadcaster(), true
	}
	recorderProvider, err := intrec.NewProvider(config, cluster.GetScheme(), options.Logger.WithName("events"), makeBroadcaster)
	if err != nil {
		return nil, err
	}

	// Create the resource lock to enable leader election)
	leaderConfig := options.LeaderElectionConfig
	if leaderConfig == nil {
		leaderConfig = rest.CopyConfig(config)
	}
	resourceLock, err := leaderelection.NewResourceLock(leaderConfig, recorderProvider, leaderelection.Options{
		LeaderElection:             options.LeaderElection,
		LeaderElectionResourceLock: options.LeaderElectionResourceLock,
		LeaderElectionID:           options.LeaderElectionID,
		LeaderElectionNamespace:    options.LeaderElectionNamespace,
	})
	if err != nil {
		return nil, err
	}

	metricsListener, err := metrics.NewListener(options.MetricsBindAddress)
	if err != nil {
		return nil, err
	}

	metricsExtraHandlers := make(map[string]http.Handler)

	healthProbeListener, err := defaultHealthProbeListener(options.HealthProbeBindAddress)
	if err != nil {
		return nil, err
	}

	return &multiClusterManager{
		cluster:                       cluster,
		recorderProvider:              recorderProvider,
		resourceLock:                  resourceLock,
		metricsListener:               metricsListener,
		metricsExtraHandlers:          metricsExtraHandlers,
		controllerOptions:             options.Controller,
		logger:                        options.Logger,
		elected:                       make(chan struct{}),
		port:                          options.Port,
		host:                          options.Host,
		certDir:                       options.CertDir,
		webhookServer:                 options.WebhookServer,
		leaseDuration:                 *options.LeaseDuration,
		renewDeadline:                 *options.RenewDeadline,
		retryPeriod:                   *options.RetryPeriod,
		healthProbeListener:           healthProbeListener,
		readinessEndpointName:         options.ReadinessEndpointName,
		livenessEndpointName:          options.LivenessEndpointName,
		gracefulShutdownTimeout:       *options.GracefulShutdownTimeout,
		internalProceduresStop:        make(chan struct{}),
		leaderElectionStopped:         make(chan struct{}),
		leaderElectionReleaseOnCancel: options.LeaderElectionReleaseOnCancel,
	}, nil
}

const (
	// Values taken from: https://github.com/kubernetes/apiserver/blob/master/pkg/apis/config/v1alpha1/defaults.go
	defaultLeaseDuration          = 15 * time.Second
	defaultRenewDeadline          = 10 * time.Second
	defaultRetryPeriod            = 2 * time.Second
	defaultGracefulShutdownPeriod = 30 * time.Second

	defaultReadinessEndpoint = "/readyz"
	defaultLivenessEndpoint  = "/healthz"
	defaultMetricsEndpoint   = "/metrics"
)

// a private method that copied from sigs.k8s.io/controller-runtime/pkg/manager/manager.go and modified
func setOptionsDefaults(options ctrl.Options) ctrl.Options {
	leaseDuration, renewDeadline, retryPeriod := defaultLeaseDuration, defaultRenewDeadline, defaultRetryPeriod
	if options.LeaseDuration == nil {
		options.LeaseDuration = &leaseDuration
	}

	if options.RenewDeadline == nil {
		options.RenewDeadline = &renewDeadline
	}

	if options.RetryPeriod == nil {
		options.RetryPeriod = &retryPeriod
	}

	if options.ReadinessEndpointName == "" {
		options.ReadinessEndpointName = defaultReadinessEndpoint
	}

	if options.LivenessEndpointName == "" {
		options.LivenessEndpointName = defaultLivenessEndpoint
	}

	if options.GracefulShutdownTimeout == nil {
		gracefulShutdownTimeout := defaultGracefulShutdownPeriod
		options.GracefulShutdownTimeout = &gracefulShutdownTimeout
	}

	if options.Logger == nil {
		options.Logger = logf.RuntimeLog.WithName("manager")
	}

	return options
}

// defaultHealthProbeListener creates the default health probes listener bound to the given address.
// a private method that copied from sigs.k8s.io/controller-runtime/pkg/manager/manager.go
func defaultHealthProbeListener(addr string) (net.Listener, error) {
	if addr == "" || addr == "0" {
		return nil, nil
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("error listening on %s: %v", addr, err)
	}
	return ln, nil
}
*/
