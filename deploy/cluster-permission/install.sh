#!/bin/bash

set -o nounset
set -o pipefail

echo "SETUP install cluster-permission"
kubectl apply -f config/crds
kubectl apply -f config/rbac
kubectl apply -f config/deploy
if kubectl wait --for=condition=available --timeout=600s deployment/cluster-permission -n open-cluster-management; then
    echo "Deployment available"
else
    echo "Deployment not available"
    exit 1
fi

echo "############  cluster-permission controller is installed successfully!!"
