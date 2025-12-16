#!/bin/bash

set -e

echo "Installing managed-serviceaccount..."

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
echo "Created temporary directory: $TEMP_DIR"

# Clone the managed-serviceaccount repository
echo "Cloning managed-serviceaccount repository..."
git clone https://github.com/open-cluster-management-io/managed-serviceaccount.git "$TEMP_DIR/managed-serviceaccount"

# Change to the repository directory
cd "$TEMP_DIR/managed-serviceaccount"

# Deploy managed-serviceaccount
echo "Deploying managed-serviceaccount..."

helm upgrade --install \
    -n open-cluster-management-addon --create-namespace \
    managed-serviceaccount charts/managed-serviceaccount/ \
    --set tag=latest \
    --set featureGates.ephemeralIdentity=true \
    --set hubDeployMode=AddOnTemplate \
    --set targetCluster=cluster1

# Clean up
echo "Cleaning up temporary directory..."
cd -
rm -rf "$TEMP_DIR"

echo "managed-serviceaccount installation completed successfully!"
