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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/log"

	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	msacommon "open-cluster-management.io/managed-serviceaccount/pkg/common"
)

// ManagedClusterAddOnInformer is a custom informer that watches ManagedClusterAddOn resources
// with metadata.name = "managed-serviceaccount"
type ManagedClusterAddOnInformer struct {
	// client is the Kubernetes client
	client *rest.RESTClient
	// stopCh is used to signal the informer to stop
	stopCh chan struct{}
	// workqueue is used to queue events for processing
	workqueue workqueue.RateLimitingInterface
	// informer is the Kubernetes informer
	informer cache.SharedIndexInformer
	// eventHandler is the function to call when events occur
	eventHandler func(obj *addonv1alpha1.ManagedClusterAddOn)
	// logger for this informer
	logger logr.Logger
}

// NewManagedClusterAddOnInformer creates a new custom informer for ManagedClusterAddOn resources
// that only selects resources with metadata.name = "managed-serviceaccount"
func NewManagedClusterAddOnInformer(config *rest.Config, eventHandler func(obj *addonv1alpha1.ManagedClusterAddOn)) (*ManagedClusterAddOnInformer, error) {
	// Create a scheme and register the addon types
	addonScheme := runtime.NewScheme()
	if err := addonv1alpha1.AddToScheme(addonScheme); err != nil {
		return nil, fmt.Errorf("failed to add addon types to scheme: %w", err)
	}

	// Create a REST client for the addon API
	config = rest.CopyConfig(config)
	config.GroupVersion = &addonv1alpha1.GroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.NewCodecFactory(addonScheme).WithoutConversion()

	client, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	// Create a field selector to only watch resources with name "managed-serviceaccount"
	fieldSelector := fields.OneTermEqualSelector("metadata.name", msacommon.AddonName)

	// Create a list watcher with the field selector
	listWatcher := cache.NewListWatchFromClient(
		client,
		"managedclusteraddons",
		metav1.NamespaceAll,
		fieldSelector,
	)

	// Create the informer
	informer := cache.NewSharedIndexInformer(
		listWatcher,
		&addonv1alpha1.ManagedClusterAddOn{},
		time.Hour*10, // resync period - ManagedClusterAddOn namespaces change infrequently
		cache.Indexers{},
	)

	// Create workqueue with rate limiting
	workqueue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		"ManagedClusterAddOnInformer",
	)

	return &ManagedClusterAddOnInformer{
		client:       client,
		stopCh:       make(chan struct{}),
		workqueue:    workqueue,
		informer:     informer,
		eventHandler: eventHandler,
		logger:       log.Log.WithName("ManagedClusterAddOnInformer"),
	}, nil
}

// Start starts the informer
func (i *ManagedClusterAddOnInformer) Start() error {
	i.logger.Info("Starting ManagedClusterAddOnInformer")

	// Add event handlers
	_, err := i.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			// no-op
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldAddon, oldOk := oldObj.(*addonv1alpha1.ManagedClusterAddOn)
			newAddon, newOk := newObj.(*addonv1alpha1.ManagedClusterAddOn)

			if !newOk {
				i.logger.Error(nil, "newObj is not a ManagedClusterAddOn", "object", newObj)
				return
			}

			if !oldOk {
				i.logger.Error(nil, "oldObj is not a ManagedClusterAddOn", "object", oldObj)
				return
			}

			// Only enqueue if the addon's Status.Namespace or Spec.InstallNamespace has changed
			// These are the fields the controller cares about for generating subjects
			if oldAddon.Status.Namespace != newAddon.Status.Namespace ||
				oldAddon.Spec.InstallNamespace != newAddon.Spec.InstallNamespace {
				i.logger.Info("ManagedClusterAddOn namespace changed, enqueuing for reconciliation",
					"namespace", newAddon.Namespace,
					"name", newAddon.Name,
					"oldStatusNamespace", oldAddon.Status.Namespace,
					"newStatusNamespace", newAddon.Status.Namespace,
					"oldInstallNamespace", oldAddon.Spec.InstallNamespace,
					"newInstallNamespace", newAddon.Spec.InstallNamespace)
				i.enqueueAddon(newAddon)
			}
		},
		DeleteFunc: func(obj any) {
			// no-op
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	// Start the informer
	go i.informer.Run(i.stopCh)

	// Wait for cache to sync
	if !cache.WaitForCacheSync(i.stopCh, i.informer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// Start workers to process the queue
	go i.runWorker()

	return nil
}

// Stop stops the informer
func (i *ManagedClusterAddOnInformer) Stop() {
	i.logger.Info("Stopping ManagedClusterAddOnInformer")
	close(i.stopCh)
	i.workqueue.ShutDown()
}

// enqueueAddon adds a ManagedClusterAddOn to the work queue
func (i *ManagedClusterAddOnInformer) enqueueAddon(addon *addonv1alpha1.ManagedClusterAddOn) {
	key := fmt.Sprintf("%s/%s", addon.Namespace, addon.Name)
	i.workqueue.Add(key)
}

// runWorker processes items from the work queue
func (i *ManagedClusterAddOnInformer) runWorker() {
	for i.processNextWorkItem() {
	}
}

// processNextWorkItem processes the next item in the work queue
func (i *ManagedClusterAddOnInformer) processNextWorkItem() bool {
	obj, shutdown := i.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj any) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer i.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date than when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			i.workqueue.Forget(obj)
			i.logger.Error(nil, "expected string in workqueue but got %#v", obj)
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// resource to be synced.
		if err := i.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			i.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		i.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		i.logger.Error(err, "error processing work item")
		return true
	}

	return true
}

// syncHandler processes a single item from the work queue
func (i *ManagedClusterAddOnInformer) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		i.logger.Error(err, "invalid resource key", "key", key)
		return nil
	}

	// Get the ManagedClusterAddOn from the informer cache
	obj, exists, err := i.informer.GetIndexer().GetByKey(key)
	if err != nil {
		i.logger.Error(err, "failed to get ManagedClusterAddOn from cache", "key", key)
		return err
	}

	if !exists {
		// The object no longer exists, so we can stop processing
		i.logger.Info("ManagedClusterAddOn no longer exists", "key", key)
		return nil
	}

	addon, ok := obj.(*addonv1alpha1.ManagedClusterAddOn)
	if !ok {
		i.logger.Error(nil, "object is not a ManagedClusterAddOn", "key", key, "object", obj)
		return nil
	}

	// Call the event handler
	if i.eventHandler != nil {
		i.eventHandler(addon)
	}

	return nil
}

// GetManagedClusterAddOn retrieves a ManagedClusterAddOn by namespace and name
func (i *ManagedClusterAddOnInformer) GetManagedClusterAddOn(namespace, name string) (*addonv1alpha1.ManagedClusterAddOn, error) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	obj, exists, err := i.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(addonv1alpha1.Resource("managedclusteraddons"), name)
	}

	addon, ok := obj.(*addonv1alpha1.ManagedClusterAddOn)
	if !ok {
		return nil, fmt.Errorf("object is not a ManagedClusterAddOn")
	}

	return addon, nil
}

// ListManagedClusterAddOns lists all ManagedClusterAddOns in the cache
func (i *ManagedClusterAddOnInformer) ListManagedClusterAddOns() ([]*addonv1alpha1.ManagedClusterAddOn, error) {
	items := i.informer.GetIndexer().List()
	addons := make([]*addonv1alpha1.ManagedClusterAddOn, 0, len(items))

	for _, item := range items {
		if addon, ok := item.(*addonv1alpha1.ManagedClusterAddOn); ok {
			addons = append(addons, addon)
		} else {
			i.logger.Error(nil, "object is not a ManagedClusterAddOn", "object", item)
		}
	}

	return addons, nil
}

// HasSynced returns true if the informer has synced
func (i *ManagedClusterAddOnInformer) HasSynced() bool {
	return i.informer.HasSynced()
}
