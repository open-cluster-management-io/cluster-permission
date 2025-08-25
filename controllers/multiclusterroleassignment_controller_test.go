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

	"k8s.io/client-go/rest"
	mcrav1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestMulticlusterRoleAssignmentReconciler_Reconcile(t *testing.T) {
	reconciler := &MulticlusterRoleAssignmentReconciler{}
	req := ctrl.Request{}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile failed: %v", err)
	}
}

func TestMulticlusterRoleAssignmentReconciler_SetupWithManager(t *testing.T) {
	scheme, err := mcrav1alpha1.SchemeBuilder.Build()
	if err != nil {
		t.Fatalf("Failed to build scheme: %v", err)
	}

	mgr, err := manager.New(&rest.Config{}, manager.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	reconciler := &MulticlusterRoleAssignmentReconciler{}

	err = reconciler.SetupWithManager(mgr)
	if err != nil {
		t.Errorf("SetupWithManager failed: %v", err)
	}
}
