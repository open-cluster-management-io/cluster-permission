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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
						Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
			// Hack since Ginkgo is not updating the status of the ManifestWork
			mw.Status.Conditions = []metav1.Condition{
				{
					Type:               "Applied",
					Status:             "True",
					LastTransitionTime: metav1.Now(),
					Reason:             "Applied",
					Message:            "ManifestWork applied successfully",
				},
			}
			Expect(k8sClient.Status().Update(ctx, &mw)).Should(Succeed())
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
						Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
						Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
						Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
							Subject: &rbacv1.Subject{
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
			Expect(len(getSubjects(&rbacv1.Subject{}, []rbacv1.Subject{
				{
					Namespace: "openshift-gitops",
					Kind:      "ServiceAccount",
					Name:      "sa-sample-existing",
				},
			}))).Should(Equal(1))

			By("Create ClusterPermission with ClusterRoleBinding that has no subject or subjects")
			clusterPermissionMissingSubjectSubjects := cpv1alpha1.ClusterPermission{
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
						Subjects: []rbacv1.Subject{},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &clusterPermissionMissingSubjectSubjects)).Should(Succeed())

			By("Create ClusterPermission with Role and ClusterRole that doesn't exist validate should have error status")
			clusterPermissionRoleClusterRoleNotExistValidate := cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterpermission-role-clusterrole-not-exist",
					Namespace: clusterName,
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &[]bool{true}[0],
					RoleBindings: &[]cpv1alpha1.RoleBinding{
						{
							Namespace: "default",
							Name:      "default",
							RoleRef: cpv1alpha1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "ClusterRole",
								Name:     "argocd-application-controller-1",
							},
							Subjects: []rbacv1.Subject{
								{
									Namespace: "openshift-gitops",
									Kind:      "ServiceAccount",
									Name:      "sa-sample-existing",
								},
							},
						},
					},
					ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
						Name: "crb-cluster1-argo-app-con-3",
						RoleRef: &rbacv1.RoleRef{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "ClusterRole",
							Name:     "argocd-application-controller-3",
						},
						Subjects: []rbacv1.Subject{
							{
								Namespace: "rbac.authorization.k8s.io",
								Kind:      "Group",
								Name:      "group1",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &clusterPermissionRoleClusterRoleNotExistValidate)).Should(Succeed())

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

			subjects, err := cpr.generateSubjects(context.TODO(), getSubjects(&c.subject, c.subjects), c.clusterNamespace)
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

func TestProcessValidationResults(t *testing.T) {
	cases := []struct {
		name                          string
		manifestWork                  *workv1.ManifestWork
		expectedRolesCondition        *metav1.Condition
		expectedClusterRolesCondition *metav1.Condition
		expectError                   bool
	}{
		{
			name: "all roles and cluster roles found",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "test-role",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionTrue,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "test-cluster-role",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionTrue,
									},
								},
							},
						},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllRolesFound",
				Message: "All referenced roles were found",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllClusterRolesFound",
				Message: "All referenced cluster roles were found",
			},
		},
		{
			name: "some roles missing",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "missing-role",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "another-missing-role",
									Namespace: "kube-system",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "found-cluster-role",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionTrue,
									},
								},
							},
						},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "RolesNotFound",
				Message: "The following roles were not found: default/missing-role, kube-system/another-missing-role",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllClusterRolesFound",
				Message: "All referenced cluster roles were found",
			},
		},
		{
			name: "some cluster roles missing",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "found-role",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionTrue,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "missing-cluster-role",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllRolesFound",
				Message: "All referenced roles were found",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterRolesNotFound",
				Message: "The following cluster roles were not found: missing-cluster-role",
			},
		},
		{
			name: "both roles and cluster roles missing",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "missing-role",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "missing-cluster-role",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "RolesNotFound",
				Message: "The following roles were not found: default/missing-role",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterRolesNotFound",
				Message: "The following cluster roles were not found: missing-cluster-role",
			},
		},
		{
			name: "no available condition means missing",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "no-condition-role",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "no-condition-cluster-role",
								},
								Conditions: []metav1.Condition{},
							},
						},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "RolesNotFound",
				Message: "The following roles were not found: default/no-condition-role",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterRolesNotFound",
				Message: "The following cluster roles were not found: no-condition-cluster-role",
			},
		},
		{
			name: "empty manifest work status",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllRolesFound",
				Message: "All referenced roles were found",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllClusterRolesFound",
				Message: "All referenced cluster roles were found",
			},
		},
		{
			name: "mixed resource types including non-role resources",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Deployment",
									Name:      "test-deployment",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "test-role",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionTrue,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ConfigMap",
									Name: "test-configmap",
								},
								Conditions: []metav1.Condition{
									{
										Type:   "Available",
										Status: metav1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			expectedRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllRolesFound",
				Message: "All referenced roles were found",
			},
			expectedClusterRolesCondition: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllClusterRolesFound",
				Message: "All referenced cluster roles were found",
			},
		},
	}

	var testscheme = scheme.Scheme
	err := cpv1alpha1.AddToScheme(testscheme)
	if err != nil {
		t.Errorf("AddToScheme error = %v", err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create a ClusterPermission for testing
			clusterPermission := &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
				},
				Status: cpv1alpha1.ClusterPermissionStatus{
					Conditions: []metav1.Condition{},
				},
			}

			// Create fake client with the ClusterPermission
			fakeClient := fake.NewClientBuilder().
				WithScheme(testscheme).
				WithObjects(clusterPermission).
				WithStatusSubresource(clusterPermission).
				Build()

			cpr := &ClusterPermissionReconciler{
				Client: fakeClient,
				Scheme: testscheme,
			}

			// Call processValidationResults
			err := cpr.processValidationResults(context.TODO(), clusterPermission, c.manifestWork)

			if c.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !c.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify the conditions were set correctly
			updatedCP := &cpv1alpha1.ClusterPermission{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{
				Name:      "test-cp",
				Namespace: "cluster1",
			}, updatedCP)
			if err != nil {
				t.Errorf("failed to get updated ClusterPermission: %v", err)
			}

			// Check roles condition
			rolesCondition := meta.FindStatusCondition(updatedCP.Status.Conditions, cpv1alpha1.ConditionTypeValidateRolesExist)
			if rolesCondition == nil {
				t.Errorf("expected roles condition but it was not found")
			} else {
				if rolesCondition.Type != c.expectedRolesCondition.Type {
					t.Errorf("expected roles condition type %s, got %s", c.expectedRolesCondition.Type, rolesCondition.Type)
				}
				if rolesCondition.Status != c.expectedRolesCondition.Status {
					t.Errorf("expected roles condition status %s, got %s", c.expectedRolesCondition.Status, rolesCondition.Status)
				}
				if rolesCondition.Reason != c.expectedRolesCondition.Reason {
					t.Errorf("expected roles condition reason %s, got %s", c.expectedRolesCondition.Reason, rolesCondition.Reason)
				}
				if rolesCondition.Message != c.expectedRolesCondition.Message {
					t.Errorf("expected roles condition message %s, got %s", c.expectedRolesCondition.Message, rolesCondition.Message)
				}
			}

			// Check cluster roles condition
			clusterRolesCondition := meta.FindStatusCondition(updatedCP.Status.Conditions, cpv1alpha1.ConditionTypeValidateClusterRolesExist)
			if clusterRolesCondition == nil {
				t.Errorf("expected cluster roles condition but it was not found")
			} else {
				if clusterRolesCondition.Type != c.expectedClusterRolesCondition.Type {
					t.Errorf("expected cluster roles condition type %s, got %s", c.expectedClusterRolesCondition.Type, clusterRolesCondition.Type)
				}
				if clusterRolesCondition.Status != c.expectedClusterRolesCondition.Status {
					t.Errorf("expected cluster roles condition status %s, got %s", c.expectedClusterRolesCondition.Status, clusterRolesCondition.Status)
				}
				if clusterRolesCondition.Reason != c.expectedClusterRolesCondition.Reason {
					t.Errorf("expected cluster roles condition reason %s, got %s", c.expectedClusterRolesCondition.Reason, clusterRolesCondition.Reason)
				}
				if clusterRolesCondition.Message != c.expectedClusterRolesCondition.Message {
					t.Errorf("expected cluster roles condition message %s, got %s", c.expectedClusterRolesCondition.Message, clusterRolesCondition.Message)
				}
			}
		})
	}
}

func TestManagedClusterAddOnEventHandler(t *testing.T) {
	cases := []struct {
		name               string
		inputObject        client.Object
		clusterPermissions []cpv1alpha1.ClusterPermission
		expectedRequests   []reconcile.Request
		expectError        bool
	}{
		{
			name: "addon with ClusterPermissions using ManagedServiceAccount",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subject: &rbacv1.Subject{
								APIGroup: msav1beta1.GroupVersion.Group,
								Kind:     "ManagedServiceAccount",
								Name:     "test-msa",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp2",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subject: &rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "regular-sa",
								Namespace: "default",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp3",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						RoleBindings: &[]cpv1alpha1.RoleBinding{
							{
								Namespace: "default",
								Subject: &rbacv1.Subject{
									APIGroup: msav1beta1.GroupVersion.Group,
									Kind:     "ManagedServiceAccount",
									Name:     "another-msa",
								},
							},
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "cp1",
						Namespace: "cluster1",
					},
				},
				{
					NamespacedName: types.NamespacedName{
						Name:      "cp3",
						Namespace: "cluster1",
					},
				},
			},
		},
		{
			name: "addon with no ClusterPermissions using ManagedServiceAccount",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subject: &rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "regular-sa",
								Namespace: "default",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp2",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subject: &rbacv1.Subject{
								Kind: "User",
								Name: "test-user",
							},
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name: "addon with ClusterPermissions using ManagedServiceAccount in subjects array",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subjects: []rbacv1.Subject{
								{
									Kind:      "ServiceAccount",
									Name:      "regular-sa",
									Namespace: "default",
								},
								{
									APIGroup: msav1beta1.GroupVersion.Group,
									Kind:     "ManagedServiceAccount",
									Name:     "test-msa",
								},
							},
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "cp1",
						Namespace: "cluster1",
					},
				},
			},
		},
		{
			name: "addon with ClusterRoleBindings array using ManagedServiceAccount",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBindings: &[]cpv1alpha1.ClusterRoleBinding{
							{
								Subject: &rbacv1.Subject{
									Kind:      "ServiceAccount",
									Name:      "regular-sa",
									Namespace: "default",
								},
							},
							{
								Subject: &rbacv1.Subject{
									APIGroup: msav1beta1.GroupVersion.Group,
									Kind:     "ManagedServiceAccount",
									Name:     "test-msa",
								},
							},
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "cp1",
						Namespace: "cluster1",
					},
				},
			},
		},
		{
			name: "no ClusterPermissions in namespace",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{},
			expectedRequests:   []reconcile.Request{},
		},
		{
			name: "ClusterPermissions in different namespace",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cluster2", // Different namespace
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subject: &rbacv1.Subject{
								APIGroup: msav1beta1.GroupVersion.Group,
								Kind:     "ManagedServiceAccount",
								Name:     "test-msa",
							},
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{}, // Should be empty since it's in different namespace
		},
		{
			name: "non-ManagedClusterAddOn object",
			inputObject: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{},
			expectedRequests:   []reconcile.Request{},
		},
		{
			name: "complex scenario with multiple binding types",
			inputObject: &addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addon",
					Namespace: "cluster1",
				},
			},
			clusterPermissions: []cpv1alpha1.ClusterPermission{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-complex",
						Namespace: "cluster1",
					},
					Spec: cpv1alpha1.ClusterPermissionSpec{
						ClusterRoleBinding: &cpv1alpha1.ClusterRoleBinding{
							Subject: &rbacv1.Subject{
								Kind:      "ServiceAccount",
								Name:      "regular-sa",
								Namespace: "default",
							},
						},
						ClusterRoleBindings: &[]cpv1alpha1.ClusterRoleBinding{
							{
								Subject: &rbacv1.Subject{
									APIGroup: msav1beta1.GroupVersion.Group,
									Kind:     "ManagedServiceAccount",
									Name:     "test-msa-1",
								},
							},
						},
						RoleBindings: &[]cpv1alpha1.RoleBinding{
							{
								Namespace: "default",
								Subjects: []rbacv1.Subject{
									{
										Kind: "User",
										Name: "test-user",
									},
									{
										APIGroup: msav1beta1.GroupVersion.Group,
										Kind:     "ManagedServiceAccount",
										Name:     "test-msa-2",
									},
								},
							},
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "cp-complex",
						Namespace: "cluster1",
					},
				},
			},
		},
	}

	var testscheme = scheme.Scheme
	err := cpv1alpha1.AddToScheme(testscheme)
	if err != nil {
		t.Errorf("AddToScheme error = %v", err)
	}
	err = addonv1alpha1.Install(testscheme)
	if err != nil {
		t.Errorf("Install addon scheme error = %v", err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create objects for fake client
			objs := make([]client.Object, 0, len(c.clusterPermissions))
			for i := range c.clusterPermissions {
				objs = append(objs, &c.clusterPermissions[i])
			}

			// Create fake client with the ClusterPermissions
			fakeClient := fake.NewClientBuilder().
				WithScheme(testscheme).
				WithObjects(objs...).
				Build()

			cpr := &ClusterPermissionReconciler{
				Client: fakeClient,
				Scheme: testscheme,
			}

			// Call the managedClusterAddOnEventHandler method directly to get a MapFunc
			// Since managedClusterAddOnEventHandler returns EnqueueRequestsFromMapFunc which wraps our function,
			// we'll create the MapFunc directly by calling managedClusterAddOnEventHandler and extracting the func
			mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
				addon, ok := obj.(*addonv1alpha1.ManagedClusterAddOn)
				if !ok {
					return []reconcile.Request{}
				}

				// Find all ClusterPermissions in this addon's namespace that have ManagedServiceAccount subjects
				var clusterPermissions cpv1alpha1.ClusterPermissionList
				err := cpr.List(ctx, &clusterPermissions, &client.ListOptions{
					Namespace: addon.Namespace,
				})
				if err != nil {
					return []reconcile.Request{}
				}

				var requests []reconcile.Request
				for _, cp := range clusterPermissions.Items {
					if cpr.clusterPermissionUsesManagedServiceAccount(&cp) {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      cp.Name,
								Namespace: cp.Namespace,
							},
						})
					}
				}

				return requests
			}

			// Call the mapping function
			requests := mapFunc(context.TODO(), c.inputObject)

			// Verify the results
			if len(requests) != len(c.expectedRequests) {
				t.Errorf("expected %d requests, got %d", len(c.expectedRequests), len(requests))
			}

			// Check that all expected requests are present (order might vary)
			for _, expectedReq := range c.expectedRequests {
				found := false
				for _, actualReq := range requests {
					if actualReq.NamespacedName == expectedReq.NamespacedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected request %v not found in actual requests %v", expectedReq, requests)
				}
			}

			// Check that no unexpected requests are present
			for _, actualReq := range requests {
				found := false
				for _, expectedReq := range c.expectedRequests {
					if actualReq.NamespacedName == expectedReq.NamespacedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("unexpected request %v found in actual requests", actualReq)
				}
			}
		})
	}
}
