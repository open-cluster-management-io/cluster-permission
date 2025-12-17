package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

var _ = ginkgo.Describe("ClusterPermission with ManagedServiceAccount test", ginkgo.Ordered,
	ginkgo.Label("cluster-permission-msa"), func() {

		clusterPermissionName := "cp-msa-e2e-" + rand.String(5)
		msaName := "msa-e2e-" + rand.String(5)
		msaNamespace := "open-cluster-management-agent-addon"

		ginkgo.It("should create rolebinding with ManagedServiceAccount subject", func() {
			ctx := context.TODO()

			// Define the ManagedServiceAccount GVR
			msaGVR := schema.GroupVersionResource{
				Group:    "authentication.open-cluster-management.io",
				Version:  "v1beta1",
				Resource: "managedserviceaccounts",
			}

			// 1. Create a ManagedServiceAccount in the spoke cluster namespace
			ginkgo.By("Creating ManagedServiceAccount in hub cluster")
			managedSA := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "authentication.open-cluster-management.io/v1beta1",
					"kind":       "ManagedServiceAccount",
					"metadata": map[string]interface{}{
						"name":      msaName,
						"namespace": spokeClusterName,
					},
					"spec": map[string]interface{}{
						"rotation": map[string]interface{}{},
					},
				},
			}

			_, err := dynamicClient.Resource(msaGVR).Namespace(spokeClusterName).Create(ctx, managedSA, v1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 2. Check if the ServiceAccount is created on the spoke cluster
			ginkgo.By("Checking if ServiceAccount is created on spoke cluster")
			gomega.Eventually(func() error {
				_, err := spokeKubeClient.CoreV1().ServiceAccounts(msaNamespace).Get(ctx, msaName, v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			// 3. Create a ClusterPermission with ManagedServiceAccount subject rolebinding
			ginkgo.By("Creating ClusterPermission with ManagedServiceAccount subject")
			clusterPermission := &cpv1alpha1.ClusterPermission{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      clusterPermissionName,
					Namespace: spokeClusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Roles: &[]cpv1alpha1.Role{
						{
							Namespace: msaNamespace,
							Rules: []rbacv1.PolicyRule{
								{
									Verbs:     []string{"get", "list"},
									APIGroups: []string{""},
									Resources: []string{"pods"},
								},
							},
						},
					},
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Name:      clusterPermissionName,
							Namespace: msaNamespace,
							Subject: &rbacv1.Subject{
								APIGroup: "authentication.open-cluster-management.io",
								Kind:     "ManagedServiceAccount",
								Name:     msaName,
							},
							RoleRef: cpv1alpha1.RoleRef{
								Kind:     "Role",
								APIGroup: "rbac.authorization.k8s.io",
								Name:     clusterPermissionName,
							},
						},
					},
				},
			}

			_, err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Create(ctx, clusterPermission, v1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 4. Check status of ClusterPermission
			ginkgo.By("Checking ClusterPermission status has AppliedRBACManifestWork condition")
			gomega.Eventually(func() bool {
				cp, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Get(ctx, clusterPermissionName, v1.GetOptions{})
				if err != nil {
					return false
				}
				for _, condition := range cp.Status.Conditions {
					if condition.Type == cpv1alpha1.ConditionTypeAppliedRBACManifestWork && condition.Status == v1.ConditionTrue {
						return true
					}
				}
				return false
			}).Should(gomega.BeTrue())

			// 5. Check if the rolebinding has the correct subject on the spoke cluster
			ginkgo.By("Checking RoleBinding has correct ManagedServiceAccount subject")
			gomega.Eventually(func() bool {
				rb, err := spokeKubeClient.RbacV1().RoleBindings(msaNamespace).Get(ctx, clusterPermissionName, v1.GetOptions{})
				if err != nil {
					return false
				}

				// Verify the rolebinding exists and has the correct subject
				// The ManagedServiceAccount should be translated to a ServiceAccount in the spoke cluster
				if len(rb.Subjects) == 0 {
					return false
				}

				// Check if the subject is the ServiceAccount created by ManagedServiceAccount
				for _, subject := range rb.Subjects {
					if subject.Kind == "ServiceAccount" &&
						subject.Name == msaName &&
						subject.Namespace == msaNamespace {
						return true
					}
				}
				return false
			}).Should(gomega.BeTrue())

			// 6. Verify the Role exists on spoke cluster
			ginkgo.By("Checking Role exists on spoke cluster")
			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().Roles(msaNamespace).Get(ctx, clusterPermissionName, v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			// 7. Cleanup: Delete ClusterPermission
			ginkgo.By("Deleting ClusterPermission")
			err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Delete(ctx, clusterPermissionName, v1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 8. Check if the RoleBinding is deleted
			ginkgo.By("Checking RoleBinding is deleted on spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().RoleBindings(msaNamespace).Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			// 9. Check if the Role is deleted
			ginkgo.By("Checking Role is deleted on spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().Roles(msaNamespace).Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			// 10. Cleanup: Delete ManagedServiceAccount
			ginkgo.By("Deleting ManagedServiceAccount")
			err = dynamicClient.Resource(msaGVR).Namespace(spokeClusterName).Delete(ctx, msaName, v1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 11. Verify ServiceAccount is deleted from spoke cluster
			ginkgo.By("Checking ServiceAccount is deleted from spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.CoreV1().ServiceAccounts(msaNamespace).Get(ctx, msaName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())
		})
	})
