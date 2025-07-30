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
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	msav1beta1 "open-cluster-management.io/managed-serviceaccount/apis/authentication/v1beta1"
	msacommon "open-cluster-management.io/managed-serviceaccount/pkg/common"
)

const ocmAgentAddon = "ocm-agent-addon"

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

			By("Creating the ManagedServiceAccount addon")
			saAddon := addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterName,
					Name:      msacommon.AddonName,
				},
			}
			Expect(k8sClient.Create(ctx, &saAddon)).Should(Succeed())

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
						Subjects: []rbacv1.Subject{},
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
							Subjects:  []rbacv1.Subject{},
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
			managedSA := msav1beta1.ManagedServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterName,
					Name:      msaName,
				},
			}
			Expect(k8sClient.Create(ctx, &managedSA)).Should(Succeed())

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
						Subjects: []rbacv1.Subject{},
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
							Subjects:  []rbacv1.Subject{},
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
							Subjects:          []rbacv1.Subject{},
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

			By("Creating the ClusterPermission with existing roles")
			clusterPermissionExistingRoles := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterpermission-existing-roles",
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Namespace: "default",
							Name:      "default-rb-" + clusterName,
							RoleRef: cpv1alpha1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "ClusterRole",
								Name:     "argocd-application-controller-1",
							},
							Subjects: []rbacv1.Subject{},
							Subject: rbacv1.Subject{
								Namespace: "openshift-gitops",
								Kind:      "ServiceAccount",
								Name:      "sa-sample-existing",
							},
						},
						{
							Namespace: "kubevirt",
							Name:      "kubevirt-rb-" + clusterName,
							RoleRef: cpv1alpha1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "Role",
								Name:     "argocd-application-controller-2",
							},
							Subjects: []rbacv1.Subject{},
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "User",
								Name:     "user1",
							},
						},
						{
							Namespace: "kubevirt2",
							RoleRef: cpv1alpha1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "Role",
								Name:     "argocd-application-controller-2",
							},
							Subjects: []rbacv1.Subject{},
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "User",
								Name:     "user1",
							},
						},
					},
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						Name: "crb-" + clusterName + "-argo-app-con-3",
						RoleRef: &rbacv1.RoleRef{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "ClusterRole",
							Name:     "argocd-application-controller-3",
						},
						Subjects: []rbacv1.Subject{},
						Subject: rbacv1.Subject{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "Group",
							Name:     "group1",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermissionExistingRoles)).Should(Succeed())
			mwKeyExistingRoles := types.NamespacedName{Name: generateManifestWorkName(clusterPermissionExistingRoles), Namespace: clusterName}
			mwExistingRoles := workv1.ManifestWork{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mwKeyExistingRoles, &mwExistingRoles); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())

			By("Creating a ClusterPermission that has multiple ClusterRoleBindings")
			clusterPermissionMultipleCRBs := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterpermission-multiple-crbs",
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRoleBindings: &[]cpv1alpha1.ClusterRoleBinding{
						{
							Name: "crb-1",
							RoleRef: &rbacv1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "ClusterRole",
								Name:     "argocd-application-controller-1",
							},
							Subjects: []rbacv1.Subject{},
							Subject: rbacv1.Subject{
								Kind: "User",
								Name: "user1",
							},
						},
						{
							Name: "crb-2",
							RoleRef: &rbacv1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "ClusterRole",
								Name:     "argocd-application-controller-2",
							},
							Subjects: []rbacv1.Subject{
								{
									Kind: "User",
									Name: "user2",
								},
								{
									Kind: "Group",
									Name: "group1",
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermissionMultipleCRBs)).Should(Succeed())
			mwKeyMultipleCRBs := types.NamespacedName{Name: generateManifestWorkName(clusterPermissionMultipleCRBs), Namespace: clusterName}
			mwMultipleCRBs := workv1.ManifestWork{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mwKeyMultipleCRBs, &mwMultipleCRBs); err != nil {
					return false
				}
				return true
			}).Should(BeTrue())
		})
	})

	Context("When ClusterPermission is created without prereqs", func() {
		It("Should have error status", func() {
			By("Creating the ClusterPermission with no rules and bindings")
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
						Subjects: []rbacv1.Subject{},
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
							Subjects: []rbacv1.Subject{},
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

			By("Creating the ClusterPermission with missing RoleRef APIGroup")
			clusterPermissionMissingAPIGroup := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterpermission-roleref-missing-apigroup",
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Namespace: "default",
							Name:      "default-rb-" + clusterName,
							RoleRef: cpv1alpha1.RoleRef{
								Kind: "ClusterRole",
								Name: "argocd-application-controller-1",
							},
							Subjects: []rbacv1.Subject{},
							Subject: rbacv1.Subject{
								Namespace: "openshift-gitops",
								Kind:      "ServiceAccount",
								Name:      "sa-sample-existing",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermissionMissingAPIGroup)).Should(Succeed())

			By("Creating the ClusterPermission with missing RoleRef Name")
			clusterPermissionMissingName := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterpermission-roleref-missing-name",
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Namespace: "default",
							Name:      "default-rb-" + clusterName,
							RoleRef: cpv1alpha1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "ClusterRole",
							},
							Subjects: []rbacv1.Subject{},
							Subject: rbacv1.Subject{
								Namespace: "openshift-gitops",
								Kind:      "ServiceAccount",
								Name:      "sa-sample-existing",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermissionMissingName)).Should(Succeed())

			By("Test getSubjects()")
			Expect(len(getSubjects(rbacv1.Subject{}, []rbacv1.Subject{
				{
					Namespace: "openshift-gitops",
					Kind:      "ServiceAccount",
					Name:      "sa-sample-existing",
				},
			}))).Should(Equal(1))

			By("Create ClusterPermission with ClusterRoleBinding that has no subject or subjects")
			clusterPermission = cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterpermission-no-subject-subjects",
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						RoleRef: &rbacv1.RoleRef{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "ClusterRole",
							Name:     "argocd-application-controller-3",
						},
					},
				},
			}
		})
	})
})

func TestGenerateSubjects(t *testing.T) {
	cases := []struct {
		name             string
		subject          rbacv1.Subject
		subjects         []rbacv1.Subject
		clusterNamespace string
		objs             []client.Object
		expectedSubject  rbacv1.Subject
	}{
		{
			name: "subject is not ManagedServiceAccount",
			subject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "test",
				Namespace: "test",
			},
			clusterNamespace: "cluster1",
			objs: []client.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      msacommon.AddonName,
						Namespace: "cluster1",
					},
					Spec: addonv1alpha1.ManagedClusterAddOnSpec{
						InstallNamespace: "test",
					},
					Status: addonv1alpha1.ManagedClusterAddOnStatus{
						Namespace: ocmAgentAddon,
					},
				},
			},
			expectedSubject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "test",
				Namespace: "test",
			},
		},
		{
			name: "subject is ManagedServiceAccount, msa status namespace is not empty",
			subject: rbacv1.Subject{
				Kind:     "ManagedServiceAccount",
				APIGroup: msav1beta1.GroupVersion.Group,
				Name:     "test",
			},
			clusterNamespace: "cluster1",
			objs: []client.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      msacommon.AddonName,
						Namespace: "cluster1",
					},
					Spec: addonv1alpha1.ManagedClusterAddOnSpec{
						InstallNamespace: "test",
					},
					Status: addonv1alpha1.ManagedClusterAddOnStatus{
						Namespace: ocmAgentAddon,
					},
				},
			},
			expectedSubject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				APIGroup:  corev1.GroupName,
				Name:      "test",
				Namespace: ocmAgentAddon,
			},
		},
		{
			name: "subject is ManagedServiceAccount, msa status namespace is empty",
			subject: rbacv1.Subject{
				Kind:     "ManagedServiceAccount",
				APIGroup: msav1beta1.GroupVersion.Group,
				Name:     "test",
			},
			clusterNamespace: "cluster1",
			objs: []client.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      msacommon.AddonName,
						Namespace: "cluster1",
					},
					Spec: addonv1alpha1.ManagedClusterAddOnSpec{
						InstallNamespace: ocmAgentAddon,
					},
				},
			},
			expectedSubject: rbacv1.Subject{
				Kind:      "ServiceAccount",
				APIGroup:  corev1.GroupName,
				Name:      "test",
				Namespace: ocmAgentAddon,
			},
		},
	}

	var testscheme = scheme.Scheme
	err := addonv1alpha1.Install(testscheme)
	if err != nil {
		t.Errorf("Install addon scheme error = %v", err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(testscheme).WithObjects(c.objs...).Build()
			cpr := &ClusterPermissionReconciler{
				Client: fakeClient,
				Scheme: testscheme,
			}

			subjects, err := cpr.generateSubjects(context.TODO(), getSubjects(c.subject, c.subjects), c.clusterNamespace)
			if err != nil {
				t.Errorf("generateSubjects() error = %v", err)
			}

			if len(subjects) == 0 {
				t.Errorf("generateSubjects() return zero subjects")
			}

			if !reflect.DeepEqual(subjects[0], c.expectedSubject) {
				t.Errorf("expected subject %v, got %v", c.expectedSubject, subjects[0])
			}
		})
	}
}
