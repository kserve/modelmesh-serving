# Getting Started

## Prerequisites

- **Kubernetes cluster** - A Kubernetes cluster is required. You will need `cluster-admin` authority in order to complete all of the prescribed steps.

- **Kubectl and Kustomize** - The installation will occur via the terminal using [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/).

- **etcd** - ModelMesh Serving requires an [etcd](https://etcd.io/) server in order to coordinate internal state which can be either dedicated or shared. More on this later.

- **S3-compatible object storage** - Before models can be deployed, a remote S3-compatible datastore is needed from which to pull the model data. This could be for example an [IBM Cloud Object Storage](https://www.ibm.com/cloud/object-storage) instance, or a locally running [MinIO](https://github.com/minio/minio) deployment. Note that this is not required to be in place prior to the initial installation.

We provide an install script `--quickstart` option to quickly run ModelMesh Serving with a provisioned etcd server. This may be useful for experimentation or development but should not be used in production.

## Cluster Scope or Namespace Scope

ModelMesh Serving can be used in either cluster scope or namespace mode.

- **Cluster scope mode** - Its components can exist in multiple user namespaces which are controlled by one instance of ModelMesh Serving Controller in the control plane namespace. Only one ModelMesh Serving instance can be installed within a Kubernetes cluster. A namespace label `modelmesh-enabled` needs to be "true" to enable a user namespace for ModelMesh Serving.
- **Namespace scope mode** - All of its components must exist within a single namespace and only one instance of ModelMesh Serving can be installed per namespace. Multiple ModelMesh Serving instances can be installed in separate namespaces within the cluster.

The default configuration is for the cluster scope mode. Use the `--namespace-scope-mode` option of the install script for namespace scope.

## Deployed Components

|            | Type             | Pod                        | Count   | Default CPU request/limit per-pod | Default mem request/limit per-pod          |
| ---------- | ---------------- | -------------------------- | ------- | --------------------------------- | ------------------------------------------ |
| 1          | Controller       | modelmesh controller pod   | 1       | 50m / 1                           | 96Mi / 512Mi                               |
| 2          | Object Storage   | MinIO pod (optional)       | 1       | 200m / 200m                       | 256Mi / 256Mi                              |
| 3          | Metastore        | ETCD pod                   | 1       | 200m / 200m                       | 512Mi / 512Mi                              |
| 4          | Built-in Runtime | Nvidia Triton runtime Pods | 0 \(\*) | 850m / 10 or 900m / 11 \(\*\*)    | 1568Mi / 1984Mi or 1664Mi / 2496Mi \(\*\*) |
| 5          | Built-in Runtime | The MLServer runtime Pods  | 0 \(\*) | 850m / 10 or 900m / 11 \(\*\*)    | 1568Mi / 1984Mi or 1664Mi / 2496Mi \(\*\*) |
| 6          | Built-in Runtime | The OVMS runtime Pods      | 0 \(\*) | 850m / 10 or 900m / 11 \(\*\*)    | 1568Mi / 1984Mi or 1664Mi / 2496Mi \(\*\*) |
| **totals** |                  |                            | 3       | 450m / 1.4                        | 864Mi / 1.25Gi                             |

When a ModelMesh Serving instance is installed with the `--quickstart` option, pods shown in 1 - 6 are created.
However, do note that the quickstart-deployed etcd and MinIO pods are intended for development/experimentation and not for production.

(\*) [`ScaleToZero`](../production-use/scaling.md#scale-to-zero) is enabled by default, so runtimes will have 0 replicas until an `InferenceService` is created that uses that runtime. Once an `InferenceService` is assigned, the runtime pods will scale up to 2.

When `ScaleToZero` **is enabled** (default), deployments for runtime pods will be scaled to 0 when there are no `InferenceService`s for that runtime. When `ScaletoZero` is enabled and first `InferenceService` CR is submitted, ModelMesh Serving will spin up the corresponding built-in runtime pods.

When `ScaletoZero` is **disabled**, pods shown in 4 to 6 are created (default two pods per runtime), which will greatly increase the total CPU(request/limit) and total memory(request/limit).

(\*\*) When the REST inferencing is enabled via the `restProxy` config parameter, every model serving pod will include an additional container that consumes resources. The default allocation for this proxy container is:

```yaml
resources:
  requests:
    cpu: "50m"
    memory: "96Mi"
  limits:
    cpu: "1"
    memory: "512Mi"
```

The deployed footprint can be significantly reduced in the following ways:

- Individual built-in runtimes can be disabled by setting `disabled: true` in their corresponding `ServingRuntime` resource - if the corresponding model types aren't used.

- The number of Pods per runtime can be changed from the default of 2 (e.g. down to 1), via the `podsPerRuntime` global configuration parameter (see [configuration](../configuration)). It is recommended for this value to be a minimum of 2 for production deployments.

- Memory and/or CPU resource allocations can be reduced (or increased) on the primary model server container in any of the built-in `ServingRuntime` resources (container name `triton`, `mlserver`, or `ovms`). This has the effect of adjusting the total capacity available for holding served models in memory.

```shell
> kubectl edit servingruntime triton-2.x
> kubectl edit servingruntime mlserver-0.x
> kubectl edit servingruntime ovms-1.x
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

- See this [example walkthrough](../predictors) of deploying a TensorFlow model as an `InferenceService`.
