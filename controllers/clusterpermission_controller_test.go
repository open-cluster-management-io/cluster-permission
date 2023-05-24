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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	msav1alpha1 "open-cluster-management.io/managed-serviceaccount/api/v1alpha1"
	msacommon "open-cluster-management.io/managed-serviceaccount/pkg/common"
)

var _ = Describe("ClusterPermission controller", func() {

	const (
		clusterName = "cluster1"
		mbacName    = "clusterpermission-1"
		msaName     = "managedsa-1"
	)

	mbacKey := types.NamespacedName{Name: mbacName, Namespace: clusterName}
	ctx := context.Background()

	Context("When ClusterPermission is created/updated/deleted", func() {
		It("Should create/update/delete ManifestWork", func() {
			By("Creating the OCM ManagedCluster")
			managedCluster := clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			Expect(k8sClient.Create(ctx, &managedCluster)).Should(Succeed())

			By("Creating the OCM ManagedCluster namespace")
			managedClusterNs := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   clusterName,
					Labels: map[string]string{"a": "b"},
				},
			}
			Expect(k8sClient.Create(ctx, &managedClusterNs)).Should(Succeed())

			By("Creating the ClusterPermission")
			clusterPermission := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{"apps"},
							Resources: []string{"services"},
							Verbs:     []string{"create"},
						}},
					},
					Roles: &[]cpv1alpha1.Role{{
						Namespace: "default",
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
							Verbs:     []string{"get"},
						}},
					}},
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						Subject: rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "klusterlet-work-sa",
							Namespace: "open-cluster-management-agent",
						},
					},
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Namespace: "default",
							RoleRef:   cpv1alpha1.RoleRef{Kind: "ClusterRole"},
							Subject: rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "klusterlet-work-sa",
								Namespace: "open-cluster-management-agent",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			mwKey := types.NamespacedName{Name: generateManifestWorkName(clusterPermission), Namespace: clusterName}
			mw := workv1.ManifestWork{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())

			By("Updating the ClusterPermission")
			time.Sleep(3 * time.Second)
			oldRv := mw.GetResourceVersion()
			Expect(k8sClient.Get(ctx, mbacKey, &clusterPermission)).Should(Succeed())
			clusterPermission.Spec.Roles = nil
			Eventually(func() bool {
				if err := k8sClient.Update(ctx, &clusterPermission); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return false
				}
				return oldRv != mw.GetResourceVersion()
			}).Should(BeTrue())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Get(ctx, mbacKey, &clusterPermission)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Delete(ctx, &clusterPermission); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, mbacKey, &clusterPermission); err != nil {
					return true
				}
				return false
			}).Should(BeTrue())

			By("Creating the ManagedServiceAccount")
			managedSA := msav1alpha1.ManagedServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterName,
					Name:      msaName,
				},
			}
			Expect(k8sClient.Create(ctx, &managedSA)).Should(Succeed())

			By("Creating the ManagedServiceAccount addon")
			saAddon := addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterName,
					Name:      msacommon.AddonName,
				},
			}
			Expect(k8sClient.Create(ctx, &saAddon)).Should(Succeed())

			By("Creating the ClusterPermission with subject ManagedServiceAccount")
			clusterPermission = cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{"apps"},
							Resources: []string{"services"},
							Verbs:     []string{"create"},
						}},
					},
					Roles: &[]cpv1alpha1.Role{{
						Namespace: "default",
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
							Verbs:     []string{"get"},
						}},
					}},
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						Subject: rbacv1.Subject{
							APIGroup: "authentication.open-cluster-management.io",
							Kind:     "ManagedServiceAccount",
							Name:     msaName,
						},
					},
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Namespace: "default",
							RoleRef:   cpv1alpha1.RoleRef{Kind: "ClusterRole"},
							Subject: rbacv1.Subject{
								APIGroup: "authentication.open-cluster-management.io",
								Kind:     "ManagedServiceAccount",
								Name:     msaName,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Delete(ctx, &clusterPermission)).Should(Succeed())

			By("Creating the ClusterPermission with namespaceSelector")
			clusterPermission = cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{"apps"},
							Resources: []string{"services"},
							Verbs:     []string{"create"},
						}},
					},
					Roles: &[]cpv1alpha1.Role{{
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}},
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
							Verbs:     []string{"get"},
						}},
					}},
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}},
							RoleRef:           cpv1alpha1.RoleRef{Kind: "ClusterRole"},
							Subject: rbacv1.Subject{
								APIGroup: "authentication.open-cluster-management.io",
								Kind:     "ManagedServiceAccount",
								Name:     msaName,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())

			k8sClient.Get(ctx, mbacKey, &clusterPermission)
			clusterPermission.Spec = cpv1alpha1.ClusterPermissionSpec{
				ClusterRole: &cpv1alpha1.ClusterRole{
					Rules: []rbacv1.PolicyRule{{
						APIGroups: []string{"apps"},
						Resources: []string{"deployments"},
						Verbs:     []string{"create"},
					}},
				}}
			Expect(k8sClient.Update(ctx, &clusterPermission)).Should(Succeed())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Delete(ctx, &clusterPermission)).Should(Succeed())
		})
	})

	Context("When ClusterPermission is created without prereqs", func() {
		It("Should have error status", func() {
			By("Creating the ClusterPermission with no rules")
			clusterPermission := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: clusterName,
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			mwKey := types.NamespacedName{Name: generateManifestWorkName(clusterPermission), Namespace: clusterName}
			mw := workv1.ManifestWork{}
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return true
				}
				return false
			}).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mbacKey, &clusterPermission); err != nil {
					return false
				}
				if len(clusterPermission.Status.Conditions) > 0 {
					return clusterPermission.Status.Conditions[0].Type == cpv1alpha1.ConditionTypeValidation &&
						clusterPermission.Status.Conditions[0].Status == metav1.ConditionFalse
				}
				return false
			}).Should(BeTrue())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Delete(ctx, &clusterPermission)).Should(Succeed())

			By("Creating the ClusterPermission with invalid ClusterRoleBinding ManagedServiceAccount")
			clusterPermission = cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{"apps"},
							Resources: []string{"services"},
							Verbs:     []string{"create"},
						}},
					},
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						Subject: rbacv1.Subject{
							APIGroup: "authentication.open-cluster-management.io",
							Kind:     "ManagedServiceAccount",
							Name:     msaName + "1",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return true
				}
				return false
			}).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mbacKey, &clusterPermission); err != nil {
					return false
				}
				if len(clusterPermission.Status.Conditions) > 0 {
					return clusterPermission.Status.Conditions[0].Type == cpv1alpha1.ConditionTypeAppliedRBACManifestWork &&
						clusterPermission.Status.Conditions[0].Status == metav1.ConditionFalse
				}
				return false
			}).Should(BeTrue())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Delete(ctx, &clusterPermission)).Should(Succeed())

			By("Creating the ClusterPermission with invalid RoleBinding ManagedServiceAccount")
			clusterPermission = cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{"apps"},
							Resources: []string{"services"},
							Verbs:     []string{"create"},
						}},
					},
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Subject: rbacv1.Subject{
								APIGroup: "authentication.open-cluster-management.io",
								Kind:     "ManagedServiceAccount",
								Name:     msaName,
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return true
				}
				return false
			}).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mbacKey, &clusterPermission); err != nil {
					return false
				}
				if len(clusterPermission.Status.Conditions) > 0 {
					return clusterPermission.Status.Conditions[0].Type == cpv1alpha1.ConditionTypeAppliedRBACManifestWork &&
						clusterPermission.Status.Conditions[0].Status == metav1.ConditionFalse
				}
				return false
			}).Should(BeTrue())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Delete(ctx, &clusterPermission)).Should(Succeed())

			By("Creating the ClusterPermission with invalid managed cluster")
			clusterPermission = cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mbacName,
					Namespace: "default",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRole: &cpv1alpha1.ClusterRole{
						Rules: []rbacv1.PolicyRule{{
							APIGroups: []string{"apps"},
							Resources: []string{"services"},
							Verbs:     []string{"create"},
						}},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermission)).Should(Succeed())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, mwKey, &mw); err != nil {
					return true
				}
				return false
			}).Should(BeTrue())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: mbacName, Namespace: "default"}, &clusterPermission); err != nil {
					return false
				}
				if len(clusterPermission.Status.Conditions) > 0 {
					return clusterPermission.Status.Conditions[0].Type == cpv1alpha1.ConditionTypeValidation &&
						clusterPermission.Status.Conditions[0].Status == metav1.ConditionFalse
				}
				return false
			}).Should(BeTrue())

			By("Deleting the ClusterPermission")
			Expect(k8sClient.Delete(ctx, &clusterPermission)).Should(Succeed())
		})
	})
})
