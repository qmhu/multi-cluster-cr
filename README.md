# Kubernetes Multi-cluster controller-runtime

Multi-cluster controller-runtime is a framework for building Kubernetes Controllers interacting with multiple clusters. 
Multi-cluster controller-runtime do not reinvent a controller framework but developed base on `controller-runtime` and extended to multi-cluster level. 
Most of the usage in `controller-runtime` is similar to Multi-cluster controller-runtime, so you can migrate controller code easily.

Multi-cluster controller-runtime is also a library providing a ConfigWatcher interface that make it easy to work with other Multi-cluster framework like `Kubefed`.

# Features
* [x] Support control multi kubernetes clusters at the same time
* [x] Dynamic `Add` `Update` and `Delete` clusters when your controller is running
* [x] Designed to be similar to `controller-runtime` to make developer easier to migrate single cluster controller to Multi-cluster controller
* [x] Support for Multi-cluster clients by combining multiple clients acting as one
* [x] out-of-box ConfigWatchers to interactive with other Multi-cluster frameworks like `Kubefed`

# Usage
Multi-cluster controller-runtime is easy to use if you are familiar with `controller-runtime`.

## Step1: Installation
First use go get to install the latest version of the library.

```shell
go get -u qmhu/multi-cluster-cr
```

## Step2: Prepare k8s Configs
Secondly you should prepare some k8s configs.

Choice 1: use `ctrl.GetConfigOrDie` to get k8s config and wrapper to a named kubeconfig
```go
config := []config{config.NewNamedConfig("your-cluster-name", ctrl.GetConfigOrDie())}
```

Choice 2: use `config.LoadConfigsFromConfigFile` to load k8s configs from a directory or a file
```go
configs, err := config.LoadConfigsFromConfigFile("~/.kube/config")
if err != nil {
    setupLog.Error(err, "failed to load configs from file.")
    return
}
```

## Step3: Setup server and register configs
The server is similar to `controller-runtime`'s `manager`, let's create it.

```go
// create multi cluster controller server just like controller-runtime Manager
// server use config to do leaderElection and use options to builder controller-runtime Manager inside
multiClusterServer, err := server.NewServer(ctrl.GetConfigOrDie(), ctrl.Options{
    Scheme:                  scheme,
    MetricsBindAddress:      metricsAddr,
    Port:                    9443,
    HealthProbeBindAddress:  probeAddr,
    LeaderElection:          enableLeaderElection,
    LeaderElectionID:        "80807133.tutorial.kubebuilder.io",
    LeaderElectionNamespace: "default",
})
if err != nil {
    setupLog.Error(err, "unable to new server")
    os.Exit(1)
}

// register configs to server
for _, config := range configs {
    err := multiClusterServer.Add(config)
    if err != nil {
        setupLog.Error(err, "unable to add a config to server")
        os.Exit(1)
    }
}
```

## Step4: Initialize and setup reconciler
Create a reconciler object with client, logger, schema.
Add setup function to server, server will execute it when build managers.

```go
podReconciler := PodReconciler{
    Client: multiClusterServer.GetClient(), // Multi-cluster client
    Log:    multiClusterServer.GetLogger(), // server logger
    Scheme: multiClusterServer.GetScheme(), // First cluster's scheme
}

// add reconciler setup function
multiClusterServer.AddReconcilerSetup(podReconciler.SetupWithManager)
```

## Step5: Start server
Start the server will start all registered reconcilers and handle requests with all registered clusters.

```go
// start server - it'll start controller based on registered clusters
if err := multiClusterServer.Start(ctrl.SetupSignalHandler()); err != nil {
    setupLog.Error(err, "problem running server")
    os.Exit(1)
}
```

## Step6: Writer your Reconciler
If the reconciler is written by `controller-runtime` before, you don't need to change any code.

```go
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
```

# Dynamic cluster management
Multi-cluster controller-runtime support add update and delete clusters when your controller is running. So we use a ConfigWatcher to interactive with other Multi-cluster frameworks like `Kubefed`

## KubeFedWatcher
KubeFedWatcher watches all cluster that registered in KubeFed and delivery kubeconfig events to manage controllers. Full code please referring to sample/kubefed

```go
// kubefedWatcher watch KubeFedCluster in hub cluster and delivery kubeconfig events from it.
kubefedWatcher, err := configwatcher.NewKubeFedWatcher(ctrl.GetConfigOrDie())
if err != nil {
    setupLog.Error(err, "new kubefed watcher failed")
    return
}

go func() {
    // close kubefedWatcher and release resources
    defer kubefedWatcher.Stop()

    for {
        select {
        case event := <-kubefedWatcher.Events():
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
        case err := <-kubefedWatcher.Errors():
            setupLog.Error(err, "receive error from kubefedWatcher")
        }
    }
}()
```

## FileWatcher
FileWatcher watches a filepath and delivery kubeconfig events to manage controllers. Full code please referring to sample/filewatcher

```go
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
```

## Customization
You can also manage your clusters by execute `Add` & `Remove` for server. 

Implement your watcher with Interface ConfigWatcher is recommended.
