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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

func TestGenerateResourceAppliedCondition(t *testing.T) {
	now := metav1.Now()
	cases := []struct {
		name              string
		conditions        []metav1.Condition
		expectedCondition metav1.Condition
	}{
		{
			name: "Applied condition is true",
			conditions: []metav1.Condition{
				{
					Type:               workv1.ManifestApplied,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: now,
					Message:            "Successfully applied",
				},
			},
			expectedCondition: metav1.Condition{
				Type:               cpv1alpha1.ConditionTypeApplied,
				Status:             metav1.ConditionTrue,
				Reason:             "AppliedManifestComplete",
				Message:            "Apply manifest complete",
				LastTransitionTime: now,
			},
		},
		{
			name: "Applied condition is false",
			conditions: []metav1.Condition{
				{
					Type:               workv1.ManifestApplied,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: now,
					Message:            "Failed to apply: some error",
				},
			},
			expectedCondition: metav1.Condition{
				Type:               cpv1alpha1.ConditionTypeApplied,
				Status:             metav1.ConditionFalse,
				Reason:             "FailedApplyManifest",
				Message:            "Failed to apply: some error",
				LastTransitionTime: now,
			},
		},
		{
			name:       "No Applied condition",
			conditions: []metav1.Condition{},
			expectedCondition: metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeApplied,
				Status:  metav1.ConditionFalse,
				Reason:  "FailedGetManifestCondition",
				Message: "Failed to get the manifest condition",
			},
		},
		{
			name: "Multiple conditions with Applied condition",
			conditions: []metav1.Condition{
				{
					Type:   "Available",
					Status: metav1.ConditionTrue,
				},
				{
					Type:               workv1.ManifestApplied,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
			expectedCondition: metav1.Condition{
				Type:               cpv1alpha1.ConditionTypeApplied,
				Status:             metav1.ConditionTrue,
				Reason:             "AppliedManifestComplete",
				Message:            "Apply manifest complete",
				LastTransitionTime: now,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := generateResourceAppliedCondition(c.conditions)

			if result.Type != c.expectedCondition.Type {
				t.Errorf("expected condition type %s, got %s", c.expectedCondition.Type, result.Type)
			}
			if result.Status != c.expectedCondition.Status {
				t.Errorf("expected condition status %s, got %s", c.expectedCondition.Status, result.Status)
			}
			if result.Reason != c.expectedCondition.Reason {
				t.Errorf("expected condition reason %s, got %s", c.expectedCondition.Reason, result.Reason)
			}
			if result.Message != c.expectedCondition.Message {
				t.Errorf("expected condition message %s, got %s", c.expectedCondition.Message, result.Message)
			}
			// Only check LastTransitionTime if it's set in expected condition
			if !c.expectedCondition.LastTransitionTime.IsZero() && result.LastTransitionTime != c.expectedCondition.LastTransitionTime {
				t.Errorf("expected LastTransitionTime %v, got %v", c.expectedCondition.LastTransitionTime, result.LastTransitionTime)
			}
		})
	}
}

func TestBuildResourceStatus(t *testing.T) {
	cases := []struct {
		name                   string
		manifestWork           *workv1.ManifestWork
		expectedResourceStatus *cpv1alpha1.ResourceStatus
	}{
		{
			name: "empty manifest work",
			manifestWork: &workv1.ManifestWork{
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{},
					},
				},
			},
			expectedResourceStatus: &cpv1alpha1.ResourceStatus{},
		},
		{
			name: "all resource types present",
			manifestWork: &workv1.ManifestWork{
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "cr1",
								},
								Conditions: []metav1.Condition{
									{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRoleBinding",
									Name: "crb1",
								},
								Conditions: []metav1.Condition{
									{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Role",
									Name:      "r1",
									Namespace: "ns1",
								},
								Conditions: []metav1.Condition{
									{Type: workv1.ManifestApplied, Status: metav1.ConditionFalse},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "RoleBinding",
									Name:      "rb1",
									Namespace: "ns1",
								},
								Conditions: []metav1.Condition{
									{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
								},
							},
						},
					},
				},
			},
			expectedResourceStatus: &cpv1alpha1.ResourceStatus{
				ClusterRoles: []cpv1alpha1.ClusterRoleStatus{
					{Name: "cr1"},
				},
				ClusterRoleBindings: []cpv1alpha1.ClusterRoleBindingStatus{
					{Name: "crb1"},
				},
				Roles: []cpv1alpha1.RoleStatus{
					{Name: "r1", Namespace: "ns1"},
				},
				RoleBindings: []cpv1alpha1.RoleBindingStatus{
					{Name: "rb1", Namespace: "ns1"},
				},
			},
		},
		{
			name: "resources with missing applied conditions",
			manifestWork: &workv1.ManifestWork{
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "cr-no-condition",
								},
								Conditions: []metav1.Condition{},
							},
						},
					},
				},
			},
			expectedResourceStatus: &cpv1alpha1.ResourceStatus{
				ClusterRoles: []cpv1alpha1.ClusterRoleStatus{
					{Name: "cr-no-condition"},
				},
			},
		},
		{
			name: "non-RBAC resources are ignored",
			manifestWork: &workv1.ManifestWork{
				Status: workv1.ManifestWorkStatus{
					ResourceStatus: workv1.ManifestResourceStatus{
						Manifests: []workv1.ManifestCondition{
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind:      "Deployment",
									Name:      "my-deployment",
									Namespace: "default",
								},
								Conditions: []metav1.Condition{
									{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
								},
							},
							{
								ResourceMeta: workv1.ManifestResourceMeta{
									Kind: "ClusterRole",
									Name: "cr1",
								},
								Conditions: []metav1.Condition{
									{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
								},
							},
						},
					},
				},
			},
			expectedResourceStatus: &cpv1alpha1.ResourceStatus{
				ClusterRoles: []cpv1alpha1.ClusterRoleStatus{
					{Name: "cr1"},
				},
			},
		},
	}

	var testscheme = scheme.Scheme
	_ = cpv1alpha1.AddToScheme(testscheme)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpsr := &ClusterPermissionStatusReconciler{
				Scheme: testscheme,
			}

			result := cpsr.buildResourceStatus(c.manifestWork)

			if len(c.expectedResourceStatus.ClusterRoles) != len(result.ClusterRoles) {
				t.Errorf("expected %d ClusterRoles, got %d",
					len(c.expectedResourceStatus.ClusterRoles), len(result.ClusterRoles))
			}
			if len(c.expectedResourceStatus.ClusterRoleBindings) != len(result.ClusterRoleBindings) {
				t.Errorf("expected %d ClusterRoleBindings, got %d",
					len(c.expectedResourceStatus.ClusterRoleBindings), len(result.ClusterRoleBindings))
			}
			if len(c.expectedResourceStatus.Roles) != len(result.Roles) {
				t.Errorf("expected %d Roles, got %d",
					len(c.expectedResourceStatus.Roles), len(result.Roles))
			}
			if len(c.expectedResourceStatus.RoleBindings) != len(result.RoleBindings) {
				t.Errorf("expected %d RoleBindings, got %d",
					len(c.expectedResourceStatus.RoleBindings), len(result.RoleBindings))
			}
		})
	}
}

func TestReconcileEdgeCases(t *testing.T) {
	trueVal := true

	cases := []struct {
		name            string
		manifestWork    *workv1.ManifestWork
		clusterPermission *cpv1alpha1.ClusterPermission
		expectError     bool
	}{
		{
			name: "ManifestWork with no owner reference",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
					},
				},
			},
		},
		{
			name: "ManifestWork with non-controller owner reference",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: nil,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
					},
				},
			},
		},
		{
			name: "ClusterPermission being deleted",
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
						{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
					},
				},
			},
			clusterPermission: &cpv1alpha1.ClusterPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cp",
					Namespace:         "cluster1",
					DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
					Finalizers:        []string{"test-finalizer"},
				},
			},
		},
		{
			name: "ManifestWork not found - should not return error",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-mw",
					Namespace: "cluster1",
				},
			},
		},
		{
			name: "ClusterPermission not found - should not return error",
			manifestWork: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "non-existent-cp",
							Controller: &trueVal,
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []metav1.Condition{
						{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
					},
				},
			},
		},
	}

	var testscheme = scheme.Scheme
	_ = cpv1alpha1.AddToScheme(testscheme)
	_ = workv1.Install(testscheme)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var objects []client.Object
			if c.name != "ManifestWork not found - should not return error" {
				objects = append(objects, c.manifestWork)
			}
			if c.clusterPermission != nil {
				objects = append(objects, c.clusterPermission)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(testscheme).
				WithObjects(objects...).
				WithStatusSubresource(objects...).
				Build()

			cpsr := &ClusterPermissionStatusReconciler{
				Client: fakeClient,
				Scheme: testscheme,
			}

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
		})
	}
}

func TestReconcileWithStatusUpdateNoChange(t *testing.T) {
	trueVal := true
	falseVal := false

	var testscheme = scheme.Scheme
	_ = cpv1alpha1.AddToScheme(testscheme)
	_ = workv1.Install(testscheme)

	// Test case where status update is skipped because there's no change
	clusterPermission := &cpv1alpha1.ClusterPermission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "cluster1",
			UID:       "test-uid-12345",
		},
		Spec: cpv1alpha1.ClusterPermissionSpec{
			Validate: &falseVal,
		},
		Status: cpv1alpha1.ClusterPermissionStatus{
			Conditions: []metav1.Condition{
				{
					Type:    cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
					Status:  metav1.ConditionTrue,
					Reason:  "AppliedManifestWorkComplete",
					Message: "Apply manifest work complete\nRun the following command to check the ManifestWork status:\nkubectl -n cluster1 get ManifestWork test-mw -o yaml",
				},
			},
		},
	}

	manifestWork := &workv1.ManifestWork{
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
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testscheme).
		WithObjects(clusterPermission, manifestWork).
		WithStatusSubresource(clusterPermission, manifestWork).
		Build()

	cpsr := &ClusterPermissionStatusReconciler{
		Client: fakeClient,
		Scheme: testscheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      manifestWork.Name,
			Namespace: manifestWork.Namespace,
		},
	}

	_, err := cpsr.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateClusterPermissionStatusWithRetry(t *testing.T) {
	falseVal := false

	var testscheme = scheme.Scheme
	_ = cpv1alpha1.AddToScheme(testscheme)
	_ = workv1.Install(testscheme)

	// Create a ClusterPermission with existing status that will be updated
	clusterPermission := &cpv1alpha1.ClusterPermission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "cluster1",
			UID:       "test-uid-12345",
		},
		Spec: cpv1alpha1.ClusterPermissionSpec{
			Validate: &falseVal,
		},
		Status: cpv1alpha1.ClusterPermissionStatus{
			Conditions: []metav1.Condition{
				{
					Type:    cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
					Status:  metav1.ConditionFalse,
					Reason:  "OldReason",
					Message: "Old message",
				},
			},
		},
	}

	manifestWork := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mw",
			Namespace: "cluster1",
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
							Name: "test-cr",
						},
						Conditions: []metav1.Condition{
							{Type: workv1.ManifestApplied, Status: metav1.ConditionTrue},
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testscheme).
		WithObjects(clusterPermission, manifestWork).
		WithStatusSubresource(clusterPermission, manifestWork).
		Build()

	cpsr := &ClusterPermissionStatusReconciler{
		Client: fakeClient,
		Scheme: testscheme,
	}

	// Test updateClusterPermissionStatus directly
	err := cpsr.updateClusterPermissionStatus(context.Background(), clusterPermission, manifestWork)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify status was updated
	var updatedCP cpv1alpha1.ClusterPermission
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      clusterPermission.Name,
		Namespace: clusterPermission.Namespace,
	}, &updatedCP)
	if err != nil {
		t.Fatalf("failed to get updated ClusterPermission: %v", err)
	}

	appliedCond := meta.FindStatusCondition(updatedCP.Status.Conditions, cpv1alpha1.ConditionTypeAppliedRBACManifestWork)
	if appliedCond == nil {
		t.Errorf("expected AppliedRBACManifestWork condition but it was not found")
	} else if appliedCond.Status != metav1.ConditionTrue {
		t.Errorf("expected condition status True, got %s", appliedCond.Status)
	}

	if updatedCP.Status.ResourceStatus == nil {
		t.Errorf("expected ResourceStatus to be populated but it was nil")
	}
}

func TestFindClusterPermissionForManifestWork(t *testing.T) {
	trueVal := true
	falseVal := false

	cases := []struct {
		name             string
		obj              client.Object
		expectedRequests []reconcile.Request
	}{
		{
			name: "ManifestWork with ClusterPermission controller owner",
			obj: &workv1.ManifestWork{
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
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "test-mw",
						Namespace: "cluster1",
					},
				},
			},
		},
		{
			name: "ManifestWork with non-controller ClusterPermission owner",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &falseVal,
						},
					},
				},
			},
			expectedRequests: nil,
		},
		{
			name: "ManifestWork with nil controller field",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: nil,
						},
					},
				},
			},
			expectedRequests: nil,
		},
		{
			name: "ManifestWork with different APIVersion",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
			},
			expectedRequests: nil,
		},
		{
			name: "ManifestWork with different Kind",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "SomeOtherKind",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
			},
			expectedRequests: nil,
		},
		{
			name: "ManifestWork with no owner references",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
				},
			},
			expectedRequests: nil,
		},
		{
			name: "ManifestWork with multiple owner references, one is ClusterPermission controller",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mw",
					Namespace: "cluster1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "test-cm",
							Controller: &falseVal,
						},
						{
							APIVersion: cpv1alpha1.GroupVersion.String(),
							Kind:       "ClusterPermission",
							Name:       "test-cp",
							Controller: &trueVal,
						},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "test-mw",
						Namespace: "cluster1",
					},
				},
			},
		},
		{
			name:             "Non-ManifestWork object",
			obj:              &cpv1alpha1.ClusterPermission{},
			expectedRequests: nil,
		},
	}

	var testscheme = scheme.Scheme
	_ = cpv1alpha1.AddToScheme(testscheme)
	_ = workv1.Install(testscheme)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpsr := &ClusterPermissionStatusReconciler{
				Scheme: testscheme,
			}

			requests := cpsr.findClusterPermissionForManifestWork(context.Background(), c.obj)

			if len(c.expectedRequests) != len(requests) {
				t.Errorf("expected %d requests, got %d", len(c.expectedRequests), len(requests))
				return
			}

			for i, expectedReq := range c.expectedRequests {
				if requests[i].Name != expectedReq.Name || requests[i].Namespace != expectedReq.Namespace {
					t.Errorf("expected request %v, got %v", expectedReq, requests[i])
				}
			}
		})
	}
}
