package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

var _ = ginkgo.Describe("ClusterPermission with User and Group subjects test", ginkgo.Ordered,
	ginkgo.Label("cluster-permission-users-groups"), func() {

		clusterPermissionName := "cp-ug-e2e-" + rand.String(5)
		testUser := "test-user@example.com"
		testGroup := "test-group"

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
							Verbs:     []string{"get", "list"},
							APIGroups: []string{""},
							Resources: []string{"pods"},
						},
						{
							Verbs:     []string{"get", "list"},
							APIGroups: []string{"apps"},
							Resources: []string{"deployments"},
						},
					},
				},
				ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
					Subject: &rbacv1.Subject{
						Kind:     "User",
						Name:     testUser,
						APIGroup: "rbac.authorization.k8s.io",
					},
					RoleRef: &rbacv1.RoleRef{
						Kind:     "ClusterRole",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     clusterPermissionName,
					},
				},
				Roles: &[]cpv1alpha1.Role{
					{
						Namespace: "default",
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"get", "list", "watch"},
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
							},
						},
					},
				},
				RoleBindings: &[]cpv1alpha1.RoleBinding{
					{
						Subject: &rbacv1.Subject{
							Kind:     "Group",
							Name:     testGroup,
							APIGroup: "rbac.authorization.k8s.io",
						},
						RoleRef: cpv1alpha1.RoleRef{
							Kind:     "Role",
							APIGroup: "rbac.authorization.k8s.io",
							Name:     clusterPermissionName,
						},
						Namespace: "default",
					},
				},
			},
		}

		ginkgo.It("should create ClusterPermission with User and Group subjects and verify RBAC bindings", func() {
			ctx := context.TODO()

			// 1. create clusterPermission in spokeCluster ns
			ginkgo.By("Creating ClusterPermission with User and Group subjects")
			_, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Create(ctx, clusterPermission, v1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 2. check status of clusterPermission
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

			// 3. check if manifestWork of clusterPermission is created
			ginkgo.By("Checking ManifestWork is created")
			var manifestWork *workv1.ManifestWork
			gomega.Eventually(func() error {
				mw, err := getManifestWorkOfClusterPermission(ctx, clusterPermissionName, spokeClusterName)
				if err != nil {
					return err
				}
				manifestWork = mw
				return nil
			}).Should(gomega.Succeed())

			gomega.Expect(manifestWork).ToNot(gomega.BeNil())
			gomega.Expect(manifestWork.Spec.Workload.Manifests).ToNot(gomega.BeEmpty())

			// 4. check if the RBAC resources are created on spoke cluster
			ginkgo.By("Checking ClusterRole is created on spoke cluster")
			var clusterRole *rbacv1.ClusterRole
			gomega.Eventually(func() error {
				cr, err := spokeKubeClient.RbacV1().ClusterRoles().Get(ctx, clusterPermissionName, v1.GetOptions{})
				clusterRole = cr
				return err
			}).Should(gomega.Succeed())
			gomega.Expect(clusterRole.Rules).To(gomega.HaveLen(2))

			ginkgo.By("Checking ClusterRoleBinding with User subject is created on spoke cluster")
			var clusterRoleBinding *rbacv1.ClusterRoleBinding
			gomega.Eventually(func() error {
				crb, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, clusterPermissionName, v1.GetOptions{})
				clusterRoleBinding = crb
				return err
			}).Should(gomega.Succeed())

			// Verify ClusterRoleBinding has correct User subject
			gomega.Expect(clusterRoleBinding.Subjects).To(gomega.HaveLen(1))
			gomega.Expect(clusterRoleBinding.Subjects[0].Kind).To(gomega.Equal("User"))
			gomega.Expect(clusterRoleBinding.Subjects[0].Name).To(gomega.Equal(testUser))
			gomega.Expect(clusterRoleBinding.Subjects[0].APIGroup).To(gomega.Equal("rbac.authorization.k8s.io"))
			gomega.Expect(clusterRoleBinding.RoleRef.Kind).To(gomega.Equal("ClusterRole"))
			gomega.Expect(clusterRoleBinding.RoleRef.Name).To(gomega.Equal(clusterPermissionName))

			ginkgo.By("Checking Role is created on spoke cluster")
			var role *rbacv1.Role
			gomega.Eventually(func() error {
				r, err := spokeKubeClient.RbacV1().Roles("default").Get(ctx, clusterPermissionName, v1.GetOptions{})
				role = r
				return err
			}).Should(gomega.Succeed())
			gomega.Expect(role.Rules).To(gomega.HaveLen(1))

			ginkgo.By("Checking RoleBinding with Group subject is created on spoke cluster")
			var roleBinding *rbacv1.RoleBinding
			gomega.Eventually(func() error {
				rb, err := spokeKubeClient.RbacV1().RoleBindings("default").Get(ctx, clusterPermissionName, v1.GetOptions{})
				roleBinding = rb
				return err
			}).Should(gomega.Succeed())

			// Verify RoleBinding has correct Group subject
			gomega.Expect(roleBinding.Subjects).To(gomega.HaveLen(1))
			gomega.Expect(roleBinding.Subjects[0].Kind).To(gomega.Equal("Group"))
			gomega.Expect(roleBinding.Subjects[0].Name).To(gomega.Equal(testGroup))
			gomega.Expect(roleBinding.Subjects[0].APIGroup).To(gomega.Equal("rbac.authorization.k8s.io"))
			gomega.Expect(roleBinding.RoleRef.Kind).To(gomega.Equal("Role"))
			gomega.Expect(roleBinding.RoleRef.Name).To(gomega.Equal(clusterPermissionName))

			// 5. delete clusterPermission
			ginkgo.By("Deleting ClusterPermission")
			err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Delete(ctx, clusterPermissionName, v1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 6. check if manifestWork of clusterPermission is deleted
			ginkgo.By("Checking ManifestWork is deleted")
			gomega.Eventually(func() bool {
				_, err := workClient.WorkV1().ManifestWorks(spokeClusterName).Get(ctx, manifestWork.Name, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			// 7. check if the rbac resources are deleted from spoke cluster
			ginkgo.By("Checking ClusterRole is deleted from spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().ClusterRoles().Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			ginkgo.By("Checking ClusterRoleBinding is deleted from spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			ginkgo.By("Checking Role is deleted from spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().Roles("default").Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			ginkgo.By("Checking RoleBinding is deleted from spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().RoleBindings("default").Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())
		})
	})
