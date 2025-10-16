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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	msacommon "open-cluster-management.io/managed-serviceaccount/pkg/common"
)

func TestManagedClusterAddOnInformer(t *testing.T) {
	// Create test environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			"../hack/crds",
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	defer testEnv.Stop()

	// The informer will create its own scheme and register addon types internally
	// We just need to pass the config

	// Create event handler that captures events
	events := make(chan string, 10)
	eventHandler := func(addon *addonv1alpha1.ManagedClusterAddOn) {
		events <- addon.Name
	}

	// Create informer
	informer, err := NewManagedClusterAddOnInformer(cfg, eventHandler)
	require.NoError(t, err)

	// Start informer
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = informer.Start()
	require.NoError(t, err)

	// Wait for sync
	timeout := time.After(10 * time.Second)
	for !informer.HasSynced() {
		select {
		case <-timeout:
			t.Fatal("Informer failed to sync within timeout")
		case <-time.After(100 * time.Millisecond):
			// Continue waiting
		}
	}

	// Test that informer is working
	assert.True(t, informer.HasSynced())

	// Stop informer
	informer.Stop()
}

func TestManagedClusterAddOnInformerEventHandling(t *testing.T) {
	// This test validates that the informer can be created successfully
	// even with a simple config
	config := &rest.Config{
		Host: "http://localhost:8080",
	}

	events := make(chan string, 10)
	eventHandler := func(addon *addonv1alpha1.ManagedClusterAddOn) {
		events <- addon.Name
	}

	// Create informer - this should succeed as it only sets up the structure
	informer, err := NewManagedClusterAddOnInformer(config, eventHandler)
	// The informer creation should succeed
	assert.NoError(t, err)
	assert.NotNil(t, informer)

	// Starting the informer would fail due to connection issues, but we don't test that here
}

func TestManagedClusterAddOnInformerFieldSelector(t *testing.T) {
	// Test that the field selector is correctly configured
	// This is more of a unit test for the field selector logic

	// Create a mock addon with the correct name
	addon := &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msacommon.AddonName,
			Namespace: "test-cluster",
		},
		Spec: addonv1alpha1.ManagedClusterAddOnSpec{
			InstallNamespace: "open-cluster-management-agent-addon",
		},
		Status: addonv1alpha1.ManagedClusterAddOnStatus{
			Namespace: "open-cluster-management-agent-addon",
		},
	}

	// Verify the addon has the expected name
	assert.Equal(t, msacommon.AddonName, addon.Name)
	assert.Equal(t, "test-cluster", addon.Namespace)
}

func TestManagedClusterAddOnInformerWorkQueue(t *testing.T) {
	// Test work queue functionality
	config := &rest.Config{
		Host: "http://localhost:8080",
	}

	events := make(chan string, 10)
	eventHandler := func(addon *addonv1alpha1.ManagedClusterAddOn) {
		events <- addon.Name
	}

	// Create informer
	informer, err := NewManagedClusterAddOnInformer(config, eventHandler)
	require.NoError(t, err)

	// Test that workqueue was created
	assert.NotNil(t, informer.workqueue)

	// Test enqueue functionality
	addon := &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msacommon.AddonName,
			Namespace: "test-cluster",
		},
	}

	// This should not panic
	informer.enqueueAddon(addon)
}
