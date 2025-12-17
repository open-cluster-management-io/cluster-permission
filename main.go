/*
Copyright 2023.

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
	"fmt"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	"open-cluster-management.io/cluster-permission/controllers"
	msav1beta1 "open-cluster-management.io/managed-serviceaccount/apis/authentication/v1beta1"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// Options for command line flag parsing
type Options struct {
	MetricsAddr                 string
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration
}

var options = Options{
	MetricsAddr:                 "",
	LeaderElectionLeaseDuration: 137 * time.Second,
	LeaderElectionRenewDeadline: 107 * time.Second,
	LeaderElectionRetryPeriod:   26 * time.Second,
}

var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	metricsHost = "0.0.0.0"
	metricsPort = 8286
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cpv1alpha1.AddToScheme(scheme))
	utilruntime.Must(msav1beta1.AddToScheme(scheme))
	utilruntime.Must(addonv1alpha1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(workv1.AddToScheme(scheme))
}

func main() {
	var enableLeaderElection bool
	flag.StringVar(
		&options.MetricsAddr,
		"metrics-addr",
		options.MetricsAddr,
		"The address the metric endpoint binds to.",
	)

	flag.DurationVar(
		&options.LeaderElectionLeaseDuration,
		"leader-election-lease-duration",
		options.LeaderElectionLeaseDuration,
		"The duration that non-leader candidates will wait after observing a leadership "+
			"renewal until attempting to acquire leadership of a led but unrenewed leader "+
			"slot. This is effectively the maximum duration that a leader can be stopped "+
			"before it is replaced by another candidate. This is only applicable if leader "+
			"election is enabled.",
	)

	flag.DurationVar(
		&options.LeaderElectionRenewDeadline,
		"leader-election-renew-deadline",
		options.LeaderElectionRenewDeadline,
		"The interval between attempts by the acting master to renew a leadership slot "+
			"before it stops leading. This must be less than or equal to the lease duration. "+
			"This is only applicable if leader election is enabled.",
	)

	flag.DurationVar(
		&options.LeaderElectionRetryPeriod,
		"leader-election-retry-period",
		options.LeaderElectionRetryPeriod,
		"The duration the clients should wait between attempting acquisition and renewal "+
			"of a leadership. This is only applicable if leader election is enabled.",
	)

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Leader election settings",
		"leaseDuration", options.LeaderElectionLeaseDuration,
		"renewDeadline", options.LeaderElectionRenewDeadline,
		"retryPeriod", options.LeaderElectionRetryPeriod)

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		},
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "cluster-permission-leader.open-cluster-management.io",
		LeaderElectionNamespace: "kube-system",
		LeaseDuration:           &options.LeaderElectionLeaseDuration,
		RenewDeadline:           &options.LeaderElectionRenewDeadline,
		RetryPeriod:             &options.LeaderElectionRetryPeriod,
		// Memory optimization: Configure cache options
		Cache: cache.Options{
			// Use a longer resync period for resources that change infrequently
			// Controller-runtime default is 10 hours, which is appropriate for this use case
			SyncPeriod: &[]time.Duration{10 * time.Hour}[0],
			// Configure cache size limits
			DefaultNamespaces: map[string]cache.Config{
				// Only cache specific namespaces if needed
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ClusterPermissionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterPermission")
		os.Exit(1)
	}

	if err = (&controllers.ClusterPermissionStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterPermissionStatus")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
