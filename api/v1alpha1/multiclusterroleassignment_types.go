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

package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MulticlusterRoleAssignmentSpec defines the desired state of MulticlusterRoleAssignment.
type MulticlusterRoleAssignmentSpec struct {
	// Subject defines the user, group, or service account for all role assignments.
	// +kubebuilder:validation:Required
	Subject rbacv1.Subject `json:"subject"`

	// RoleAssignments defines the list of role assignments for different roles, namespaces, and cluster sets.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	RoleAssignments []RoleAssignment `json:"roleAssignments"`
}

// RoleAssignment defines a cluster role assignment to specific namespaces and cluster sets.
type RoleAssignment struct {
	// ClusterRole defines the cluster role name to be assigned.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ClusterRole string `json:"clusterRole"`

	// TargetNamespaces defines what namespaces the role should be applied in.
	// If TargetNamespaces is not present, the role will be applied to all cluster namespaces.
	// +kubebuilder:validation:Optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// ClusterSets defines the cluster sets where the role should be applied.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	ClusterSets []string `json:"clusterSets"`
}

// MulticlusterRoleAssignmentStatus defines the observed state of MulticlusterRoleAssignment.
type MulticlusterRoleAssignmentStatus struct {
	// Conditions is the condition list.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MulticlusterRoleAssignment is the Schema for the multiclusterroleassignments API.
type MulticlusterRoleAssignment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MulticlusterRoleAssignmentSpec   `json:"spec,omitempty"`
	Status MulticlusterRoleAssignmentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MulticlusterRoleAssignmentList contains a list of MulticlusterRoleAssignment.
type MulticlusterRoleAssignmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MulticlusterRoleAssignment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MulticlusterRoleAssignment{}, &MulticlusterRoleAssignmentList{})
}
