# ClusterPermission
`ClusterPermission` is an OCM custom resource that enables administrators to automatically distribute RBAC resources to managed clusters and manage the lifecycle of those resources. It provides functionality for handling Roles, ClusterRoles, RoleBindings, and ClusterRoleBindings.

This project aims to improve the usability of `ManagedServiceAccount` ([GitHub link](https://github.com/open-cluster-management-io/managed-serviceaccount)). `ManagedServiceAccount` facilitates authentication fleet management, while `ClusterPermission` addresses the authorization aspects of fleet management.

## Description
This repository contains the API definition and controller for `ClusterPermission`.

A valid `ClusterPermission` resource should reside in an OCM "managed cluster namespace" and the associated RBAC resources will be deployed to the managed cluster associated with that managed cluster namespace. The `ClusterPermission` controller utilizes the `ManifestWork` API to ensure the creation, update, and removal of RBAC resources on each managed cluster. The `ClusterPermission` API safeguards these distributed RBAC resources against unintended modifications and removal.

Apart from the typical RBAC binding subject resources (Group, ServiceAccount, and User), `ManagedServiceAccount` can also serve as a subject. When the binding subject is a `ManagedServiceAccount`, the controller computes and generates RBAC resources based on the ServiceAccount managed by the `ManagedServiceAccount`.

## Dependencies
- The Open Cluster Management (OCM) multi-cluster environment needs to be setup. See [OCM website](https://open-cluster-management.io/) on how to setup the environment.
- Optional: [ManagedServiceAccount](https://github.com/open-cluster-management-io/managed-serviceaccount) add-on installed if you want to leverage `ManagedServiceAccount` resource as RBAC subject.

## Getting Started
1. Setup an OCM Hub cluster and registered an OCM Managed cluster. See [Open Cluster Management Quick Start](https://open-cluster-management.io/getting-started/quick-start/) for more details.

2. On the Hub cluster, install the `ClusterPermission` API and run the controller,
```
cd cluster-permission/
make install
make run
```

3. On the Hub cluster, apply the sample (modify the cluster1 namespace to your managed cluster name):
```
kubectl -n cluster1 apply -f config/samples/rbac.open-cluster-management.io_v1alpha1_clusterpermission
kubectl -n cluster1 get clusterpermission -o yaml
...
  status:
    conditions:
    - lastTransitionTime: "2023-04-12T15:19:04Z"
      message: |-
        Run the following command to check the ManifestWork status:
        kubectl -n cluster1 get ManifestWork clusterpermission-sample-f15f0 -o yaml
      reason: AppliedRBACManifestWork
      status: "True"
      type: AppliedRBACManifestWork
```

4. On the Managed cluster, check the RBAC resources
```
kubectl -n default get role
NAME                 CREATED AT
clusterpermission-sample   2023-04-12T15:19:04Z
```

## Community, discussion, contribution, and support

Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for how to contribute to the repo.

### Communication channels

Slack channel: [#open-cluster-mgmt](https://kubernetes.slack.com/channels/open-cluster-mgmt)

## License

This code is released under the Apache 2.0 license. See the file [LICENSE](LICENSE) for more information.
