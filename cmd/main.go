/*
Copyright 2022.

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
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/ratelimiter"
	"github.com/yndd/ndd-runtime/pkg/shared"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	adminv1alpha1 "github.com/yndd/admin/api/v1alpha1"
	"github.com/yndd/admin/internal/controllers"
	"github.com/yndd/admin/pkg/porch2"
	//+kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	//log = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(adminv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var debug bool
	var profiler bool
	var concurrency int
	var pollInterval time.Duration
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&concurrency, "concurrency", 1, "Number of items to process simultaneously")
	flag.DurationVar(&pollInterval, "poll-interval", 1*time.Minute, "Poll interval controls how often an individual resource should be checked for drift.")
	flag.BoolVar(&debug, "debug", false, "Enable debug")
	flag.BoolVar(&profiler, "profile", false, "Enable profiler")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	zlog := zap.New(zap.UseDevMode(debug), zap.JSONEncoder())
	ctrl.SetLogger(zlog)
	logger := logging.NewLogrLogger(zlog.WithName("topo"))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b3cf4aa5.yndd.io",
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
		logger.Debug("unable to start manager", "error", err)
		os.Exit(1)
	}

	porchClient, err := porch2.CreateClient()
	if err != nil {
		logger.Debug("unable to create porch client", "error", err)
		os.Exit(1)
	}

	porchRESTClient, err := porch2.CreateRESTClient()
	if err != nil {
		logger.Debug("unable to create porch client", "error", err)
		os.Exit(1)
	}

	// initialize controllers
	if err := controllers.Setup(mgr, &shared.NddControllerOptions{
		Logger: logger,
		Poll:   pollInterval,
		Copts: controller.Options{
			MaxConcurrentReconciles: concurrency,
			RateLimiter:             ratelimiter.NewDefaultProviderRateLimiter(ratelimiter.DefaultProviderRPS),
		},
		PorchClient:     porchClient,
		PorchRESTClient: porchRESTClient,
	}); err != nil {
		logger.Debug("Cannot add controllers to manager", "error", err)
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Debug("unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Debug("unable to set up ready checkr", "error", err)
		os.Exit(1)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Debug("problem running manager", "error", err)
		os.Exit(1)
	}
}
