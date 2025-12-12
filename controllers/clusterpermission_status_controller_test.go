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
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
)

func TestUpdateValidateConditions(t *testing.T) {
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

			cpsr := &ClusterPermissionStatusReconciler{
				Client: fakeClient,
				Scheme: testscheme,
			}

			// Call updateValidateConditions
			var conditions []metav1.Condition
			err := cpsr.updateValidateConditions(&conditions, c.manifestWork)

			if c.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !c.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check roles condition
			rolesCondition := meta.FindStatusCondition(conditions, cpv1alpha1.ConditionTypeValidateRolesExist)
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
			clusterRolesCondition := meta.FindStatusCondition(conditions, cpv1alpha1.ConditionTypeValidateClusterRolesExist)
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

func TestReconcile(t *testing.T) {
	trueVal := true
	falseVal := false

	cases := []struct {
		name                                  string
		clusterPermission                     *cpv1alpha1.ClusterPermission
		manifestWork                          *workv1.ManifestWork
		expectedAppliedRBACManifestWorkCond   *metav1.Condition
		expectedValidateRolesExistCond        *metav1.Condition
		expectedValidateClusterRolesExistCond *metav1.Condition
		expectedResourceStatus                *cpv1alpha1.ResourceStatus
		expectError                           bool
	}{
		{
			name: "AppliedRBACManifestWork condition added when ManifestWork Applied condition is true",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &falseVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionTrue,
							Reason:  "AppliedManifestWorkComplete",
							Message: "Apply manifest work complete",
						},
					},
				},
			},
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionTrue,
				Reason: "AppliedManifestWorkComplete",
				Message: "Apply manifest work complete" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
		},
		{
			name: "AppliedRBACManifestWork condition false when ManifestWork Applied condition is false",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &falseVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionFalse,
							Reason:  "ApplyManifestFailed",
							Message: "Failed to apply manifest",
						},
					},
				},
			},
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionFalse,
				Reason: "ApplyManifestFailed",
				Message: "Failed to apply manifest" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
		},
		{
			name: "AppliedRBACManifestWork condition false when ManifestWork Applied condition is missing",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &falseVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{},
				},
			},
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionFalse,
				Reason: "ApplyRBACManifestWorkNotComplete",
				Message: "Apply RBAC manifestWork not complete" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
		},
		{
			name: "ResourceStatus added with correct conditions when validate is false",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &falseVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionTrue,
							Reason:  "AppliedManifestWorkComplete",
							Message: "Apply manifest work complete",
						},
					},
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "test-cluster-role",
								},
								Conditions: []metav1.Condition{
									{
										Type:   workv1.ManifestApplied,
										Status: metav1.ConditionTrue,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRoleBinding",
									Name: "test-cluster-role-binding",
								},
								Conditions: []metav1.Condition{
									{
										Type:   workv1.ManifestApplied,
										Status: metav1.ConditionTrue,
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
										Type:   workv1.ManifestApplied,
										Status: metav1.ConditionFalse,
									},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "RoleBinding",
									Name:      "test-role-binding",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{
										Type:   workv1.ManifestApplied,
										Status: metav1.ConditionTrue,
									},
								},
							},
						},
					},
				},
			},
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionTrue,
				Reason: "AppliedManifestWorkComplete",
				Message: "Apply manifest work complete" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
			expectedResourceStatus: &cpv1alpha1.ResourceStatus{
				ClusterRoles: []cpv1alpha1.ClusterRoleStatus{
					{
						Name: "test-cluster-role",
						Conditions: []metav1.Condition{
							{
								Type:    cpv1alpha1.ConditionTypeApplied,
								Status:  metav1.ConditionTrue,
								Reason:  "AppliedManifestComplete",
								Message: "Apply manifest complete",
							},
						},
					},
				},
				ClusterRoleBindings: []cpv1alpha1.ClusterRoleBindingStatus{
					{
						Name: "test-cluster-role-binding",
						Conditions: []metav1.Condition{
							{
								Type:    cpv1alpha1.ConditionTypeApplied,
								Status:  metav1.ConditionTrue,
								Reason:  "AppliedManifestComplete",
								Message: "Apply manifest complete",
							},
						},
					},
				},
				Roles: []cpv1alpha1.RoleStatus{
					{
						Name:      "test-role",
						Namespace: "default",
						Conditions: []metav1.Condition{
							{
								Type:    cpv1alpha1.ConditionTypeApplied,
								Status:  metav1.ConditionFalse,
								Reason:  "FailedApplyManifest",
								Message: "",
							},
						},
					},
				},
				RoleBindings: []cpv1alpha1.RoleBindingStatus{
					{
						Name:      "test-role-binding",
						Namespace: "default",
						Conditions: []metav1.Condition{
							{
								Type:    cpv1alpha1.ConditionTypeApplied,
								Status:  metav1.ConditionTrue,
								Reason:  "AppliedManifestComplete",
								Message: "Apply manifest complete",
							},
						},
					},
				},
			},
		},
		{
			name: "Validation conditions added when validate is true - all roles found",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &trueVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionTrue,
							Reason:  "AppliedManifestWorkComplete",
							Message: "Apply manifest work complete",
						},
					},
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
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionTrue,
				Reason: "AppliedManifestWorkComplete",
				Message: "Apply manifest work complete" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
			expectedValidateRolesExistCond: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllRolesFound",
				Message: "All referenced roles were found",
			},
			expectedValidateClusterRolesExistCond: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllClusterRolesFound",
				Message: "All referenced cluster roles were found",
			},
		},
		{
			name: "Validation conditions added when validate is true - some roles missing",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &trueVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionTrue,
							Reason:  "AppliedManifestWorkComplete",
							Message: "Apply manifest work complete",
						},
					},
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
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionTrue,
				Reason: "AppliedManifestWorkComplete",
				Message: "Apply manifest work complete" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
			expectedValidateRolesExistCond: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "RolesNotFound",
				Message: "The following roles were not found: default/missing-role",
			},
			expectedValidateClusterRolesExistCond: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterRolesNotFound",
				Message: "The following cluster roles were not found: missing-cluster-role",
			},
		},
		{
			name: "Validation conditions added when validate is true - mixed results",
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "cluster1",
					UID:       "test-uid-12345",
				},
				Spec: cpv1alpha1.ClusterPermissionSpec{
					Validate: &trueVal,
				},
			},
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionTrue,
							Reason:  "AppliedManifestWorkComplete",
							Message: "Apply manifest work complete",
						},
					},
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
			expectedAppliedRBACManifestWorkCond: &metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionTrue,
				Reason: "AppliedManifestWorkComplete",
				Message: "Apply manifest work complete" + "\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n cluster1 get ManifestWork test-mw -o yaml",
			},
			expectedValidateRolesExistCond: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
				Status:  metav1.ConditionTrue,
				Reason:  "AllRolesFound",
				Message: "All referenced roles were found",
			},
			expectedValidateClusterRolesExistCond: &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterRolesNotFound",
				Message: "The following cluster roles were not found: missing-cluster-role",
			},
		},
	}

	// Setup scheme
	var testscheme = scheme.Scheme
	err := cpv1alpha1.AddToScheme(testscheme)
	if err != nil {
		t.Fatalf("AddToScheme error = %v", err)
	}
	err = workv1.Install(testscheme)
	if err != nil {
		t.Fatalf("Install workv1 error = %v", err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create fake client with objects
			fakeClient := fake.NewClientBuilder().
				WithScheme(testscheme).
				WithObjects(c.clusterPermission, c.manifestWork).
				WithStatusSubresource(c.clusterPermission, c.manifestWork).
				Build()

			cpsr := &ClusterPermissionStatusReconciler{
				Client: fakeClient,
				Scheme: testscheme,
			}

			// Call Reconcile
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      c.manifestWork.Name,
					Namespace: c.manifestWork.Namespace,
				},
			}

			_, err := cpsr.Reconcile(context.Background(), req)

			if c.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !c.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify ClusterPermission status was updated
			var updatedCP cpv1alpha1.ClusterPermission
			err = fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      c.clusterPermission.Name,
				Namespace: c.clusterPermission.Namespace,
			}, &updatedCP)
			if err != nil {
				t.Fatalf("failed to get updated ClusterPermission: %v", err)
			}

			// Check AppliedRBACManifestWork condition
			if c.expectedAppliedRBACManifestWorkCond != nil {
				appliedCond := meta.FindStatusCondition(updatedCP.Status.Conditions, cpv1alpha1.ConditionTypeAppliedRBACManifestWork)
				if appliedCond == nil {
					t.Errorf("expected AppliedRBACManifestWork condition but it was not found")
				} else {
					if appliedCond.Type != c.expectedAppliedRBACManifestWorkCond.Type {
						t.Errorf("expected AppliedRBACManifestWork condition type %s, got %s",
							c.expectedAppliedRBACManifestWorkCond.Type, appliedCond.Type)
					}
					if appliedCond.Status != c.expectedAppliedRBACManifestWorkCond.Status {
						t.Errorf("expected AppliedRBACManifestWork condition status %s, got %s",
							c.expectedAppliedRBACManifestWorkCond.Status, appliedCond.Status)
					}
					if appliedCond.Reason != c.expectedAppliedRBACManifestWorkCond.Reason {
						t.Errorf("expected AppliedRBACManifestWork condition reason %s, got %s",
							c.expectedAppliedRBACManifestWorkCond.Reason, appliedCond.Reason)
					}
					if appliedCond.Message != c.expectedAppliedRBACManifestWorkCond.Message {
						t.Errorf("expected AppliedRBACManifestWork condition message %s, got %s",
							c.expectedAppliedRBACManifestWorkCond.Message, appliedCond.Message)
					}
				}
			}

			// Check ValidateRolesExist condition
			if c.expectedValidateRolesExistCond != nil {
				rolesExistCond := meta.FindStatusCondition(updatedCP.Status.Conditions, cpv1alpha1.ConditionTypeValidateRolesExist)
				if rolesExistCond == nil {
					t.Errorf("expected ValidateRolesExist condition but it was not found")
				} else {
					if rolesExistCond.Type != c.expectedValidateRolesExistCond.Type {
						t.Errorf("expected ValidateRolesExist condition type %s, got %s",
							c.expectedValidateRolesExistCond.Type, rolesExistCond.Type)
					}
					if rolesExistCond.Status != c.expectedValidateRolesExistCond.Status {
						t.Errorf("expected ValidateRolesExist condition status %s, got %s",
							c.expectedValidateRolesExistCond.Status, rolesExistCond.Status)
					}
					if rolesExistCond.Reason != c.expectedValidateRolesExistCond.Reason {
						t.Errorf("expected ValidateRolesExist condition reason %s, got %s",
							c.expectedValidateRolesExistCond.Reason, rolesExistCond.Reason)
					}
					if rolesExistCond.Message != c.expectedValidateRolesExistCond.Message {
						t.Errorf("expected ValidateRolesExist condition message %s, got %s",
							c.expectedValidateRolesExistCond.Message, rolesExistCond.Message)
					}
				}
			}

			// Check ValidateClusterRolesExist condition
			if c.expectedValidateClusterRolesExistCond != nil {
				clusterRolesExistCond := meta.FindStatusCondition(updatedCP.Status.Conditions, cpv1alpha1.ConditionTypeValidateClusterRolesExist)
				if clusterRolesExistCond == nil {
					t.Errorf("expected ValidateClusterRolesExist condition but it was not found")
				} else {
					if clusterRolesExistCond.Type != c.expectedValidateClusterRolesExistCond.Type {
						t.Errorf("expected ValidateClusterRolesExist condition type %s, got %s",
							c.expectedValidateClusterRolesExistCond.Type, clusterRolesExistCond.Type)
					}
					if clusterRolesExistCond.Status != c.expectedValidateClusterRolesExistCond.Status {
						t.Errorf("expected ValidateClusterRolesExist condition status %s, got %s",
							c.expectedValidateClusterRolesExistCond.Status, clusterRolesExistCond.Status)
					}
					if clusterRolesExistCond.Reason != c.expectedValidateClusterRolesExistCond.Reason {
						t.Errorf("expected ValidateClusterRolesExist condition reason %s, got %s",
							c.expectedValidateClusterRolesExistCond.Reason, clusterRolesExistCond.Reason)
					}
					if clusterRolesExistCond.Message != c.expectedValidateClusterRolesExistCond.Message {
						t.Errorf("expected ValidateClusterRolesExist condition message %s, got %s",
							c.expectedValidateClusterRolesExistCond.Message, clusterRolesExistCond.Message)
					}
				}
			}

			// Check ResourceStatus
			if c.expectedResourceStatus != nil {
				if updatedCP.Status.ResourceStatus == nil {
					t.Errorf("expected ResourceStatus to be populated but it was nil")
				} else {
					// Check ClusterRoles
					if len(c.expectedResourceStatus.ClusterRoles) != len(updatedCP.Status.ResourceStatus.ClusterRoles) {
						t.Errorf("expected %d ClusterRoles, got %d",
							len(c.expectedResourceStatus.ClusterRoles), len(updatedCP.Status.ResourceStatus.ClusterRoles))
					} else {
						for i, expectedCR := range c.expectedResourceStatus.ClusterRoles {
							actualCR := updatedCP.Status.ResourceStatus.ClusterRoles[i]
							if expectedCR.Name != actualCR.Name {
								t.Errorf("expected ClusterRole name %s, got %s", expectedCR.Name, actualCR.Name)
							}
							if len(expectedCR.Conditions) != len(actualCR.Conditions) {
								t.Errorf("expected %d conditions for ClusterRole %s, got %d",
									len(expectedCR.Conditions), expectedCR.Name, len(actualCR.Conditions))
							}
							for j, expectedCond := range expectedCR.Conditions {
								actualCond := actualCR.Conditions[j]
								if expectedCond.Type != actualCond.Type {
									t.Errorf("expected condition type %s, got %s", expectedCond.Type, actualCond.Type)
								}
								if expectedCond.Status != actualCond.Status {
									t.Errorf("expected condition status %s, got %s", expectedCond.Status, actualCond.Status)
								}
								if expectedCond.Reason != actualCond.Reason {
									t.Errorf("expected condition reason %s, got %s", expectedCond.Reason, actualCond.Reason)
								}
								if expectedCond.Message != actualCond.Message {
									t.Errorf("expected condition message %s, got %s", expectedCond.Message, actualCond.Message)
								}
							}
						}
					}

					// Check ClusterRoleBindings
					if len(c.expectedResourceStatus.ClusterRoleBindings) != len(updatedCP.Status.ResourceStatus.ClusterRoleBindings) {
						t.Errorf("expected %d ClusterRoleBindings, got %d",
							len(c.expectedResourceStatus.ClusterRoleBindings), len(updatedCP.Status.ResourceStatus.ClusterRoleBindings))
					} else {
						for i, expectedCRB := range c.expectedResourceStatus.ClusterRoleBindings {
							actualCRB := updatedCP.Status.ResourceStatus.ClusterRoleBindings[i]
							if expectedCRB.Name != actualCRB.Name {
								t.Errorf("expected ClusterRoleBinding name %s, got %s", expectedCRB.Name, actualCRB.Name)
							}
							if len(expectedCRB.Conditions) != len(actualCRB.Conditions) {
								t.Errorf("expected %d conditions for ClusterRoleBinding %s, got %d",
									len(expectedCRB.Conditions), expectedCRB.Name, len(actualCRB.Conditions))
							}
							for j, expectedCond := range expectedCRB.Conditions {
								actualCond := actualCRB.Conditions[j]
								if expectedCond.Type != actualCond.Type {
									t.Errorf("expected condition type %s, got %s", expectedCond.Type, actualCond.Type)
								}
								if expectedCond.Status != actualCond.Status {
									t.Errorf("expected condition status %s, got %s", expectedCond.Status, actualCond.Status)
								}
								if expectedCond.Reason != actualCond.Reason {
									t.Errorf("expected condition reason %s, got %s", expectedCond.Reason, actualCond.Reason)
								}
								if expectedCond.Message != actualCond.Message {
									t.Errorf("expected condition message %s, got %s", expectedCond.Message, actualCond.Message)
								}
							}
						}
					}

					// Check Roles
					if len(c.expectedResourceStatus.Roles) != len(updatedCP.Status.ResourceStatus.Roles) {
						t.Errorf("expected %d Roles, got %d",
							len(c.expectedResourceStatus.Roles), len(updatedCP.Status.ResourceStatus.Roles))
					} else {
						for i, expectedR := range c.expectedResourceStatus.Roles {
							actualR := updatedCP.Status.ResourceStatus.Roles[i]
							if expectedR.Name != actualR.Name {
								t.Errorf("expected Role name %s, got %s", expectedR.Name, actualR.Name)
							}
							if expectedR.Namespace != actualR.Namespace {
								t.Errorf("expected Role namespace %s, got %s", expectedR.Namespace, actualR.Namespace)
							}
							if len(expectedR.Conditions) != len(actualR.Conditions) {
								t.Errorf("expected %d conditions for Role %s/%s, got %d",
									len(expectedR.Conditions), expectedR.Namespace, expectedR.Name, len(actualR.Conditions))
							}
							for j, expectedCond := range expectedR.Conditions {
								actualCond := actualR.Conditions[j]
								if expectedCond.Type != actualCond.Type {
									t.Errorf("expected condition type %s, got %s", expectedCond.Type, actualCond.Type)
								}
								if expectedCond.Status != actualCond.Status {
									t.Errorf("expected condition status %s, got %s", expectedCond.Status, actualCond.Status)
								}
								if expectedCond.Reason != actualCond.Reason {
									t.Errorf("expected condition reason %s, got %s", expectedCond.Reason, actualCond.Reason)
								}
								if expectedCond.Message != actualCond.Message {
									t.Errorf("expected condition message %s, got %s", expectedCond.Message, actualCond.Message)
								}
							}
						}
					}

					// Check RoleBindings
					if len(c.expectedResourceStatus.RoleBindings) != len(updatedCP.Status.ResourceStatus.RoleBindings) {
						t.Errorf("expected %d RoleBindings, got %d",
							len(c.expectedResourceStatus.RoleBindings), len(updatedCP.Status.ResourceStatus.RoleBindings))
					} else {
						for i, expectedRB := range c.expectedResourceStatus.RoleBindings {
							actualRB := updatedCP.Status.ResourceStatus.RoleBindings[i]
							if expectedRB.Name != actualRB.Name {
								t.Errorf("expected RoleBinding name %s, got %s", expectedRB.Name, actualRB.Name)
							}
							if expectedRB.Namespace != actualRB.Namespace {
								t.Errorf("expected RoleBinding namespace %s, got %s", expectedRB.Namespace, actualRB.Namespace)
							}
							if len(expectedRB.Conditions) != len(actualRB.Conditions) {
								t.Errorf("expected %d conditions for RoleBinding %s/%s, got %d",
									len(expectedRB.Conditions), expectedRB.Namespace, expectedRB.Name, len(actualRB.Conditions))
							}
							for j, expectedCond := range expectedRB.Conditions {
								actualCond := actualRB.Conditions[j]
								if expectedCond.Type != actualCond.Type {
									t.Errorf("expected condition type %s, got %s", expectedCond.Type, actualCond.Type)
								}
								if expectedCond.Status != actualCond.Status {
									t.Errorf("expected condition status %s, got %s", expectedCond.Status, actualCond.Status)
								}
								if expectedCond.Reason != actualCond.Reason {
									t.Errorf("expected condition reason %s, got %s", expectedCond.Reason, actualCond.Reason)
								}
								if expectedCond.Message != actualCond.Message {
									t.Errorf("expected condition message %s, got %s", expectedCond.Message, actualCond.Message)
								}
							}
						}
					}
				}
			}
		})
	}
}
