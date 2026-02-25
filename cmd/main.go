/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/util/flags"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spf13/pflag"
	infrastructurev1alpha1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	"github.com/willemm/cluster-api-provider-scvmm/internal/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(infrastructurev1alpha1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(ipamv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var managerOptions flags.ManagerOptions
	var enableLeaderElection bool
	var probeAddr string
	var machineConcurrency int
	var clusterConcurrency int
	flags.AddManagerOptions(pflag.CommandLine, &managerOptions)
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	// Keep to 1 because of ntlm concurrency issue
	flag.IntVar(&machineConcurrency, "machine-concurrency", 1, "The number of scvmm machines to handle concurrently.")
	flag.IntVar(&clusterConcurrency, "cluster-concurrency", 1, "The number of scvmm cluster objects to handle concurrently.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	tlsOpts, metricsOpts, err := flags.GetManagerOptions(managerOptions)
	if err != nil {
		setupLog.Error(err, "unable to get manager options")
		os.Exit(1)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})
	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                *metricsOpts,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f4af8138.cluster.x-k8s.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ScvmmClusterReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr, concurrency(clusterConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ScvmmCluster")
		os.Exit(1)
	}
	if err = (&controllers.ScvmmMachineReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr, concurrency(machineConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ScvmmMachine")
		os.Exit(1)
	}
	if err = (&controllers.ScvmmProviderReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ScvmmProvider")
		os.Exit(1)
	}
	if err = (&controllers.ScvmmPersistentDiskReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ScvmmPersistentDisk")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	// Keep to 1 until ntlm concurrency issue is fixed:
	// https://github.com/masterzen/winrm/issues/142
	// https://github.com/bodgit/ntlmssp/issues/51
	//controllers.CreateWinrmWorkers(machineConcurrency + clusterConcurrency)
	controllers.CreateWinrmWorkers(1)
	defer controllers.StopWinrmWorkers()

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func concurrency(c int) controller.Options {
	return controller.Options{MaxConcurrentReconciles: c}
}
