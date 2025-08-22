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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

// ValidationRoleRef represents a role reference that needs validation
type ValidationRoleRef struct {
	Name      string
	Namespace string // empty for ClusterRoles
	Kind      string // "Role" or "ClusterRole"
}

// generateManifestWorkName returns the ManifestWork name for a given ClusterPermission.
// It uses the ClusterPermission name with the suffix of the first 5 characters of the UID
func generateManifestWorkName(clusterPermission cpv1alpha1.ClusterPermission) string {
	return clusterPermission.Name + "-" + string(clusterPermission.UID)[0:5]
}

// extractRoleReferencesForValidation extracts all role and clusterrole references that need validation
func extractRoleReferencesForValidation(clusterPermission *cpv1alpha1.ClusterPermission) []ValidationRoleRef {
	var roleRefs []ValidationRoleRef
	seenRefs := make(map[string]bool) // to avoid duplicates

	// Extract ClusterRole references from ClusterRoleBindings
	if clusterPermission.Spec.ClusterRoleBinding != nil && clusterPermission.Spec.ClusterRoleBinding.RoleRef != nil {
		ref := clusterPermission.Spec.ClusterRoleBinding.RoleRef
		if ref.Kind == "ClusterRole" {
			key := "ClusterRole:" + ref.Name
			if !seenRefs[key] {
				roleRefs = append(roleRefs, ValidationRoleRef{
					Name: ref.Name,
					Kind: "ClusterRole",
				})
				seenRefs[key] = true
			}
		}
	}

	if clusterPermission.Spec.ClusterRoleBindings != nil {
		for _, crb := range *clusterPermission.Spec.ClusterRoleBindings {
			if crb.RoleRef != nil && crb.RoleRef.Kind == "ClusterRole" {
				key := "ClusterRole:" + crb.RoleRef.Name
				if !seenRefs[key] {
					roleRefs = append(roleRefs, ValidationRoleRef{
						Name: crb.RoleRef.Name,
						Kind: "ClusterRole",
					})
					seenRefs[key] = true
				}
			}
		}
	}

	// Extract Role references from RoleBindings
	if clusterPermission.Spec.RoleBindings != nil {
		for _, rb := range *clusterPermission.Spec.RoleBindings {
			if rb.RoleRef.Kind == "Role" {
				// For roles, we need to know the namespace
				namespace := rb.Namespace
				if namespace != "" {
					key := "Role:" + namespace + ":" + rb.RoleRef.Name
					if !seenRefs[key] {
						roleRefs = append(roleRefs, ValidationRoleRef{
							Name:      rb.RoleRef.Name,
							Namespace: namespace,
							Kind:      "Role",
						})
						seenRefs[key] = true
					}
				}
			} else if rb.RoleRef.Kind == "ClusterRole" {
				key := "ClusterRole:" + rb.RoleRef.Name
				if !seenRefs[key] {
					roleRefs = append(roleRefs, ValidationRoleRef{
						Name: rb.RoleRef.Name,
						Kind: "ClusterRole",
					})
					seenRefs[key] = true
				}
			}
		}
	}

	return roleRefs
}

// buildManifestWork wraps the payloads in a ManifestWork
func buildManifestWork(clusterPermission cpv1alpha1.ClusterPermission, manifestWorkName string,
	clusterRole *rbacv1.ClusterRole,
	clusterRoleBindings []rbacv1.ClusterRoleBinding,
	roles []rbacv1.Role,
	roleBindings []rbacv1.RoleBinding,
	roleRefs []ValidationRoleRef,
	validateCP bool) *workv1.ManifestWork {
	var manifests []workv1.Manifest
	var manifestConfigs []workv1.ManifestConfigOption

	if clusterRole != nil {
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: clusterRole}})
	}

	if len(clusterRoleBindings) > 0 {
		for i := range clusterRoleBindings {
			manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: &clusterRoleBindings[i]}})
		}
	}

	if len(roles) > 0 {
		for i := range roles {
			manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: &roles[i]}})
		}
	}

	if len(roleBindings) > 0 {
		for i := range roleBindings {
			manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: &roleBindings[i]}})
		}
	}

	if validateCP && len(roleRefs) > 0 {
		for _, roleRef := range roleRefs {
			if roleRef.Kind == "ClusterRole" {
				manifestConfigs = append(manifestConfigs, workv1.ManifestConfigOption{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:    "rbac.authorization.k8s.io",
						Resource: "clusterroles",
						Name:     roleRef.Name,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: "ReadOnly",
					},
				})
				manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: &rbacv1.ClusterRole{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Kind:       "ClusterRole",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: roleRef.Name,
					},
				}}})
			} else if roleRef.Kind == "Role" {
				manifestConfigs = append(manifestConfigs, workv1.ManifestConfigOption{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     "rbac.authorization.k8s.io",
						Resource:  "roles",
						Name:      roleRef.Name,
						Namespace: roleRef.Namespace,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: "ReadOnly",
					},
				})
				manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: &rbacv1.Role{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Kind:       "Role",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleRef.Name,
						Namespace: roleRef.Namespace,
					},
				}}})
			}
		}
	}

	// setup the owner so when ClusterPermission is deleted, the associated ManifestWork is also deleted
	owner := metav1.NewControllerRef(&clusterPermission, cpv1alpha1.GroupVersion.WithKind("ClusterPermission"))

	return &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            manifestWorkName,
			Namespace:       clusterPermission.Namespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
			ManifestConfigs: manifestConfigs,
		},
	}
}
