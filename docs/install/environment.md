## Prerequisites

- **Kubernetes cluster** - A Kubernetes cluster is required. You will need `cluster-admin` authority in order to complete all of the prescribed steps.

- **Kubectl and Kustomize** - The installation will occur via the terminal using [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/).

- **etcd** - ModelMesh Serving requires an [etcd](https://etcd.io/) server in order to coordinate internal state which can be either dedicated or shared. More on this later.

We provide an install script to quickly run ModelMesh Serving with a provisioned etcd server. This may be useful for experimentation or development but should not be used in production.

## Namespace Scope

ModelMesh Serving is namespace scoped, meaning all of its components must exist within a single namespace and only one instance of ModelMesh Serving can be installed per namespace.Multiple ModelMesh Serving instances can be installed in separate namespaces within the cluster.

## Deployed Components

|            | Type             | Pod                        | Count | Default CPU request/limit per-pod | Default mem request/limit per-pod |
| ---------- | ---------------- | -------------------------- | ----- | --------------------------------- | --------------------------------- |
| 1          | Controller       | modelmesh controller pod   | 1     | 50m / 1                           | 96Mi / 512Mi                      |
| 2          | Object Storage   | minio pod                  | 1     | 200m / 200m                       | 256Mi / 256Mi                     |
| 3          | Metastore        | ETCD pods                  | 1     | 200m / 200m                       | 512Mi / 512Mi                     |
| 4          | Built-in Runtime | Nvidia Triton runtime Pods | 2     | 850m / 10                         | 1568Mi / 1984Mi                   |
| 5          | Built-in Runtime | The MLServer runtime Pods  | 2     | 850m / 10                         | 1568Mi / 1984Mi                   |
| **totals** |                  |                            | 7     | 3850m / 41.4                      | 6.96Gi / 8.59Gi                   |

When ModelMesh serving instance is installed with the `--quickstart` option, pods shown in 1 - 5 are created with a total CPU(request/limit) of 3850m / 41.4 and total memory(request/limit) of 6.96Gi / 8.59Gi.

By default, runtime Pods will only be started when there's at least one Predictor created that uses a corresponding model type.

The deployed footprint can be significantly reduced in the following ways:

- Individual built-in runtimes can be disabled by setting `disabled: true` in their corresponding `ServingRuntime` resource - if the corresponding model types aren't used.

- The number of Pods per runtime can be changed from the default of 2 (e.g. down to 1), via the `podsPerRuntime` global configuration parameter (see [configuration](../configuration)). It is recommended for this value to be a minimum of 2 for production deployments.

- Memory and/or CPU resource allocations can be reduced (or increased) on the primary model server container in either of the built-in `ServingRuntime` resources (container name `triton` or `mlserver`). This has the effect of adjusting the total capacity available for holding served models in memory.

```shell
> kubectl edit servingruntime triton-2.x
> kubectl edit servingruntime mlserver-0.x
```

Please be aware that:

1. Changes made to the _built-in_ runtime resources will likely be reverted when upgrading/re-installing
2. Most of this resource allocation behaviour/config will change in future versions to become more dynamic - both the number of pods deployed and the system resources allocated to them

The following resources will also be created in the same namespace:

- `model-serving-defaults` - ConfigMap holding default values tied to a release, should not be modified. Configuration can be overriden by creating a user ConfigMap, see [configuration](../configuration)
- `tc-config` - ConfigMap used for some internal coordination
- `storage-config` - Secret holding config for each of the storage backends from which models can be loaded - see [storage config[(../predictors/storage.md) and [the example](../predictors/README.md)
