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
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	cpv1alpha1 "open-cluster-management.io/cluster-permission/api/v1alpha1"
	msav1beta1 "open-cluster-management.io/managed-serviceaccount/apis/authentication/v1beta1"
	msacommon "open-cluster-management.io/managed-serviceaccount/pkg/common"
)

const VALIDATION_MW_RETRY_COUNT = 10
const VALIDATION_MW_RETRY_INTERVAL = 5 * time.Second

// ClusterPermissionReconciler reconciles a ClusterPermission object
type ClusterPermissionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterPermissionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cpv1alpha1.ClusterPermission{}).
		Owns(&workv1.ManifestWork{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=rbac.open-cluster-management.io,resources=clusterpermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.open-cluster-management.io,resources=clusterpermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rbac.open-cluster-management.io,resources=clusterpermissions/finalizers,verbs=update
//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=get;list;watch
//+kubebuilder:rbac:groups=authentication.open-cluster-management.io,resources=managedserviceaccounts,verbs=get;list;watch

// Reconcile validates the ClusterPermission spec and applies a ManifestWork with the RBAC resources in it's payload
func (r *ClusterPermissionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling ClusterPermission...")
	defer log.Info("done reconciling ClusterPermission")

	var clusterPermission cpv1alpha1.ClusterPermission
	if err := r.Get(ctx, req.NamespacedName, &clusterPermission); err != nil {
		log.Error(err, "unable to fetch ClusterPermission")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !clusterPermission.DeletionTimestamp.IsZero() {
		log.Info("deleting ClusterPermission, all associated ManifestWorks will be garbage collected")
		return ctrl.Result{}, nil
	}

	log.Info("validating ClusterPermission")

	/* Validations */
	if clusterPermission.Spec.ClusterRoleBinding == nil &&
		(clusterPermission.Spec.ClusterRoleBindings == nil || len(*clusterPermission.Spec.ClusterRoleBindings) == 0) &&
		(clusterPermission.Spec.RoleBindings == nil || len(*clusterPermission.Spec.RoleBindings) == 0) {
		log.Info("no bindings defined for ClusterPermission")

		err := r.updateStatus(ctx, &clusterPermission, &metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidation,
			Status:  metav1.ConditionFalse,
			Reason:  "FailedValidationNoBindingsDefined",
			Message: "no bindings defined",
		})

		return ctrl.Result{}, err
	}

	// verify the ClusterPermission namespace is in a ManagedCluster namespace
	var managedCluster clusterv1.ManagedCluster
	if err := r.Get(ctx, types.NamespacedName{Name: clusterPermission.Namespace}, &managedCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("not found ManagedCluster")

			err := r.updateStatus(ctx, &clusterPermission, &metav1.Condition{
				Type:    cpv1alpha1.ConditionTypeValidation,
				Status:  metav1.ConditionFalse,
				Reason:  "FailedValidationNotInManagedClusterNamespace",
				Message: "namespace value is not a managed cluster",
			})

			return ctrl.Result{}, err
		}

		log.Error(err, "unable to fetch ManagedCluster")
		return ctrl.Result{}, err
	}

	// Handle validation if enabled
	if clusterPermission.Spec.Validate != nil && *clusterPermission.Spec.Validate {
		log.Info("validation enabled, processing role validation")
		if err := r.handleValidation(ctx, &clusterPermission); err != nil {
			log.Error(err, "failed to handle validation")
			return ctrl.Result{}, err
		}
	}

	log.Info("preparing ManifestWork payload")

	clusterRole, clusterRoleBindings, roles, roleBindings, err := r.generateManifestWorkPayload(
		ctx, &clusterPermission)
	if err != nil {
		log.Error(err, "failed to generate payload")

		errStatus := r.updateStatus(ctx, &clusterPermission, &metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
			Status:  metav1.ConditionFalse,
			Reason:  "FailedBuildManifestWork",
			Message: err.Error(),
		})

		return ctrl.Result{}, errStatus
	}

	mwName := generateManifestWorkName(clusterPermission)
	manifestWork := buildManifestWork(clusterPermission, mwName,
		clusterRole, clusterRoleBindings, roles, roleBindings)

	var mw workv1.ManifestWork
	err = r.Get(ctx, types.NamespacedName{Name: mwName, Namespace: clusterPermission.Namespace}, &mw)
	if apierrors.IsNotFound(err) {
		log.Info("creating ManifestWork")
		err = r.Client.Create(ctx, manifestWork)
		if err != nil {
			log.Error(err, "unable to create ManifestWork")
			return ctrl.Result{}, err
		}
	} else if err == nil {
		log.Info("updating ManifestWork")
		mw.Spec = manifestWork.Spec
		err = r.Client.Update(ctx, &mw)
		if err != nil {
			log.Error(err, "unable to update ManifestWork")
			return ctrl.Result{}, err
		}
	} else {
		log.Error(err, "unable to fetch ManifestWork")
		return ctrl.Result{}, err
	}

	err = r.updateStatus(ctx, &clusterPermission, &metav1.Condition{
		Type:   cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
		Status: metav1.ConditionTrue,
		Reason: cpv1alpha1.ConditionTypeAppliedRBACManifestWork,
		Message: "Run the following command to check the ManifestWork status:\n" +
			"kubectl -n " + clusterPermission.Namespace + " get ManifestWork " + mwName + " -o yaml",
	})

	return ctrl.Result{}, err
}

// updateStatus will update the status of the ClusterPermission if there are changes to the status
// after applying the given condition. It will also retry on conflict error.
func (r *ClusterPermissionReconciler) updateStatus(ctx context.Context,
	clusterPermission *cpv1alpha1.ClusterPermission, cond *metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		newStatus := clusterPermission.Status.DeepCopy()
		meta.SetStatusCondition(&newStatus.Conditions, *cond)
		if equality.Semantic.DeepEqual(clusterPermission.Status, newStatus) {
			return nil
		}
		clusterPermission.Status = *newStatus
		return r.Client.Status().Update(ctx, clusterPermission, &client.SubResourceUpdateOptions{})
	})
}

// validateSubject checks if the subject is a ManagedServiceAccount
// if it's a ManagedServiceAccount then verify that the CR exists
func (r *ClusterPermissionReconciler) validateSubject(ctx context.Context, subjects []rbacv1.Subject,
	clusterNamespace string) error {
	var msa msav1beta1.ManagedServiceAccount
	if len(subjects) > 0 {
		for _, sub := range subjects {
			if sub.APIGroup == msav1beta1.GroupVersion.Group && sub.Kind == "ManagedServiceAccount" {
				err := r.Get(ctx, types.NamespacedName{
					Namespace: clusterNamespace,
					Name:      sub.Name,
				}, &msa)

				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func getSubjects(subject rbacv1.Subject, subjects []rbacv1.Subject) []rbacv1.Subject {
	if len(subjects) > 0 {
		return subjects
	} else {
		// should be safe since one of them has to exist due to CRD validation
		return []rbacv1.Subject{subject}
	}
}

// generateSubjects checks if the subjects in the subjects array is a ManagedServiceAccount
// if it is, then append the subjects that represent the ManagedCluster ServiceAccount to the array and return the array
// otherwise, append the same subjects as before and return the array
func (r *ClusterPermissionReconciler) generateSubjects(ctx context.Context,
	subjects []rbacv1.Subject, clusterNamespace string) ([]rbacv1.Subject, error) {
	saSubjects := []rbacv1.Subject{}

	addonNs := ""

	for _, sub := range subjects {
		if sub.APIGroup == msav1beta1.GroupVersion.Group && sub.Kind == "ManagedServiceAccount" {
			if addonNs == "" {
				var addon addonv1alpha1.ManagedClusterAddOn
				if err := r.Get(ctx, types.NamespacedName{Namespace: clusterNamespace, Name: msacommon.AddonName}, &addon); err != nil {
					return []rbacv1.Subject{}, err
				}

				addonNs = addon.Status.Namespace
				if addonNs == "" {
					addonNs = addon.Spec.InstallNamespace
				}
			}

			saSubjects = append(saSubjects, rbacv1.Subject{
				APIGroup:  corev1.GroupName,
				Kind:      "ServiceAccount",
				Namespace: addonNs,
				Name:      sub.Name,
			})
		} else {
			saSubjects = append(saSubjects, sub)
		}
	}

	return saSubjects, nil
}

// generateManifestWorkPayload creates the payload for the ManifestWork based on the ClusterPermission spec
func (r *ClusterPermissionReconciler) generateManifestWorkPayload(ctx context.Context, clusterPermission *cpv1alpha1.ClusterPermission) (
	*rbacv1.ClusterRole, []rbacv1.ClusterRoleBinding, []rbacv1.Role, []rbacv1.RoleBinding, error) {
	var clusterRole *rbacv1.ClusterRole
	var clusterRoleBindings []rbacv1.ClusterRoleBinding
	var roles []rbacv1.Role
	var roleBindings []rbacv1.RoleBinding

	// ClusterRole payload
	if clusterPermission.Spec.ClusterRole != nil {
		clusterRole = &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterPermission.Name,
			},
			Rules: clusterPermission.Spec.ClusterRole.Rules,
		}
	}

	// Only process ClusterRoleBinding if ClusterRoleBindings is nil or empty
	if (clusterPermission.Spec.ClusterRoleBindings == nil || len(*clusterPermission.Spec.ClusterRoleBindings) == 0) &&
		clusterPermission.Spec.ClusterRoleBinding != nil {
		crbSubjects := getSubjects(clusterPermission.Spec.ClusterRoleBinding.Subject,
			clusterPermission.Spec.ClusterRoleBinding.Subjects)
		if err := r.validateSubject(ctx, crbSubjects, clusterPermission.Namespace); err != nil {
			return nil, nil, nil, nil, err
		}

		subjects, err := r.generateSubjects(ctx, crbSubjects, clusterPermission.Namespace)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		// default to ClusterPermission name unless using custom name
		clusterRoleBindingName := clusterPermission.Name
		if clusterPermission.Spec.ClusterRoleBinding.Name != "" {
			clusterRoleBindingName = clusterPermission.Spec.ClusterRoleBinding.Name
		}

		// default to creating a ClusterRole unless using existing ClusterRole
		clusterRoleBindingRoleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterPermission.Name,
		}
		if clusterPermission.Spec.ClusterRoleBinding.RoleRef != nil {
			clusterRoleBindingRoleRef = *clusterPermission.Spec.ClusterRoleBinding.RoleRef
		}

		clusterRoleBindings = append(clusterRoleBindings, rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
			},
			RoleRef:  clusterRoleBindingRoleRef,
			Subjects: subjects,
		})
	}

	// ClusterRoleBindings payload (plural)
	if clusterPermission.Spec.ClusterRoleBindings != nil && len(*clusterPermission.Spec.ClusterRoleBindings) > 0 {
		for _, clusterRoleBinding := range *clusterPermission.Spec.ClusterRoleBindings {
			crbSubjects := getSubjects(clusterRoleBinding.Subject, clusterRoleBinding.Subjects)
			if err := r.validateSubject(ctx, crbSubjects, clusterPermission.Namespace); err != nil {
				return nil, nil, nil, nil, err
			}

			subjects, err := r.generateSubjects(ctx, crbSubjects, clusterPermission.Namespace)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			// default to ClusterPermission name unless using custom name
			clusterRoleBindingName := clusterPermission.Name
			if clusterRoleBinding.Name != "" {
				clusterRoleBindingName = clusterRoleBinding.Name
			}

			// default to creating a ClusterRole unless using existing ClusterRole
			clusterRoleBindingRoleRef := rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterPermission.Name,
			}
			if clusterRoleBinding.RoleRef != nil {
				clusterRoleBindingRoleRef = *clusterRoleBinding.RoleRef
			}

			clusterRoleBindings = append(clusterRoleBindings, rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: rbacv1.SchemeGroupVersion.String(),
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleBindingName,
				},
				RoleRef:  clusterRoleBindingRoleRef,
				Subjects: subjects,
			})
		}
	}

	// Roles payload
	if clusterPermission.Spec.Roles != nil && len(*clusterPermission.Spec.Roles) > 0 {
		for _, role := range *clusterPermission.Spec.Roles {
			if role.Namespace == "" && role.NamespaceSelector == nil {
				return nil, nil, nil, nil,
					errors.New("both Role Namespace and NamespaceSelector cannot be nil and empty")
			}
			if role.Namespace != "" && role.NamespaceSelector != nil {
				return nil, nil, nil, nil,
					errors.New("both Role Namespace and NamespaceSelector cannot populated at the same time")
			}

			if role.NamespaceSelector != nil {
				labelSelector, err := metav1.LabelSelectorAsSelector(role.NamespaceSelector)
				if err != nil {
					return nil, nil, nil, nil, err
				}

				nsList := &corev1.NamespaceList{}
				if err = r.Client.List(ctx, nsList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
					return nil, nil, nil, nil, err
				}

				if nsList == nil || nsList.Items == nil && len(nsList.Items) == 0 {
					return nil, nil, nil, nil,
						errors.New("unable to find any Namespace using NamespaceSelector")
				}

				for _, ns := range nsList.Items {
					roles = append(roles, rbacv1.Role{
						TypeMeta: metav1.TypeMeta{
							APIVersion: rbacv1.SchemeGroupVersion.String(),
							Kind:       "Role",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      clusterPermission.Name,
							Namespace: ns.Name,
						},
						Rules: role.Rules,
					})
				}
			} else if role.Namespace != "" {
				roles = append(roles, rbacv1.Role{
					TypeMeta: metav1.TypeMeta{
						APIVersion: rbacv1.SchemeGroupVersion.String(),
						Kind:       "Role",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterPermission.Name,
						Namespace: role.Namespace,
					},
					Rules: role.Rules,
				})
			}
		}
	}

	// RoleBindings payload
	if clusterPermission.Spec.RoleBindings != nil && len(*clusterPermission.Spec.RoleBindings) > 0 {
		for _, roleBinding := range *clusterPermission.Spec.RoleBindings {
			rbSubjects := getSubjects(roleBinding.Subject, roleBinding.Subjects)
			if roleBinding.Namespace == "" && roleBinding.NamespaceSelector == nil {
				return nil, nil, nil, nil,
					errors.New("both RoleBinding Namespace and NamespaceSelector cannot be nil and empty")
			}
			if roleBinding.Namespace != "" && roleBinding.NamespaceSelector != nil {
				return nil, nil, nil, nil,
					errors.New("both RoleBinding Namespace and NamespaceSelector cannot populated at the same time")
			}
			if roleBinding.NamespaceSelector != nil {
				labelSelector, err := metav1.LabelSelectorAsSelector(roleBinding.NamespaceSelector)
				if err != nil {
					return nil, nil, nil, nil, err
				}

				nsList := &corev1.NamespaceList{}
				if err = r.Client.List(ctx, nsList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
					return nil, nil, nil, nil, err
				}

				if nsList == nil || nsList.Items == nil && len(nsList.Items) == 0 {
					return nil, nil, nil, nil,
						errors.New("unable to find any Namespace using NamespaceSelector")
				}

				for _, ns := range nsList.Items {
					if err := r.validateSubject(ctx, rbSubjects, clusterPermission.Namespace); err != nil {
						return nil, nil, nil, nil, err
					}

					subjects, err := r.generateSubjects(ctx, rbSubjects, clusterPermission.Namespace)
					if err != nil {
						return nil, nil, nil, nil, err
					}

					roleBindings = append(roleBindings, rbacv1.RoleBinding{
						TypeMeta: metav1.TypeMeta{
							APIVersion: rbacv1.SchemeGroupVersion.String(),
							Kind:       "RoleBinding",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      clusterPermission.Name,
							Namespace: ns.Name,
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.GroupName,
							Kind:     roleBinding.RoleRef.Kind,
							Name:     clusterPermission.Name,
						},
						Subjects: subjects,
					})
				}
			} else if roleBinding.Namespace != "" {
				if err := r.validateSubject(ctx, rbSubjects, clusterPermission.Namespace); err != nil {
					return nil, nil, nil, nil, err
				}

				subjects, err := r.generateSubjects(ctx, rbSubjects, clusterPermission.Namespace)
				if err != nil {
					return nil, nil, nil, nil, err
				}

				roleBindingName := clusterPermission.Name
				if roleBinding.Name != "" {
					roleBindingName = roleBinding.Name
				}

				roleBindingRoleRef := rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     roleBinding.RoleRef.Kind,
					Name:     clusterPermission.Name,
				}
				if roleBinding.RoleRef.APIGroup != "" && roleBinding.RoleRef.Name != "" {
					roleBindingRoleRef = rbacv1.RoleRef{
						APIGroup: roleBinding.RoleRef.APIGroup,
						Kind:     roleBinding.RoleRef.Kind,
						Name:     roleBinding.RoleRef.Name,
					}
				} else if roleBinding.RoleRef.APIGroup == "" && roleBinding.RoleRef.Name != "" {
					return nil, nil, nil, nil,
						errors.New("the RoleBinding for Namespace " + roleBinding.Namespace + " missing APIGroup for the RoleRef")
				} else if roleBinding.RoleRef.APIGroup != "" && roleBinding.RoleRef.Name == "" {
					return nil, nil, nil, nil,
						errors.New("the RoleBinding for Namespace " + roleBinding.Namespace + " missing Name for the RoleRef")
				}

				roleBindings = append(roleBindings, rbacv1.RoleBinding{
					TypeMeta: metav1.TypeMeta{
						APIVersion: rbacv1.SchemeGroupVersion.String(),
						Kind:       "RoleBinding",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleBindingName,
						Namespace: roleBinding.Namespace,
					},
					RoleRef:  roleBindingRoleRef,
					Subjects: subjects,
				})
			}
		}
	}

	return clusterRole, clusterRoleBindings, roles, roleBindings, nil
}

// handleValidation processes role validation when spec.validate is enabled
func (r *ClusterPermissionReconciler) handleValidation(ctx context.Context, clusterPermission *cpv1alpha1.ClusterPermission) error {
	log := log.FromContext(ctx)

	// Extract role references that need validation
	roleRefs := extractRoleReferencesForValidation(clusterPermission)
	if len(roleRefs) == 0 {
		log.Info("no role references found for validation")
		return nil
	}

	validationMWName := generateValidationManifestWorkName(*clusterPermission)
	validationManifestWork := buildValidationManifestWork(*clusterPermission, validationMWName, roleRefs)

	// Create or update validation ManifestWork
	var existingValidationMW workv1.ManifestWork
	err := r.Get(ctx, types.NamespacedName{Name: validationMWName, Namespace: clusterPermission.Namespace}, &existingValidationMW)
	if apierrors.IsNotFound(err) {
		log.Info("creating validation ManifestWork")
		err = r.Client.Create(ctx, validationManifestWork)
		if err != nil {
			log.Error(err, "unable to create validation ManifestWork")
			return err
		}
	} else if err == nil {
		log.Info("updating validation ManifestWork")
		existingValidationMW.Spec = validationManifestWork.Spec
		err = r.Client.Update(ctx, &existingValidationMW)
		if err != nil {
			log.Error(err, "unable to update validation ManifestWork")
			return err
		}
	} else {
		log.Error(err, "unable to fetch validation ManifestWork")
		return err
	}

	// Process validation results from ManifestWork status
	return r.processValidationResults(ctx, clusterPermission, validationMWName)
}

// processValidationResults processes the feedback from validation ManifestWork and updates status conditions
func (r *ClusterPermissionReconciler) processValidationResults(ctx context.Context, clusterPermission *cpv1alpha1.ClusterPermission, validationMWName string) error {
	log := log.FromContext(ctx)

	// Get the validation ManifestWork to check its status
	var validationMW workv1.ManifestWork
	success := false
	for i := 0; i < VALIDATION_MW_RETRY_COUNT; i++ {
		log.Info("checking validation ManifestWork status", "attempt", i+1)
		err := r.Get(ctx, types.NamespacedName{Name: validationMWName, Namespace: clusterPermission.Namespace}, &validationMW)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// ManifestWork doesn't exist yet, nothing to process
				return nil
			}
			return err
		}
		if validationMW.Status.Conditions[0].Type == "Applied" && validationMW.Status.Conditions[0].Status == "True" {
			log.Info("validation ManifestWork completed successfully")
			success = true
			break
		}
		time.Sleep(VALIDATION_MW_RETRY_INTERVAL)
	}

	if !success {
		log.Error(errors.New("validation ManifestWork failed to complete"), "validation ManifestWork failed to complete")
		return errors.New("validation ManifestWork failed to complete")
	}

	// Analyze the status feedback to determine which roles are missing
	var missingRoles []string
	var missingClusterRoles []string

	// Check feedback from ManifestWork status
	found := false
	for _, status := range validationMW.Status.ResourceStatus.Manifests {
		for _, condition := range status.Conditions {
			if condition.Type == "Available" && condition.Status == "True" {
				found = true
				break
			}
		}

		// If role not found in feedback or feedback indicates error, consider it missing
		if !found {
			if status.ResourceMeta.Kind == "Role" {
				missingRoles = append(missingRoles, status.ResourceMeta.Namespace+"/"+status.ResourceMeta.Name)
			} else if status.ResourceMeta.Kind == "ClusterRole" {
				missingClusterRoles = append(missingClusterRoles, status.ResourceMeta.Name)
			}
		}
	}

	// Update status conditions based on validation results
	var conditions []*metav1.Condition

	if len(missingRoles) > 0 {
		conditions = append(conditions, &metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
			Status:  metav1.ConditionFalse,
			Reason:  "RolesNotFound",
			Message: "The following roles were not found: " + joinStrings(missingRoles, ", "),
		})
	} else {
		conditions = append(conditions, &metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateRolesExist,
			Status:  metav1.ConditionTrue,
			Reason:  "AllRolesFound",
			Message: "All referenced roles were found",
		})
	}

	if len(missingClusterRoles) > 0 {
		conditions = append(conditions, &metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
			Status:  metav1.ConditionFalse,
			Reason:  "ClusterRolesNotFound",
			Message: "The following cluster roles were not found: " + joinStrings(missingClusterRoles, ", "),
		})
	} else {
		conditions = append(conditions, &metav1.Condition{
			Type:    cpv1alpha1.ConditionTypeValidateClusterRolesExist,
			Status:  metav1.ConditionTrue,
			Reason:  "AllClusterRolesFound",
			Message: "All referenced cluster roles were found",
		})
	}

	// Update all conditions
	for _, condition := range conditions {
		if err := r.updateStatus(ctx, clusterPermission, condition); err != nil {
			log.Error(err, "failed to update validation status condition", "type", condition.Type)
		}
	}

	return nil
}

// joinStrings joins a slice of strings with a separator
func joinStrings(strings []string, separator string) string {
	if len(strings) == 0 {
		return ""
	}
	if len(strings) == 1 {
		return strings[0]
	}

	result := strings[0]
	for i := 1; i < len(strings); i++ {
		result += separator + strings[i]
	}
	return result
}
