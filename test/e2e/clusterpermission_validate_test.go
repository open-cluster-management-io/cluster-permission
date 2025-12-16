package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

var _ = ginkgo.Describe("ClusterPermission validation test", ginkgo.Ordered,
	ginkgo.Label("cluster-permission-validate"), func() {

		ginkgo.Context("When ClusterRole and ClusterRoleBinding exist", func() {
			clusterPermissionName := "cp-validate-cr-exists-" + rand.String(5)
			existingClusterRoleName := "existing-cr-" + rand.String(5)
			existingClusterRoleBindingName := "existing-crb-" + rand.String(5)
			validateEnabled := true

			clusterPermission := &cpv1alpha1.ClusterPermission{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      clusterPermissionName,
					Namespace: spokeClusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &validateEnabled,
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						Subject: &rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "my-service-account",
							Namespace: "default",
						},
						RoleRef: &rbacv1.RoleRef{
							Kind:     "ClusterRole",
							APIGroup: "rbac.authorization.k8s.io",
							Name:     existingClusterRoleName,
						},
					},
				},
			}

			ginkgo.It("should validate ClusterRole and ClusterRoleBinding exist and set correct status", func() {
				ctx := context.TODO()

				// 1. Create the existing ClusterRole on spoke cluster
				ginkgo.By("Creating existing ClusterRole on spoke cluster")
				existingClusterRole := &rbacv1.ClusterRole{
					ObjectMeta: v1.ObjectMeta{
						Name: existingClusterRoleName,
					},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"get", "list"},
							APIGroups: []string{"apps"},
							Resources: []string{"deployments"},
						},
					},
				}
				_, err := spokeKubeClient.RbacV1().ClusterRoles().Create(ctx, existingClusterRole, v1.CreateOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// 2. Create the existing ClusterRoleBinding on spoke cluster
				ginkgo.By("Creating existing ClusterRoleBinding on spoke cluster")
				existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: v1.ObjectMeta{
						Name: existingClusterRoleBindingName,
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      "test-sa",
							Namespace: "default",
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "ClusterRole",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     existingClusterRoleName,
					},
				}
				_, err = spokeKubeClient.RbacV1().ClusterRoleBindings().Create(ctx, existingClusterRoleBinding, v1.CreateOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// 3. Create ClusterPermission with Validate enabled
				ginkgo.By("Creating ClusterPermission with Validate enabled")
				_, err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Create(ctx, clusterPermission, v1.CreateOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// 4. Check status of ClusterPermission - ValidateClusterRolesExist condition should be True
				ginkgo.By("Checking ClusterPermission status has ValidateClusterRolesExist condition as True")
				gomega.Eventually(func() bool {
					cp, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Get(ctx, clusterPermissionName, v1.GetOptions{})
					if err != nil {
						return false
					}
					for _, condition := range cp.Status.Conditions {
						if condition.Type == cpv1alpha1.ConditionTypeValidateClusterRolesExist && condition.Status == v1.ConditionTrue {
							return true
						}
					}
					return false
				}).Should(gomega.BeTrue())

				// 5. Cleanup - Delete ClusterPermission
				ginkgo.By("Deleting ClusterPermission")
				err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Delete(ctx, clusterPermissionName, v1.DeleteOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// 6. Cleanup - Delete existing ClusterRole
				ginkgo.By("Deleting existing ClusterRole on spoke cluster")
				err = spokeKubeClient.RbacV1().ClusterRoles().Delete(ctx, existingClusterRoleName, v1.DeleteOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// 7. Cleanup - Delete existing ClusterRoleBinding
				ginkgo.By("Deleting existing ClusterRoleBinding on spoke cluster")
				err = spokeKubeClient.RbacV1().ClusterRoleBindings().Delete(ctx, existingClusterRoleBindingName, v1.DeleteOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.Context("When Role and RoleBinding do not exist", func() {
			clusterPermissionName := "cp-validate-role-missing-" + rand.String(5)
			nonExistingRoleName := "non-existing-role-" + rand.String(5)
			validateEnabled := true

			clusterPermission := &cpv1alpha1.ClusterPermission{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      clusterPermissionName,
					Namespace: spokeClusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &validateEnabled,
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Subject: &rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "my-service-account",
								Namespace: "default",
							},
							RoleRef: cpv1alpha1.RoleRef{
								Kind:     "Role",
								APIGroup: "rbac.authorization.k8s.io",
								Name:     nonExistingRoleName,
							},
							Namespace: "default",
						},
					},
				},
			}

			ginkgo.It("should validate Role does not exist and set correct status", func() {
				ctx := context.TODO()

				// 1. Ensure the Role does not exist on spoke cluster
				ginkgo.By("Ensuring Role does not exist on spoke cluster")
				_, err := spokeKubeClient.RbacV1().Roles("default").Get(ctx, nonExistingRoleName, v1.GetOptions{})
				gomega.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())

				// 2. Create ClusterPermission with Validate enabled
				ginkgo.By("Creating ClusterPermission with Validate enabled")
				_, err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Create(ctx, clusterPermission, v1.CreateOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// 3. Check status of ClusterPermission - ValidateRolesExist condition should be False
				ginkgo.By("Checking ClusterPermission status has ValidateRolesExist condition as False")
				gomega.Eventually(func() bool {
					cp, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Get(ctx, clusterPermissionName, v1.GetOptions{})
					if err != nil {
						return false
					}
					for _, condition := range cp.Status.Conditions {
						if condition.Type == cpv1alpha1.ConditionTypeValidateRolesExist && condition.Status == v1.ConditionFalse {
							return true
						}
					}
					return false
				}).Should(gomega.BeTrue())

				// 4. Cleanup - Delete ClusterPermission
				ginkgo.By("Deleting ClusterPermission")
				err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Delete(ctx, clusterPermissionName, v1.DeleteOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})
	})
