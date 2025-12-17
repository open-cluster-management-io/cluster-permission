package e2e

import (
	"context"
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	cpclient "open-cluster-management.io/cluster-permission/client/clientset/versioned"
)

var (
	hubKubeConfig    string
	spokeKubeConfig  string
	spokeClusterName string
)

func init() {
	flag.StringVar(&hubKubeConfig, "hub-kubeconfig", "", "The kubeConfig of the hub cluster")
	flag.StringVar(&spokeKubeConfig, "spoke-kubeconfig", "", "The kubeConfig of the managed cluster")
	flag.StringVar(&spokeClusterName, "spoke-cluster-name", "", "The name of the managed cluster")
}

var (
	hubKubeClient           kubernetes.Interface
	spokeKubeClient         *kubernetes.Clientset
	clusterClient           clusterclient.Interface
	workClient              workv1client.Interface
	addonClient             addonclient.Interface
	clusterPermissionClient cpclient.Interface
	dynamicClient           dynamic.Interface
)

func TestE2E(tt *testing.T) {
	OutputFail := func(message string, callerSkip ...int) {
		Fail(message, callerSkip...)
	}

	RegisterFailHandler(OutputFail)
	RunSpecs(tt, "ocm E2E Suite")
}

var _ = BeforeSuite(func() {
	if hubKubeConfig == "" {
		hubKubeConfig = os.Getenv("HUB_KUBECONFIG")
	}
	if spokeKubeConfig == "" {
		spokeKubeConfig = os.Getenv("SPOKE_KUBECONFIG")
	}
	if spokeClusterName == "" {
		spokeClusterName = os.Getenv("SPOKE_CLUSTER_NAME")
	}

	hubClusterCfg, err := clientcmd.BuildConfigFromFlags("", hubKubeConfig)
	if err != nil {
		Fail(err.Error())
	}

	hubKubeClient, err = kubernetes.NewForConfig(hubClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	clusterClient, err = clusterclient.NewForConfig(hubClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	workClient, err = workv1client.NewForConfig(hubClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	addonClient, err = addonclient.NewForConfig(hubClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	clusterPermissionClient, err = cpclient.NewForConfig(hubClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	dynamicClient, err = dynamic.NewForConfig(hubClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	spokeClusterCfg, err := clientcmd.BuildConfigFromFlags("", spokeKubeConfig)
	if err != nil {
		Fail(err.Error())
	}

	spokeKubeClient, err = kubernetes.NewForConfig(spokeClusterCfg)
	if err != nil {
		Fail(err.Error())
	}

	SetDefaultEventuallyTimeout(90 * time.Second)
	SetDefaultEventuallyPollingInterval(5 * time.Second)

	// check spoke cluster is available
	Eventually(func() error {
		cluster, err := clusterClient.ClusterV1().ManagedClusters().
			Get(context.TODO(), spokeClusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !meta.IsStatusConditionTrue(cluster.Status.Conditions, "ManagedClusterConditionAvailable") {
			return fmt.Errorf("the managedCluster %s is not available", spokeClusterName)
		}
		return nil
	}).Should(Succeed())

	// check managed-serviceaccount addon is available
	Eventually(func() error {
		addon, err := addonClient.AddonV1alpha1().ManagedClusterAddOns(spokeClusterName).
			Get(context.TODO(), "managed-serviceaccount", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !meta.IsStatusConditionTrue(addon.Status.Conditions, "Available") {
			return fmt.Errorf("the managed-serviceaccount addon in cluster %s is not available", spokeClusterName)
		}
		return nil
	}).Should(Succeed())

})

var _ = AfterSuite(func() {
	// cleanup all deployed resources
})

// getManifestWorkOfClusterPermission retrieves the ManifestWork owned by the specified ClusterPermission
func getManifestWorkOfClusterPermission(ctx context.Context, clusterPermissionName, namespace string) (*workv1.ManifestWork, error) {
	// List all ManifestWorks in the namespace
	manifestWorks, err := workClient.WorkV1().ManifestWorks(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ManifestWorks in namespace %s: %w", namespace, err)
	}

	// Iterate through ManifestWorks to find the one owned by the ClusterPermission
	for i := range manifestWorks.Items {
		mw := &manifestWorks.Items[i]
		for _, ownerRef := range mw.OwnerReferences {
			if ownerRef.Kind == "ClusterPermission" &&
				ownerRef.APIVersion == cpv1alpha1.GroupVersion.String() &&
				ownerRef.Name == clusterPermissionName {
				return mw, nil
			}
		}
	}

	return nil, fmt.Errorf("ManifestWork owned by ClusterPermission %s not found in namespace %s", clusterPermissionName, namespace)
}
