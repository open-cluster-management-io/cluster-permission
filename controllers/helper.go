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

// generateManifestWorkName returns the ManifestWork name for a given ClusterPermission.
// It uses the ClusterPermission name with the suffix of the first 5 characters of the UID
func generateManifestWorkName(clusterPermission cpv1alpha1.ClusterPermission) string {
	return clusterPermission.Name + "-" + string(clusterPermission.UID)[0:5]
}

// buildManifestWork wraps the payloads in a ManifestWork
func buildManifestWork(clusterPermission cpv1alpha1.ClusterPermission, manifestWorkName string,
	clusterRole *rbacv1.ClusterRole,
	clusterRoleBindings []rbacv1.ClusterRoleBinding,
	roles []rbacv1.Role,
	roleBindings []rbacv1.RoleBinding) *workv1.ManifestWork {
	var manifests []workv1.Manifest

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
		},
	}
}
