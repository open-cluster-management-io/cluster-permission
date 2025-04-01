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
kubectl apply -f config/samples/clusterpermission_existing_roles.yaml -n cluster1
sleep 10
work_kubectl_command=$(kubectl -n cluster1 get clusterpermission -o yaml | grep kubectl | grep ManifestWork)
if $work_kubectl_command; then
    echo "ManifestWork found"
else
    echo "ManifestWork not found"
    exit 1
fi

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

if kubectl -n default get rolebinding default-rb-cluster1; then
    echo "default-rb-cluster1 rolebinding found"
else
    echo "default-rb-cluster1 rolebinding not found"
    exit 1
fi

