package controllers

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ClusterPermissionStatusReconciler reconciles ManifestWork objects and updates
// the status of their owner ClusterPermission
type ClusterPermissionStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterPermissionStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named("clusterpermission-status-controller").
		Watches(
			&workv1.ManifestWork{},
			handler.EnqueueRequestsFromMapFunc(r.findClusterPermissionForManifestWork),
			builder.WithPredicates(predicate.Funcs{
				GenericFunc: func(e event.GenericEvent) bool { return true },
				DeleteFunc:  func(e event.DeleteEvent) bool { return false },
				CreateFunc:  func(e event.CreateEvent) bool { return false },
				UpdateFunc:  func(e event.UpdateEvent) bool { return true },
			}),
		).
		Complete(r)
}

// findClusterPermissionForManifestWork maps a ManifestWork to its owner ClusterPermission
func (r *ClusterPermissionStatusReconciler) findClusterPermissionForManifestWork(ctx context.Context,
	obj client.Object) []reconcile.Request {
	manifestWork, ok := obj.(*workv1.ManifestWork)
	if !ok {
		return nil
	}

	// Check if the ManifestWork has a ClusterPermission owner
	for _, ownerRef := range manifestWork.GetOwnerReferences() {
		if ownerRef.APIVersion == cpv1alpha1.GroupVersion.String() &&
			ownerRef.Kind == "ClusterPermission" &&
			ownerRef.Controller != nil && *ownerRef.Controller {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      manifestWork.Name,
						Namespace: manifestWork.Namespace,
					},
				},
			}
		}
	}

	return nil
}

// +kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch
// +kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks/status,verbs=get
// +kubebuilder:rbac:groups=rbac.open-cluster-management.io,resources=clusterpermissions,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.open-cluster-management.io,resources=clusterpermissions/status,verbs=get;update;patch

// Reconcile watches ManifestWork resources and updates the ClusterPermission status
// based on the manifest statuses
func (r *ClusterPermissionStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling manifestWork status",
		"namespacedName", req.NamespacedName)

	var manifestWork workv1.ManifestWork
	err := r.Get(ctx, req.NamespacedName, &manifestWork)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		logger.Error(err, "unable to fetch ManifestWork",
			"namespacedName", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// Find the owner ClusterPermission
	var clusterPermissionName string
	for _, ownerRef := range manifestWork.GetOwnerReferences() {
		if ownerRef.APIVersion == cpv1alpha1.GroupVersion.String() &&
			ownerRef.Kind == "ClusterPermission" &&
			ownerRef.Controller != nil && *ownerRef.Controller {
			clusterPermissionName = ownerRef.Name
			break
		}
	}

	if clusterPermissionName == "" {
		logger.V(4).Info("ManifestWork does not have a ClusterPermission owner, skipping",
			"namespacedName", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Get the ClusterPermission
	var clusterPermission cpv1alpha1.ClusterPermission
	err = r.Get(ctx, types.NamespacedName{
		Name:      clusterPermissionName,
		Namespace: manifestWork.Namespace,
	}, &clusterPermission)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		logger.Error(err, "unable to fetch ClusterPermission",
			"name", clusterPermissionName, "manifestWork", manifestWork.Name,
			"namespace", manifestWork.Namespace)
		return ctrl.Result{}, err
	}

	if !clusterPermission.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("update clusterPermission status",
		"name", clusterPermission.Name, "namespace", clusterPermission.Namespace)

	// Update ClusterPermission status based on ManifestWork status
	if err := r.updateClusterPermissionStatus(ctx, &clusterPermission, &manifestWork); err != nil {
		logger.Error(err, "failed to update ClusterPermission status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateClusterPermissionStatus updates the ClusterPermission status based on ManifestWork manifest statuses
func (r *ClusterPermissionStatusReconciler) updateClusterPermissionStatus(
	ctx context.Context,
	clusterPermission *cpv1alpha1.ClusterPermission,
	manifestWork *workv1.ManifestWork) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Refresh the ClusterPermission to get the latest version
		var latest cpv1alpha1.ClusterPermission
		if err := r.Get(ctx, types.NamespacedName{
			Name:      clusterPermission.Name,
			Namespace: clusterPermission.Namespace,
		}, &latest); err != nil {
			return err
		}

		newStatus := latest.Status.DeepCopy()

		appliedCondition := meta.FindStatusCondition(manifestWork.Status.Conditions, workv1.ManifestApplied)
		if appliedCondition == nil {
			meta.SetStatusCondition(&newStatus.Conditions, metav1.Condition{
				Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
				Status: metav1.ConditionFalse,
				Reason: "ApplyRBACManifestWorkNotComplete",
				Message: "Apply RBAC manifestWork not complete\n" +
					"Run the following command to check the ManifestWork status:\n" +
					"kubectl -n " + clusterPermission.Namespace + " get ManifestWork " + manifestWork.Name + " -o yaml",
			})
			if equality.Semantic.DeepEqual(latest.Status, newStatus) {
				return nil
			}

			latest.Status = *newStatus
			return r.Client.Status().Update(ctx, &latest, &client.SubResourceUpdateOptions{})
		}

		meta.SetStatusCondition(&newStatus.Conditions, metav1.Condition{
			Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
			Status: appliedCondition.Status,
			Reason: appliedCondition.Reason,
			Message: appliedCondition.Message + "\n" +
				"Run the following command to check the ManifestWork status:\n" +
				"kubectl -n " + clusterPermission.Namespace + " get ManifestWork " + manifestWork.Name + " -o yaml",
		})

		if latest.Spec.Validate != nil && *latest.Spec.Validate {
			if err := r.updateValidateConditions(&newStatus.Conditions, manifestWork); err != nil {
				return err
			}
		}

		// Initialize ResourceStatus if it doesn't exist
		if newStatus.ResourceStatus == nil {
			newStatus.ResourceStatus = &cpv1alpha1.ResourceStatus{}
		}
		r.updateResourceStatus(newStatus.ResourceStatus, manifestWork)

		if equality.Semantic.DeepEqual(latest.Status, newStatus) {
			return nil
		}

		latest.Status = *newStatus
		return r.Client.Status().Update(ctx, &latest, &client.SubResourceUpdateOptions{})
	})
}

// updateValidateConditions processes the feedback from validation ManifestWork and updates status conditions
func (r *ClusterPermissionStatusReconciler) updateValidateConditions(conditions *[]metav1.Condition,
	mw *workv1.ManifestWork) error {
	// Analyze the status feedback to determine which roles are missing
	var missingRoles []string
	var missingClusterRoles []string

	// Check feedback from ManifestWork status
	for _, status := range mw.Status.ResourceStatus.Manifests {
		availableCondition := meta.FindStatusCondition(status.Conditions, "Available")
		if availableCondition != nil && availableCondition.Status == "True" {
			continue
		}

		switch status.ResourceMeta.Kind {
		case "Role":
			missingRoles = append(missingRoles, status.ResourceMeta.Namespace+"/"+status.ResourceMeta.Name)
		case "ClusterRole":
			missingClusterRoles = append(missingClusterRoles, status.ResourceMeta.Name)
		}
	}

	// Update status conditions based on validation results
	if len(missingRoles) > 0 {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
			Status:  metav1.ConditionFalse,
			Reason:  "RolesNotFound",
			Message: "The following roles were not found: " + joinStrings(missingRoles, ", "),
		})
	} else {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
			Status:  metav1.ConditionTrue,
			Reason:  "AllRolesFound",
			Message: "All referenced roles were found",
		})

	}

	if len(missingClusterRoles) > 0 {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
			Status:  metav1.ConditionFalse,
			Reason:  "ClusterRolesNotFound",
			Message: "The following cluster roles were not found: " + joinStrings(missingClusterRoles, ", "),
		})
	} else {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
			Status:  metav1.ConditionTrue,
			Reason:  "AllClusterRolesFound",
			Message: "All referenced cluster roles were found",
		})
	}

	return nil
}

func (r *ClusterPermissionStatusReconciler) updateResourceStatus(resourceStatus *cpv1alpha1.ResourceStatus,
	manifestWork *workv1.ManifestWork) {
	// Process each manifest status
	for _, manifestStatus := range manifestWork.Status.ResourceStatus.Manifests {
		switch manifestStatus.ResourceMeta.Kind {
		case "ClusterRole":
			r.updateClusterRoleStatus(resourceStatus, manifestStatus)
		case "ClusterRoleBinding":
			r.updateClusterRoleBindingStatus(resourceStatus, manifestStatus)
		case "Role":
			r.updateRoleStatus(resourceStatus, manifestStatus)
		case "RoleBinding":
			r.updateRoleBindingStatus(resourceStatus, manifestStatus)
		}
	}
}

// updateClusterRoleStatus updates or adds the status for a ClusterRole
func (r *ClusterPermissionStatusReconciler) updateClusterRoleStatus(
	resourceStatus *cpv1alpha1.ResourceStatus,
	manifestStatus workv1.ManifestCondition) {
	name := manifestStatus.ResourceMeta.Name
	appliedCondition := generateResourceAppliedCondition(manifestStatus.Conditions)

	// Find existing status or create new one
	found := false
	for i := range resourceStatus.ClusterRoles {
		if resourceStatus.ClusterRoles[i].Name == name {
			meta.SetStatusCondition(&resourceStatus.ClusterRoles[i].Conditions, appliedCondition)
			found = true
			break
		}
	}

	if !found {
		resourceStatus.ClusterRoles = append(resourceStatus.ClusterRoles, cpv1alpha1.ClusterRoleStatus{
			Name:       name,
			Conditions: []metav1.Condition{appliedCondition},
		})
	}
}

// updateClusterRoleBindingStatus updates or adds the status for a ClusterRoleBinding
func (r *ClusterPermissionStatusReconciler) updateClusterRoleBindingStatus(
	resourceStatus *cpv1alpha1.ResourceStatus,
	manifestStatus workv1.ManifestCondition) {
	name := manifestStatus.ResourceMeta.Name
	appliedCondition := generateResourceAppliedCondition(manifestStatus.Conditions)

	// Find existing status or create new one
	found := false
	for i := range resourceStatus.ClusterRoleBindings {
		if resourceStatus.ClusterRoleBindings[i].Name == name {
			meta.SetStatusCondition(&resourceStatus.ClusterRoleBindings[i].Conditions, appliedCondition)
			found = true
			break
		}
	}

	if !found {
		resourceStatus.ClusterRoleBindings = append(resourceStatus.ClusterRoleBindings, cpv1alpha1.ClusterRoleBindingStatus{
			Name:       name,
			Conditions: []metav1.Condition{appliedCondition},
		})
	}
}

// updateRoleStatus updates or adds the status for a Role
func (r *ClusterPermissionStatusReconciler) updateRoleStatus(
	resourceStatus *cpv1alpha1.ResourceStatus,
	manifestStatus workv1.ManifestCondition) {
	name := manifestStatus.ResourceMeta.Name
	namespace := manifestStatus.ResourceMeta.Namespace
	appliedCondition := generateResourceAppliedCondition(manifestStatus.Conditions)

	// Find existing status or create new one
	found := false
	for i := range resourceStatus.Roles {
		if resourceStatus.Roles[i].Name == name && resourceStatus.Roles[i].Namespace == namespace {
			meta.SetStatusCondition(&resourceStatus.Roles[i].Conditions, appliedCondition)
			found = true
			break
		}
	}

	if !found {
		resourceStatus.Roles = append(resourceStatus.Roles, cpv1alpha1.RoleStatus{
			Name:       name,
			Namespace:  namespace,
			Conditions: []metav1.Condition{appliedCondition},
		})
	}
}

// updateRoleBindingStatus updates or adds the status for a RoleBinding
func (r *ClusterPermissionStatusReconciler) updateRoleBindingStatus(
	resourceStatus *cpv1alpha1.ResourceStatus,
	manifestStatus workv1.ManifestCondition) {
	name := manifestStatus.ResourceMeta.Name
	namespace := manifestStatus.ResourceMeta.Namespace
	appliedCondition := generateResourceAppliedCondition(manifestStatus.Conditions)

	// Find existing status or create new one
	found := false
	for i := range resourceStatus.RoleBindings {
		if resourceStatus.RoleBindings[i].Name == name && resourceStatus.RoleBindings[i].Namespace == namespace {
			meta.SetStatusCondition(&resourceStatus.RoleBindings[i].Conditions, appliedCondition)
			found = true
			break
		}
	}

	if !found {
		resourceStatus.RoleBindings = append(resourceStatus.RoleBindings, cpv1alpha1.RoleBindingStatus{
			Name:       name,
			Namespace:  namespace,
			Conditions: []metav1.Condition{appliedCondition},
		})
	}
}

// generateResourceAppliedCondition generates Applied condition for resource
func generateResourceAppliedCondition(conditions []metav1.Condition) metav1.Condition {
	appliedCondition := meta.FindStatusCondition(conditions, workv1.ManifestApplied)
	if appliedCondition == nil {
		return metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeApplied,
			Status:  metav1.ConditionFalse,
			Reason:  "FailedGetManifestCondition",
			Message: "Failed to get the manifest condition",
		}
	}

	newCondition := metav1.Condition{
		Type:               cpv1alpha1.ConditionTypeApplied,
		Status:             appliedCondition.Status,
		LastTransitionTime: appliedCondition.LastTransitionTime,
	}

	if newCondition.Status == metav1.ConditionTrue {
		newCondition.Reason = "AppliedManifestComplete"
		newCondition.Message = "Apply manifest complete"
	} else {
		newCondition.Reason = "FailedApplyManifest"
		newCondition.Message = appliedCondition.Message
	}

	return newCondition
}
