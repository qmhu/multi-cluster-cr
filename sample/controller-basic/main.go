package main

import (
	"context"
	"flag"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	"qmhu/multi-cluster-cr/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports

	mcmanager "qmhu/multi-cluster-cr/pkg/manager"
	mcsource "qmhu/multi-cluster-cr/pkg/source"
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
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	clusters, err := mcsource.GetClustersFromConfigFile("/Users/hu/.kube/config")
	if err != nil {
		setupLog.Error(err, "unable to get clusters")
		os.Exit(1)
	}
	clusterSource := mcsource.NewBasicClusterSource(clusters)

	mgr, err := mcmanager.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "80807133.tutorial.kubebuilder.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clusterSource.Register(mgr.EventChannel())
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	podReconciler := PodReconciler{
		Client: mgr.GetClient(),
		Log:    mgr.GetLogger(),
		Scheme: mgr.GetScheme(),
	}

	if err := podReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "problem setup with manager")
		os.Exit(1)
	}

	setupLog.Info("starting source and manager")
	ctx := ctrl.SetupSignalHandler()
	go clusterSource.Start(ctx)

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
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
	r.Log.Info("got", "pod", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *PodReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return builder.NewControllerManagedBy(mgr).
		For(&v1.Pod{}).
		Complete(r)
}
