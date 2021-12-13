# Getting Started

## Prerequisites

- **Kubernetes cluster** - A Kubernetes cluster is required. You will need `cluster-admin` authority in order to complete all of the prescribed steps.

- **Kubectl and Kustomize** - The installation will occur via the terminal using [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/).

- **etcd** - ModelMesh Serving requires an [etcd](https://etcd.io/) server in order to coordinate internal state which can be either dedicated or shared. More on this later.

- **S3-compatible object storage** - Before models can be deployed, a remote S3-compatible datastore is needed from which to pull the model data. This could be for example an [IBM Cloud Object Storage](https://www.ibm.com/cloud/object-storage) instance, or a locally running [MinIO](https://github.com/minio/minio) deployment. Note that this is not required to be in place prior to the initial installation.

We provide an install script to quickly run ModelMesh Serving with a provisioned etcd server. This may be useful for experimentation or development but should not be used in production.

## Cluster Scope or Namespace Scope

ModelMesh Serving can be used in either cluster scope or namespace mode.

- **Cluster scope mode** - Its components can exist in multiple user namespaces which are controlled by one instance of ModelMesh Serving Controller in the control plane namespace. Only one ModelMesh Serving instance can be installed within a Kubernetes cluster. A namespace label `modelmesh-enabled` needs to be "true" to enable a user namespace for ModelMesh Serving.
- **Namespace scope mode** - All of its components must exist within a single namespace and only one instance of ModelMesh Serving can be installed per namespace. Multiple ModelMesh Serving instances can be installed in separate namespaces within the cluster.

The default configuration is for the cluster scope mode. Change RBAC permissions, cluster role to role, and cluster role binding to role binding, to deploy ModelMesh Serving in the namespace scope mode.

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

(\*) [`ScaleToZero`](../production-use/scaling.md#scale-to-zero) is enabled by default, so runtimes will have 0 replicas until a Predictor is created that uses that runtime. Once a Predictor is assigned, the runtime pods will scale up to 2.

When `ScaleToZero` **is enabled** (default), deployments for runtime pods will be scaled to 0 when there are no Predictors for that runtime. When `ScaletoZero` is enabled and first predictor CR is submitted, ModelMesh serving will spin up the corresponding built-in runtime pods.

When `ScaletoZero` is **disabled**, pods shown in 4 to 5 are created, with a total CPU(request/limit) of 6/63.1 and total memory(request/limit) of 11.11Gi/14.652Gi.

The deployed footprint can be significantly reduced in the following ways:

- Individual built-in runtimes can be disabled by setting `disabled: true` in their corresponding `ServingRuntime` resource - if the corresponding model types aren't used.

- The number of Pods per runtime can be changed from the default of 2 (e.g. down to 1), via the `podsPerRuntime` global configuration parameter (see [configuration](../configuration)). It is recommended for this value to be a minimum of 2 for production deployments.

- Memory and/or CPU resource allocations can be reduced (or increased) on the primary model server container in any of the built-in `ServingRuntime` resources (container name `triton` or `mlserver`). This has the effect of adjusting the total capacity available for holding served models in memory.

```shell
> kubectl edit servingruntime triton-2.x
> kubectl edit servingruntime mlserver-0.x
```

Please be aware that:

1. Changes made to the _built-in_ runtime resources will likely be reverted when upgrading/re-installing
2. Most of this resource allocation behaviour/config will change in future versions to become more dynamic - both the number of pods deployed and the system resources allocated to them

For more details see the [built-in runtime configuration](../configuration/built-in-runtimes.md)

The following resources will be created in the namespaces:

- `model-serving-defaults` - ConfigMap holding default values tied to a release, should not be modified. Configuration can be overriden by creating a user ConfigMap, see [configuration](../configuration)
- `tc-config` - ConfigMap used for some internal coordination
- `storage-config` - Secret holding config for each of the storage backends from which models can be loaded - see [the example](../predictors/)
- `model-serving-etcd` - Secret providing access to the Etcd cluster. It is created by user in the controller namespace - see [instructions](../install/install-script.md#setup-the-etcd-connection-information), and will be automatically created in user namespaces when in the cluster scope mode.

## Next Steps

- See [the configuration page](../configuration) for details of how to configure system-wide settings via a ConfigMap, either before or after installation.

- See this [example walkthrough](../predictors) of deploying a TensorFlow model as a `Predictor`.
