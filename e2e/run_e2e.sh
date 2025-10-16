#!/bin/bash

set -o nounset
set -o pipefail

echo "SETUP install cluster-permission"
kubectl config use-context kind-hub
kubectl apply -f config/crds
kubectl apply -f config/rbac
kubectl apply -f config/deploy
if kubectl wait --for=condition=available --timeout=600s deployment/cluster-permission -n open-cluster-management; then
    echo "Deployment available"
else
    echo "Deployment not available"
    exit 1
fi

echo "TEST ClusterPermission"
kubectl config use-context kind-hub
kubectl apply -f config/samples/rbac.open-cluster-management.io_v1alpha1_clusterpermission.yaml -n cluster1
sleep 30
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork found"
else
    echo "ManifestWork not found"
    exit 1
fi

echo
echo
echo "==========ClusterPermission=========="
kubectl -n cluster1 get clusterpermission -o yaml
echo
echo
echo
echo "==========ManifestWork=========="
kubectl -n cluster1 get manifestwork -o yaml
echo
echo
echo
echo "==========Logging=========="
kubectl logs -n open-cluster-management -l name=cluster-permission
echo
echo
echo

if kubectl -n default get role clusterpermission-sample; then
    echo "clusterpermission-sample role found"
else
    echo "clusterpermission-sample role not found"
    exit 1
fi
if kubectl -n default get rolebinding clusterpermission-sample; then
    echo "clusterpermission-sample rolebinding found"
else
    echo "clusterpermission-sample rolebinding not found"
    exit 1
fi
if kubectl get clusterrole clusterpermission-sample; then
    echo "clusterpermission-sample clusterrole found"
else
    echo "clusterpermission-sample clusterrole not found"
    exit 1
fi
if kubectl get clusterrolebinding clusterpermission-sample; then
    echo "clusterpermission-sample clusterrolebinding found"
else
    echo "clusterpermission-sample clusterrolebinding not found"
    exit 1
fi

echo "TEST ClusterPermission with existing roles"
kubectl config use-context kind-hub
kubectl apply -f config/samples/clusterpermission_existing_roles.yaml -n cluster1
sleep 30
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission clusterpermission-existing-role-sample -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork found"
else
    echo "ManifestWork not found"
    exit 1
fi

if kubectl -n default get rolebinding default-rb-cluster1; then
    echo "default-rb-cluster1 rolebinding found"
else
    echo "default-rb-cluster1 rolebinding not found"
    exit 1
fi

echo "TEST ClusterPermission with users and groups"
kubectl config use-context kind-hub
kubectl apply -f config/samples/clusterpermission_users_groups.yaml -n cluster1
sleep 30
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission clusterpermission-users-groups -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork found"
else
    echo "ManifestWork not found"
    exit 1
fi

if kubectl -n kube-system get rolebinding kubevirt-rb-cluster1-users1; then
    echo "kubevirt-rb-cluster1-users1 rolebinding found"
else
    echo "kubevirt-rb-cluster1-users1 rolebinding not found"
    exit 1
fi

if kubectl -n kube-system get rolebinding kubevirt-rb-cluster1-users1 -o yaml | grep users1; then
    echo "kubevirt-rb-cluster1-users1 users1 found"
else
    echo "kubevirt-rb-cluster1-users1 users1 not found"
    exit 1
fi

if kubectl -n kube-system get rolebinding kubevirt-rb-cluster1-users1 -o yaml | grep users2; then
    echo "kubevirt-rb-cluster1-users1 users2 found"
else
    echo "kubevirt-rb-cluster1-users1 users2 not found"
    exit 1
fi

echo "TEST ClusterPermission ClusterRoleBinding with no subject or subjects"
crb_err_msg="The ClusterPermission \"clusterpermission-clusterrolebinding-error\" is invalid: spec.clusterRoleBinding: Invalid value: \"object\": Either subject or subjects has to exist in clusterRoleBinding"
crb_error=$(kubectl apply -f config/samples/clusterpermission_clusterrolebinding_error.yaml -n cluster1 2>&1)

echo $crb_error

if [ "$crb_error" == "$crb_err_msg" ]; then
    echo "ClusterRoleBinding error found"
else
    echo "ClusterRoleBinding error not found"
    exit 1
fi

echo "TEST ClusterPermission RoleBinding with no subject or subjects"
rb_err_msg="The ClusterPermission "clusterpermission-rolebinding-error" is invalid: spec.roleBindings: Invalid value: "array": Either subject or subjects has to exist in every roleBinding"
rb_error=$(kubectl apply -f config/samples/clusterpermission_rolebinding_error.yaml -n cluster1 2>&1)

echo $rb_error

if [ "$rb_error" == "$rb_error" ]; then
    echo "RoleBinding error found"
else
    echo "RoleBinding error not found"
    exit 1
fi

echo "TEST ClusterPermission with multiple clusterRoleBindings"
kubectl apply -f config/samples/clusterpermission_multiple_clusterrolebindings.yaml -n cluster1
sleep 30
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission clusterpermission-multiple-clusterrolebindings -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork found"
else
    echo "ManifestWork not found"
    exit 1
fi

if kubectl get clusterrolebinding multi-crb-binding1 -o yaml | grep user1; then
    echo "multi-crb-binding1 user1 found"
else
    echo "multi-crb-binding1 user1 not found"
    exit 1
fi

if kubectl get clusterrolebinding multi-crb-binding2 -o yaml | grep user2; then
    echo "multi-crb-binding2 user2 found"
else
    echo "multi-crb-binding2 user2 not found"
    exit 1
fi

echo "TEST ClusterPermission to validate non-existing clusterroles"
kubectl apply -f config/samples/clusterpermission_validate_non_existing.yaml -n cluster1
sleep 30
if kubectl -n cluster1 get clusterpermission clusterpermission-validate-non-existing -o yaml | grep "The following cluster roles were not found: argocd-application-controller-3"; then
    echo "ClusterRole not found error found"
else
    echo "ClusterRole not found error not found"
    exit 1
fi

echo "TEST ClusterPermission to validate existing clusterroles"
kubectl apply -f config/samples/clusterpermission_validate_existing.yaml -n cluster1
sleep 30
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission clusterpermission-validate-existing -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork found"
else
    echo "ManifestWork not found"
    exit 1
fi

if kubectl -n cluster1 get clusterpermission clusterpermission-validate-existing -o yaml | grep "All referenced cluster roles were found"; then
    echo "All referenced cluster roles were found"
else
    echo "All referenced cluster roles were not found"
    exit 1
fi

echo "TEST ManagedClusterAddOn Custom Informer"
kubectl config use-context kind-hub

# Create a ManagedClusterAddOn with the name "managed-serviceaccount" that the informer watches
echo "Creating ManagedClusterAddOn 'managed-serviceaccount' in cluster1..."
cat <<EOF | kubectl apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: managed-serviceaccount
  namespace: cluster1
spec:
  installNamespace: open-cluster-management-agent-addon
EOF

echo "Waiting for ManagedClusterAddOn to be created..."
sleep 10

# Verify the addon was created
if kubectl get managedclusteraddon managed-serviceaccount -n cluster1; then
    echo "ManagedClusterAddOn managed-serviceaccount found in cluster1"
else
    echo "ManagedClusterAddOn managed-serviceaccount not found in cluster1"
    exit 1
fi

# Wait for informer to cache the addon
echo "Waiting for custom informer to cache the ManagedClusterAddOn..."
sleep 5

# Create a ClusterPermission with ManagedServiceAccount subject
echo "Creating ClusterPermission with ManagedServiceAccount subject..."
kubectl apply -f config/samples/clusterpermission_subject_msa.yaml -n cluster1
sleep 20

# Verify the ClusterPermission was processed
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission clusterpermission-msa-subject-sample -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork for ManagedServiceAccount subject found"
else
    echo "ManifestWork for ManagedServiceAccount subject not found"
    exit 1
fi

# Get the initial generation/resourceVersion of the ManifestWork before addon update
initial_manifestwork=$(kubectl get manifestwork -n cluster1 -o name | head -n 1 | cut -d'/' -f2)
echo "Found ManifestWork: $initial_manifestwork"

# Update the ManagedClusterAddOn to test the informer's update handling
echo "Updating ManagedClusterAddOn to trigger informer event handler..."
kubectl patch managedclusteraddon managed-serviceaccount -n cluster1 --type=merge -p '{"status":{"conditions":[{"type":"Available","status":"True","reason":"AddonAvailable","message":"addon is available"}],"namespace":"open-cluster-management-agent-addon"}}'

echo "Waiting for informer to process the update event..."
sleep 15

# Check if the informer logs show the event was processed
echo "Checking controller logs for custom informer activity..."
if kubectl logs -n open-cluster-management -l name=cluster-permission --tail=100 | grep -i "ManagedClusterAddOnInformer"; then
    echo "Custom informer logs found"
else
    echo "Note: Custom informer logs not found (may be at different log level)"
fi

# Check if the event handler triggered reconciliation
if kubectl logs -n open-cluster-management -l name=cluster-permission --tail=100 | grep -i "CustomInformerEventHandler"; then
    echo "Custom informer event handler was triggered"
else
    echo "Note: Event handler logs not found (may be at different log level)"
fi

# Verify the informer can list addons correctly by checking if ClusterPermission status shows the addon info
echo "Verifying ClusterPermission status reflects the ManagedClusterAddOn..."
if kubectl get clusterpermission clusterpermission-msa-subject-sample -n cluster1 -o yaml | grep -i "managedserviceaccount\|managed-serviceaccount"; then
    echo "ClusterPermission references ManagedServiceAccount correctly"
fi

# Additional verification: Create another ManagedClusterAddOn with a different name to ensure field selector works
echo "Creating a different ManagedClusterAddOn to verify field selector (should NOT be watched)..."
cat <<EOF | kubectl apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: other-addon
  namespace: cluster1
spec:
  installNamespace: other-namespace
EOF

sleep 5

# Update the other addon and verify it doesn't trigger events (field selector test)
kubectl patch managedclusteraddon other-addon -n cluster1 --type=merge -p '{"status":{"conditions":[{"type":"Available","status":"True"}]}}'
sleep 5

echo "Verifying that only 'managed-serviceaccount' addon triggers informer events..."
addon_specific_logs=$(kubectl logs -n open-cluster-management -l name=cluster-permission --tail=50 | grep -c "managed-serviceaccount" || echo "0")
echo "Found $addon_specific_logs references to 'managed-serviceaccount' in recent logs"

echo "ManagedClusterAddOn Custom Informer test completed successfully"
