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
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

func Test_generateManifestWorkName(t *testing.T) {
	type args struct {
		mbac cpv1alpha1.ClusterPermission
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "generate name",
			args: args{
				cpv1alpha1.ClusterPermission{
					ObjectMeta: v1.ObjectMeta{
						Name: "mbac1",
						UID:  "abcdefghijk",
					},
				},
			},
			want: "mbac1-abcde",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateManifestWorkName(tt.args.mbac); got != tt.want {
				t.Errorf("generateManifestWorkName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildManifestWork(t *testing.T) {
	type args struct {
		clusterPermission  cpv1alpha1.ClusterPermission
		manifestWorkName   string
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
		roles              []rbacv1.Role
		roleBindings       []rbacv1.RoleBinding
		roleRefs           []ValidationRoleRef
	}
	type wants struct {
		manifestWorkName              string
		clusterRoleVerb               string
		roleVerb                      string
		clusterRoleBindingSubjectKind string
		roleBindingRoleRef            string
	}
	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "buildManifestWork",
			args: args{
				manifestWorkName: "work-1",
				clusterRole: &rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{{
						APIGroups: []string{"apps"},
						Resources: []string{"services"},
						Verbs:     []string{"create"},
					}},
				},
				clusterRoleBinding: &rbacv1.ClusterRoleBinding{
					Subjects: []rbacv1.Subject{
						{Kind: "Group"},
					},
				},
				roles: []rbacv1.Role{
					{Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
							Verbs:     []string{"get"},
						},
					}},
				},
				roleBindings: []rbacv1.RoleBinding{
					{
						RoleRef: rbacv1.RoleRef{Kind: "ClusterRole"},
					},
				},
				clusterPermission: cpv1alpha1.ClusterPermission{
					ObjectMeta: v1.ObjectMeta{
						Name:      "clusterpermission-1",
						Namespace: "cluster1",
						UID:       types.UID("123456789"),
					},
				},
				roleRefs: []ValidationRoleRef{},
			},
			wants: wants{
				manifestWorkName:              "work-1",
				clusterRoleVerb:               "create",
				roleVerb:                      "get",
				clusterRoleBindingSubjectKind: "Group",
				roleBindingRoleRef:            "ClusterRole",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildManifestWork(tt.args.clusterPermission, tt.args.manifestWorkName, tt.args.clusterRole, []rbacv1.ClusterRoleBinding{*tt.args.clusterRoleBinding},
				tt.args.roles, tt.args.roleBindings, tt.args.roleRefs, false)
			// check work name
			if got.Name != tt.wants.manifestWorkName {
				t.Errorf("buildManifestWork() manifestWorkName = %v, want %v", got.Name, tt.wants.manifestWorkName)
			}

			// check work owner
			if got.OwnerReferences[0].Kind != "ClusterPermission" {
				t.Errorf("buildManifestWork() owner = %v, want %v", got.OwnerReferences[0].Kind, "ClusterPermission")
			}

			// check clusterrole/clusterrolebinding
			unsClusterRole, err := runtime.DefaultUnstructuredConverter.ToUnstructured(got.Spec.Workload.Manifests[0].RawExtension.Object)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			clusterRoleObj := &unstructured.Unstructured{Object: unsClusterRole}
			clusterRole := &rbacv1.ClusterRole{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(clusterRoleObj.Object, clusterRole)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			if clusterRole.Rules[0].Verbs[0] != tt.wants.clusterRoleVerb {
				t.Errorf("buildManifestWork() clusterRoleVerb = %v, want %v", clusterRole.Rules[0].Verbs[0], tt.wants.clusterRoleVerb)
			}
			unsClusterRolebinding, err := runtime.DefaultUnstructuredConverter.ToUnstructured(got.Spec.Workload.Manifests[1].RawExtension.Object)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			clusterRolebindingObj := &unstructured.Unstructured{Object: unsClusterRolebinding}
			clusterRolebing := &rbacv1.ClusterRoleBinding{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(clusterRolebindingObj.Object, clusterRolebing)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			if clusterRolebing.Subjects[0].Kind != tt.wants.clusterRoleBindingSubjectKind {
				t.Errorf("buildManifestWork() subjectKind = %v, want %v", clusterRolebing.Subjects[0].Kind, tt.wants.clusterRoleBindingSubjectKind)
			}

			// check role/rolebinding
			unsRole, err := runtime.DefaultUnstructuredConverter.ToUnstructured(got.Spec.Workload.Manifests[2].RawExtension.Object)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			roleObj := &unstructured.Unstructured{Object: unsRole}
			role := &rbacv1.Role{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(roleObj.Object, role)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			if role.Rules[0].Verbs[0] != tt.wants.roleVerb {
				t.Errorf("buildManifestWork() roleVerb = %v, want %v", role.Rules[0].Verbs[0], tt.wants.roleVerb)
			}
			unsRolebinding, err := runtime.DefaultUnstructuredConverter.ToUnstructured(got.Spec.Workload.Manifests[3].RawExtension.Object)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			rolebindingObj := &unstructured.Unstructured{Object: unsRolebinding}
			rolebinding := &rbacv1.RoleBinding{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(rolebindingObj.Object, rolebinding)
			if err != nil {
				t.Errorf("buildManifestWork() %v", err)
			}
			if rolebinding.RoleRef.Kind != tt.wants.roleBindingRoleRef {
				t.Errorf("buildManifestWork() RoleRefKind = %v, want %v", rolebinding.RoleRef.Kind, tt.wants.roleBindingRoleRef)
			}
		})
	}
}
