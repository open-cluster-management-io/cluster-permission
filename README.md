# ClusterPermission

`ClusterPermission` is an Open Cluster Management (OCM) custom resource that enables administrators to automatically distribute RBAC resources to managed clusters and manage their lifecycle. It provides centralized management of Roles, ClusterRoles, RoleBindings, and ClusterRoleBindings across multiple Kubernetes clusters.

## Overview

This project complements [ManagedServiceAccount](https://github.com/open-cluster-management-io/managed-serviceaccount) by addressing the authorization aspects of fleet management. While `ManagedServiceAccount` handles authentication across clusters, `ClusterPermission` manages authorization by distributing and maintaining RBAC resources.

### Key Features

- **Automated RBAC Distribution**: Automatically deploys RBAC resources to managed clusters
- **Lifecycle Management**: Uses OCM's ManifestWork API for creation, updates, and deletion
- **ManagedServiceAccount Integration**: Supports ManagedServiceAccount as a binding subject
- **Multi-cluster Authorization**: Centralized authorization management across OCM fleet
- **Resource Protection**: Safeguards distributed RBAC resources against unintended modifications

## Architecture

A `ClusterPermission` resource must reside in an OCM managed cluster namespace on the Hub cluster. The controller:

1. Validates the ClusterPermission specification
2. Generates appropriate RBAC manifests
3. Creates ManifestWork resources to deploy RBAC to target managed clusters
4. Monitors and maintains the lifecycle of distributed resources

Supported RBAC resources:
- **ClusterRole** and **ClusterRoleBinding**
- **Role** and **RoleBinding** (with namespace targeting)
- **Standard subjects**: User, Group, ServiceAccount
- **Enhanced subjects**: ManagedServiceAccount (requires ManagedServiceAccount addon)

## Prerequisites

- **Open Cluster Management (OCM)** environment with Hub and managed clusters
  - See [OCM Quick Start](https://open-cluster-management.io/getting-started/quick-start/) for setup instructions
- **Optional**: [ManagedServiceAccount addon](https://github.com/open-cluster-management-io/managed-serviceaccount) for enhanced authentication features

## Installation

### Option 1: Development Installation

1. Clone the repository and install CRDs:
```bash
git clone https://github.com/open-cluster-management-io/cluster-permission.git
cd cluster-permission/
make install
```

2. Run the controller locally:
```bash
make run
```

### Option 2: Helm Chart Installation

Deploy using the provided Helm chart:
```bash
helm install cluster-permission ./chart/
```

### Option 3: Direct Deployment

Apply the deployment manifests:
```bash
kubectl apply -f config/deploy/
```

## Quick Start

### 1. Basic ClusterPermission Example

Create a ClusterPermission in your managed cluster namespace (replace `cluster1` with your managed cluster name):

```bash
kubectl apply -f - <<EOF
apiVersion: rbac.open-cluster-management.io/v1alpha1
kind: ClusterPermission
metadata:
  name: example-permissions
  namespace: cluster1
spec:
  clusterRole:
    rules:
    - apiGroups: ["apps"]
      resources: ["deployments"]
      verbs: ["get", "list", "watch"]
  clusterRoleBinding:
    subject:
      kind: ServiceAccount
      name: my-service-account
      namespace: default
EOF
```

### 2. Verify Deployment

Check the ClusterPermission status:
```bash
kubectl -n cluster1 get clusterpermission example-permissions -o yaml
```

Expected status:
```yaml
status:
  conditions:
  - lastTransitionTime: "2023-04-12T15:19:04Z"
    message: |-
      Run the following command to check the ManifestWork status:
      kubectl -n cluster1 get ManifestWork example-permissions-xxxxx -o yaml
    reason: AppliedRBACManifestWork
    status: "True"
    type: AppliedRBACManifestWork
```

### 3. Verify RBAC Resources on Managed Cluster

On the managed cluster, verify the RBAC resources were created:
```bash
kubectl get clusterrole | grep example-permissions
kubectl get clusterrolebinding | grep example-permissions
```

## Usage Examples

### Standard RBAC Subjects

Apply the basic sample:
```bash
kubectl -n cluster1 apply -f config/samples/rbac.open-cluster-management.io_v1alpha1_clusterpermission.yaml
```

### Users and Groups

For user and group-based permissions:
```bash
kubectl -n cluster1 apply -f config/samples/clusterpermission_users_groups.yaml
```

### ManagedServiceAccount Integration

To use ManagedServiceAccount as a subject:
```bash
kubectl -n cluster1 apply -f config/samples/clusterpermission_subject_msa.yaml
```

### Multiple ClusterRoleBindings

For complex permission scenarios:
```bash
kubectl -n cluster1 apply -f config/samples/clusterpermission_multiple_clusterrolebindings.yaml
```

## Configuration Reference

### ClusterPermissionSpec

| Field | Type | Description |
|-------|------|-------------|
| `clusterRole` | `ClusterRole` | ClusterRole to create on managed cluster |
| `clusterRoleBinding` | `ClusterRoleBinding` | ClusterRoleBinding to create |
| `clusterRoleBindings` | `[]ClusterRoleBinding` | Multiple ClusterRoleBindings |
| `roles` | `[]Role` | Roles to create with namespace targeting |
| `roleBindings` | `[]RoleBinding` | RoleBindings with namespace support |

### Subject Types

- **ServiceAccount**: `kind: ServiceAccount`
- **User**: `kind: User`
- **Group**: `kind: Group`
- **ManagedServiceAccount**: `kind: ManagedServiceAccount` (requires addon)

## Troubleshooting

### Common Issues

1. **ClusterPermission not applying**
   - Verify the namespace is a valid managed cluster namespace
   - Check OCM hub cluster connectivity

2. **RBAC resources not appearing on managed cluster**
   - Check ManifestWork status: `kubectl -n <cluster-ns> get manifestwork`
   - Verify managed cluster agent connectivity

3. **ManagedServiceAccount subjects not working**
   - Ensure ManagedServiceAccount addon is installed
   - Verify the referenced ManagedServiceAccount exists

### Debugging Commands

```bash
# Check ClusterPermission status
kubectl -n <cluster-namespace> get clusterpermission <name> -o yaml

# Check associated ManifestWork
kubectl -n <cluster-namespace> get manifestwork

# View controller logs
kubectl logs -n cluster-permission-system deployment/cluster-permission-controller-manager
```

## Development

### Building from Source

```bash
# Build the binary
make build

# Build Docker image
make docker-build

# Run tests
make test

# Generate CRDs
make manifests

# Update generated code
make generate
```

### Code Generation

After modifying API types, regenerate code:
```bash
make generate
make manifests
```

## Community and Support

### Contributing

Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for contribution guidelines.

### Communication

- **Slack**: [#open-cluster-mgmt](https://kubernetes.slack.com/channels/open-cluster-mgmt)
- **GitHub Issues**: Report bugs and feature requests
- **GitHub Discussions**: Community questions and discussions

### Related Projects

- [Open Cluster Management](https://open-cluster-management.io/)
- [ManagedServiceAccount](https://github.com/open-cluster-management-io/managed-serviceaccount)
- [ManifestWork](https://open-cluster-management.io/concepts/manifestwork/)

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
