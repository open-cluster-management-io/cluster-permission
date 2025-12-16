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

var _ = ginkgo.Describe("ClusterPermission multiple resources test", ginkgo.Ordered,
	ginkgo.Label("cluster-permission-multiple"), func() {

		clusterPermissionName := "cp-e2e-" + rand.String(5)
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
						Name: "cp-clusterrolebinding-1",
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
						Name: "cp-clusterrolebinding-2",
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
				Roles: &[]cpv1alpha1.Role{
					{
						Namespace: "default",
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"get"},
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
							},
						},
					},
					{
						Namespace: "open-cluster-management",
						Rules: []rbacv1.PolicyRule{
							{
								Verbs:     []string{"get"},
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
							},
						},
					},
				},
				RoleBindings: &[]cpv1alpha1.RoleBinding{
					{
						Name:      "cp-rolebinding-1",
						Namespace: "default",
						Subject: &rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "my-service-account",
							Namespace: "default",
						},
						RoleRef: cpv1alpha1.RoleRef{
							Kind:     "Role",
							APIGroup: "rbac.authorization.k8s.io",
							Name:     clusterPermissionName,
						},
					},
					{
						Name:      "cp-rolebinding-2",
						Namespace: "open-cluster-management",
						Subject: &rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "my-service-account",
							Namespace: "default",
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

		ginkgo.It("should create clusterPermission with multiple resources and verify RBAC resources", func() {
			ctx := context.TODO()

			// 1. create clusterPermission in spokeCluster ns
			ginkgo.By("Creating ClusterPermission")
			_, err := clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Create(ctx, clusterPermission, v1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 2. check if manifestWork of clusterPermission is created
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

			// 3. check status of clusterPermission
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

			// 4. check if the rbac resources are created.
			ginkgo.By("Checking RBAC resources are created on spoke cluster")
			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().ClusterRoles().Get(ctx, clusterPermissionName, v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, "cp-clusterrolebinding-1", v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, "cp-clusterrolebinding-2", v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().Roles("default").Get(ctx, clusterPermissionName, v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().Roles("open-cluster-management").Get(ctx, clusterPermissionName, v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().RoleBindings("default").Get(ctx, "cp-rolebinding-1", v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.RbacV1().RoleBindings("open-cluster-management").Get(ctx, "cp-rolebinding-2", v1.GetOptions{})
				return err
			}).Should(gomega.Succeed())

			// 5. delete clusterPermission
			ginkgo.By("Deleting ClusterPermission")
			err = clusterPermissionClient.ApiV1alpha1().ClusterPermissions(spokeClusterName).Delete(ctx, clusterPermissionName, v1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// 6. check if the manifestWork is deleted
			ginkgo.By("Checking ManifestWork is deleted")
			gomega.Eventually(func() bool {
				_, err := workClient.WorkV1().ManifestWorks(spokeClusterName).Get(ctx, manifestWork.Name, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			// 7. check if the rbac resources are deleted.
			ginkgo.By("Checking RBAC resources are deleted on spoke cluster")
			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().ClusterRoles().Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, "cp-clusterrolebinding-1", v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().ClusterRoleBindings().Get(ctx, "cp-clusterrolebinding-2", v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().Roles("default").Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().Roles("open-cluster-management").Get(ctx, clusterPermissionName, v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().RoleBindings("default").Get(ctx, "cp-rolebinding-1", v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())

			gomega.Eventually(func() bool {
				_, err := spokeKubeClient.RbacV1().RoleBindings("open-cluster-management").Get(ctx, "cp-rolebinding-2", v1.GetOptions{})
				return errors.IsNotFound(err)
			}).Should(gomega.BeTrue())
		})
	})
