package main

import (
	"context"
	"flag"
	"os"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports

	"qmhu/multi-cluster-cr/pkg/configwatcher"
	"qmhu/multi-cluster-cr/pkg/known"
	"qmhu/multi-cluster-cr/pkg/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var configPath string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&configPath, "config-path", "",
		"The path to a directory that contains multiple kubeconfig files or a single kubeconfig.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if len(configPath) == 0 {
		setupLog.Error(nil, "please provide config-path by --config-path")
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// create multi cluster controller server just like controller-runtime Manager
	// server use config to do leaderElection and use options to builder controller-runtime Manager inside
	multiClusterServer, err := server.NewServer(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "80807133.tutorial.kubebuilder.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to new server")
		os.Exit(1)
	}

	podReconciler := PodReconciler{
		Client: multiClusterServer.GetClient(),
		Log:    multiClusterServer.GetLogger(),
		Scheme: multiClusterServer.GetScheme(),
	}

	// add reconciler setup function
	multiClusterServer.AddReconcilerSetup(podReconciler.SetupWithManager)

	serviceReconciler := ServiceReconciler{
		Client: multiClusterServer.GetClient(),
		Log:    multiClusterServer.GetLogger(),
		Scheme: multiClusterServer.GetScheme(),
	}

	// add reconciler setup function
	multiClusterServer.AddReconcilerSetup(serviceReconciler.SetupWithManager)

	// filewatcher watch a filepath and delivery kubeconfig events from it.
	// filepath can be a single kubeconfig file or a directory that contains many kubeconfigs.
	filewatcher, err := configwatcher.NewFileWatcher(configPath)
	if err != nil {
		setupLog.Error(err, "new file watcher failed")
		os.Exit(1)
	}

	go func() {
		// close filewatcher and release resources
		defer filewatcher.Stop()

		for {
			select {
			case event := <-filewatcher.Events():
				if event.Type == configwatcher.Added {
					// add the config to server and start a controller to list&watch the k8s cluster if server started
					err := multiClusterServer.Add(event.Config)
					if err != nil {
						setupLog.Error(err, "add config failed", "name", event.Config.Name)
					}
				}
				if event.Type == configwatcher.Updated {
					// update the existed config to server, will trigger a restart for controllers
					err := multiClusterServer.Update(event.Config)
					if err != nil {
						setupLog.Error(err, "update config failed", "name", event.Config.Name)
					}
				}
				if event.Type == configwatcher.Deleted {
					// delete the config to server and stop to list&watch the k8s cluster if controller exist
					err := multiClusterServer.Delete(event.Config.Name)
					if err != nil {
						setupLog.Error(err, "delete config failed", "name", event.Config.Name)
					}
				}
			case err := <-filewatcher.Errors():
				setupLog.Error(err, "receive error from filewatcher")
			}
		}
	}()

	setupLog.Info("starting server")
	// start server - it'll start controller based on registered clusters
	if err := multiClusterServer.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running server")
		os.Exit(1)
	}
}

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	setupLog.Info("got", "cluster", ctx.Value(known.ClusterContext), "pod", req.NamespacedName)

	// The client use context to decide which cluster to interactive with,
	// You can use the client directly just like single cluster client.
	var pod v1.Pod
	r.Client.Get(ctx, req.NamespacedName, &pod)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Pod{}).
		Complete(r)
}

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	setupLog.Info("got", "cluster", ctx.Value(known.ClusterContext), "service", req.NamespacedName)

	// The client use context to decide which cluster to interactive with,
	// You can use the client directly just like single cluster client.
	var service v1.Service
	r.Client.Get(ctx, req.NamespacedName, &service)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		Complete(r)
}
