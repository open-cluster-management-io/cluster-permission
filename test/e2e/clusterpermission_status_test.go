package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

var _ = ginkgo.Describe("ClusterPermission deleted conditions test", ginkgo.Ordered,
	ginkgo.Label("cluster-permission-deleted-conditions"), func() {

		clusterPermissionName := "cp-del-e2e-" + rand.String(5)

		ginkgo.It("should remove resource status when ClusterPermission spec is updated to have fewer resources", func() {
			ctx := context.TODO()

			// 1. Create ClusterPermission with ClusterRole and 2 ClusterRoleBindings
			ginkgo.By("Creating ClusterPermission with 1 ClusterRole and 2 ClusterRoleBindings")
			clusterPermission := &cpv1alpha1.ClusterPermission{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      clusterPermissionName,
					Namespace: spokeClusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"get"},
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
							},
						},
					},
					ClusterRoleBindings: &[]cpv1alpha1.ClusterRoleBinding{
						{
							Name: clusterPermissionName + "-binding-1",
							Subject: &rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "my-service-account-1",
								Namespace: "default",
							},
							RoleRef: &rbacv1.RoleRef{
								Kind:     "ClusterRole",
								APIGroup: "rbac.authorization.k8s.io",
								Name:     clusterPermissionName,
							},
						},
						{
							Name: clusterPermissionName + "-binding-2",
							Subject: &rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "my-service-account-2",
								Namespace: "default",
							},
							RoleRef: &rbacv1.RoleRef{
								Kind:     "ClusterRole",
								APIGroup: "rbac.authorization.k8s.io",
								Name:     clusterPermissionName,
							},
						},
					},
				},
			}

			_, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Create(ctx, clusterPermission, v1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 2. Wait for resourceStatus to be populated with 1 ClusterRole and 2 ClusterRoleBindings
			ginkgo.By("Waiting for ClusterPermission resourceStatus to show 1 ClusterRole and 2 ClusterRoleBindings")
			gomega.Eventually(func() bool {
				cp, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Get(ctx, clusterPermissionName, v1.GetOptions{})
				if err != nil {
					return false
				}
				if cp.Status.ResourceStatus == nil {
					return false
				}

				// Verify we have 1 ClusterRole and 2 ClusterRoleBindings
				return len(cp.Status.ResourceStatus.ClusterRoles) == 1 &&
					len(cp.Status.ResourceStatus.ClusterRoleBindings) == 2 &&
					meta.IsStatusConditionTrue(cp.Status.ResourceStatus.ClusterRoles[0].Conditions, cpv1alpha1.ConditionTypeApplied) &&
					meta.IsStatusConditionTrue(cp.Status.ResourceStatus.ClusterRoleBindings[0].Conditions, cpv1alpha1.ConditionTypeApplied) &&
					meta.IsStatusConditionTrue(cp.Status.ResourceStatus.ClusterRoleBindings[1].Conditions, cpv1alpha1.ConditionTypeApplied)
			}).Should(gomega.BeTrue())

			// 3. Update ClusterPermission to remove one ClusterRoleBinding
			ginkgo.By("Updating ClusterPermission to have only 1 ClusterRoleBinding")
			gomega.Eventually(func() error {
				cp, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Get(ctx, clusterPermissionName, v1.GetOptions{})
				if err != nil {
					return err
				}

				// Update spec to have only one ClusterRoleBinding
				cp.Spec.ClusterRoleBindings = &[]cpv1alpha1.ClusterRoleBinding{
					{
						Name: clusterPermissionName + "-binding-1",
						Subject: &rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "my-service-account-1",
							Namespace: "default",
						},
						RoleRef: &rbacv1.RoleRef{
							Kind:     "ClusterRole",
							APIGroup: "rbac.authorization.k8s.io",
							Name:     clusterPermissionName,
						},
					},
				}

				_, err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Update(ctx, cp, v1.UpdateOptions{})
				return err
			}).Should(gomega.Succeed())

			// 4. Verify resourceStatus is updated to show only 1 ClusterRoleBinding
			ginkgo.By("Verifying resourceStatus shows only 1 ClusterRoleBinding after update")
			gomega.Eventually(func() bool {
				cp, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Get(ctx, clusterPermissionName, v1.GetOptions{})
				if err != nil {
					return false
				}
				if cp.Status.ResourceStatus == nil {
					return false
				}

				// Verify we now have 1 ClusterRole and 1 ClusterRoleBinding
				if len(cp.Status.ResourceStatus.ClusterRoles) != 1 {
					return false
				}
				if len(cp.Status.ResourceStatus.ClusterRoleBindings) != 1 {
					return false
				}

				// Verify the remaining ClusterRoleBinding is the correct one
				if cp.Status.ResourceStatus.ClusterRoleBindings[0].Name != clusterPermissionName+"-binding-1" {
					return false
				}

				// Verify conditions are still present
				return meta.IsStatusConditionTrue(cp.Status.ResourceStatus.ClusterRoles[0].Conditions, cpv1alpha1.ConditionTypeApplied) &&
					meta.IsStatusConditionTrue(cp.Status.ResourceStatus.ClusterRoleBindings[0].Conditions, cpv1alpha1.ConditionTypeApplied)
			}).Should(gomega.BeTrue())

			// 5. Verify the deleted ClusterRoleBinding is actually removed from spoke cluster
			ginkgo.By("Verifying deleted ClusterRoleBinding is removed from spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, clusterPermissionName+"-binding-2", v1.GetOptions{})
				return err != nil
			}).Should(gomega.BeTrue())

			// Cleanup: delete clusterPermission
			ginkgo.By("Deleting ClusterPermission")
			err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Delete(ctx, clusterPermissionName, v1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})
