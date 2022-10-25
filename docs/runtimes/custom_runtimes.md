# Implementing a Custom Serving Runtime

ModelMesh Serving serves different kinds of models via different _Serving Runtime_ implementations. A Serving Runtime is one or more containers which:

- Can dynamically load and unload models from disk into memory on demand
- Exposes a gRPC service endpoint to serve inferencing requests for loaded models

More specifically, the container(s) must:

1. Implement the simple [model management gRPC SPI](#model-server-management-spi) which comprises RPC methods to load/unload models, report their size, and report the runtime's total capacity
2. Implement one or more other _arbitrary_ gRPC services to serve inferencing requests for already-loaded models

These gRPC services for (2) must all be served from the same server endpoint. The management service SPI may be served by that same endpoint or a different one. Each of these endpoints may listen on a `localhost` port, or a unix domain socket. For best performance, a domain socket is preferred for the inferencing endpoint, and the corresponding file should be created in an empty dir within one of the containers. This dir will become a mount in _all_ of the runtime containers when they are run.

<!--
## Storage Options

_TODO_ add later
-->

## Model server Management SPI

Below is a description of how to implement the `mmesh.ModelRuntime` gRPC service, specified in [`model-runtime.proto`](https://github.com/kserve/modelmesh-serving/blob/main/docs/model-runtime.proto). Note that this is currently subject to change, but we will try to ensure that any changes are backwards-compatible or at least will require minimal change on the runtime side.

### Model sizing

So that ModelMesh Serving can decide when/where models should be loaded and unloaded, a given serving runtime implementation must communicate details of how much capacity it has to hold loaded models in memory, as well as how much each loaded model consumes.

Model sizes are communicated in a few different ways:

- A rough "global" default/average size for all models must be provided in the `defaultModelSizeInBytes` field in the response to the [`runtimeStatus`](#runtimestatus) rpc method. This should be a very conservative estimate.
- A _predicted_ size can optionally be provided by implementing the `predictModelSize` rpc method. This will be called prior to `loadModel` and if implemented should return immediately (for example it should not make remote calls which could be delayed).
- The more precise size of an already-loaded model can be provided by either:

  1. Including it in the `sizeInBytes` field of the response to the `loadModel` rpc method
  2. Not setting in the `loadModel` response, and instead implementing the separate `modelSize` method to return the size. This will be called immediately after `loadModel` returns, and isn't required to be implemented if the first option is used.

  The second of these last two options is preferred when a separate step is required to determine the size after the model has already been loaded. This is so that the model can start to be used for inferencing immediately, while the sizing operation is still in progress.

Capacity is indicated once via the `capacityInBytes` field in the response to the [`runtimeStatus`](#runtimestatus) rpc method and assumed to be constant.

Ideally, the value of `capacityInBytes` should be calculated dynamically as a function of your model server container's allocated memory. One way to arrange this is via Kubernetes' [Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/#store-container-fields) - mapping the container's `requests.memory` property to an environment variable. Of course some amount of fixed overhead should likely be subtracted from this value:

```yaml
env:
  - name: MODEL_SERVER_MEM_REQ_BYTES
    valueFrom:
      resourceFieldRef:
        containerName: my-model-server
        resource: requests.memory
```

### `runtimeStatus`

```protobuf
message RuntimeStatusRequest {}
```

This is polled at the point that the main model-mesh container starts to check that the runtime is ready. You should return a response with `status` set to `STARTING` until the runtime is ready to accept other requests and load/serve models at which point `status` should be set to `READY`.

The other fields in the response only need to be set in the `READY` response (and will be ignored prior to that). Once `READY` is returned, no further calls will be made unless the model-mesh container unexpectedly restarts.

Currently, to ensure overall consistency of the system, it is required that runtimes purge any loaded/loading models when receiving a `runtimeStatus` call, and do not return `READY` until this is complete. Typically, it's only called during initialization prior to any load/unloadModel calls and hence this "purge" will be a no-op. But runtimes should also handle the case where there _are_ models loaded. It is likely that this requirement will be removed in a future update, but ModelMesh Serving will remain compatible with runtimes that still include the logic.

```protobuf
message RuntimeStatusResponse {
    enum Status {
        STARTING = 0;
        READY = 1;
        FAILING = 2; //not used yet
    }
    Status status = 1;
    // memory capacity for static loaded models, in bytes
    uint64 capacityInBytes = 2;
    // maximum number of model loads that can be in-flight at the same time
    uint32 maxLoadingConcurrency = 3;
    // timeout for model loads in milliseconds
    uint32 modelLoadingTimeoutMs = 4;
    // conservative "default" model size,
    // such that "most" models are smaller than this
    uint64 defaultModelSizeInBytes = 5;
    // version string for this model server code
    string runtimeVersion = 6;

    message MethodInfo {
        // "path" of protobuf field numbers leading to the string
        // field within the request method corresponding to the
        // model name or id
        repeated uint32 idInjectionPath = 1;
    }

    // optional map of per-gRPC rpc method configuration
    // keys should be fully-qualified gRPC method name
    // (including package/service prefix)
    map<string,MethodInfo> methodInfos = 8;

    // EXPERIMENTAL - Set to true to enable the mode where
    // each loaded model reports a maximum inferencing
    // concurrency via the maxConcurrency field of
    // the LoadModelResponse message. Additional requests
    // are queued in the modelmesh framework. Turning this
    // on will also enable latency-based autoscaling for
    // the models, which attempts to minimize request
    // queueing time and requires no other configuration.
    bool limitModelConcurrency = 9;
}
```

### `loadModel`

```protobuf
message LoadModelRequest {
    string modelId = 1;

    string modelType = 2;
    string modelPath = 3;
    string modelKey = 4;
}
```

The runtime should load a model with name/id specified by the `modelId` field into memory ready for serving, from the path specified by the `modelPath` field. At this time, **the `modelType` field value should be ignored**.

The `modelKey` field will contain a JSON string with the following contents:

```json
{
  "model_type": {
    "name": "mytype",
    "version": "2"
  }
}
```

Where `model_type` corresponds to the `modelFormat` section from the originating [`InferenceSerivce` predictor](../predictors). Note that `version` is optional and may not be present. In future, additional attributes might be present in the outer json object so your implementation should ignore them gracefully.

The response shouldn't be returned until the model has loaded successfully and is ready to use.

```protobuf
message LoadModelResponse {
    // OPTIONAL - If nontrivial cost is involved in
    // determining the size, return 0 here and
    // do the sizing in the modelSize function
    uint64 sizeInBytes = 1;

    // EXPERIMENTAL - Applies only if limitModelConcurrency = true
    // was returned from runtimeStatus rpc.
    // See RuntimeStatusResponse.limitModelConcurrency for more detail
    uint32 maxConcurrency = 2;
}
```

### `unloadModel`

```protobuf
message UnloadModelRequest {
    string modelId = 1;
}
```

The runtime should unload the previously loaded (or failed) model specified by `modelId`, and not return a response until the unload is complete and corresponding resources have been freed. If the specified model is not found/loaded, the runtime should return immediately (without error).

```protobuf
message UnloadModelResponse {}
```

### Inferencing

The model runtime server can expose any number of protobuf-based gRPC services on the `grpcDataEndpoint` to use for inferencing requests. ModelMesh Serving is agnostic to specific service definitions (request/response message content), but for tensor-in/tensor-out based services it is recommended to conform to the [KServe V2 dataplane API spec](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#grpc).

A given model runtime server will be guaranteed to only receive model inferencing requests for models that had previously completed loading successfully (via a [`loadModel`](#loadmodel) request), and to have not been unloaded since.

Though generally agnostic to the specific API methods, ModelMesh Serving does need to be able to set/override the model name/id used in a given request. There are two options for obtaining the model name/id within the (which will correspond to the same id previously passed to `loadModel`):

1. Obtain from one of the `mm-model-id` or `mm-model-id-bin` gRPC metadata headers (latter required for non-ASCII UTF-8 ids). Precisely how this is done depends on the implementation language - see gRPC documentation for more information (_TODO_ specific refs/examples here).
2. Provide the location of a specific string field within your request protobuf message (per RPC method) which will be replaced with the target model id. This is done via the `methodInfos` map in the runtime's response to the [`runtimeStatus`](#runtimestatus) RPC method. Each applicable inferencing method should have an entry whose `idInjectionPath` field is set to a list of field numbers corresponding to the heirarchy of nested messages within the request message, the last of which being the number of the string field to replace. For example, if the id is a string field in the top-level request message with number 1 (as is the case in the KServe V2 [`ModelInferRequest`](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#inference-1)), this list would be set to just `[1]`.

Option 2 is particularly applicable when [integrating with an existing gRPC-based model server](#integrating-with-existing-model-servers).

## Deploying a Runtime

Each Serving Runtime implementation is defined using the custom resource type `ServingRuntime` which defines information about the runtime such as which container images need to be loaded, and the local gRPC endpoints on which they will listen. When the resource is applied to the Kubernetes cluster, the model server will deploy the runtime specific containers which will then enable support for the corresponding model types.

The following is an example of a `ServingRuntime` custom resource

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: example-runtime
spec:
  supportedModelFormats:
    - name: new-modelformat
      version: "1"
      autoSelect: true
  containers:
    - name: model-server
      image: samplemodelserver:latest
  multiModel: true
  grpcEndpoint: "port:8085"
  grpcDataEndpoint: "port:8090"
```

In each entry of the `supportedModelFormats` list, `autoSelect: true` can optionally be specified to indicate that that the given `ServingRuntime` can be considered for automatic placement of `InferenceService`s with the corresponding model type/format if no runtime is explicitly specified.
For example, if a user applies an `InferenceService` with `predictor.model.modelFormat.name: new-modelformat` and no `runtime` value, the above `ServingRuntime` will be used since it contains an "auto-selectable" supported model format that matches `new-modelformat`. If `autoSelect` were `false` or unspecified, the `InferenceService` would fail to load with the message "No `ServingRuntime` supports specified model type and/or protocol" unless the runtime `example-runtime` was specified directly in the YAML.

### Runtime container resource allocations

_TODO_ more detail coming here

## Integrating with existing model servers

The ability to specify multiple containers provides a nice way to integrate with existing model servers via an adapter pattern, as long as they provide the required capability of dynamically loading and unloading models.

![Custom with puller](../images/rt-custom-direct.png)

_Note: In the above diagram, only the adapter and model server containers are explicitly specified in the `ServingRuntime` CR, the others are included automatically._

The [built-in runtimes](https://github.com/kserve/modelmesh-serving/tree/main/config/runtimes) based on [Nvidia's Triton Inferencing Server](https://github.com/kserve/modelmesh-serving/blob/main/config/runtimes/triton-2.x.yaml) and the [Seldon MLServer](https://github.com/SeldonIO/MLServer), and their corresponding adapters serve as good examples of this and can be used as a reference.

## Reference

### Spec Attributes

Available attributes in the `ServingRuntime` spec:

| Attribute                          | Description                                                                                                                                                               |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `multiModel`                       | Whether this `ServingRuntime` is ModelMesh-compatible and intended for multi-model usage (as opposed to KServe single-model serving).                                     |
| `disabled`                         | Disables this runtime                                                                                                                                                     |
| `containers`                       | List of containers associated with the runtime                                                                                                                            |
| `containers[ ].image`              | The container image for the current container                                                                                                                             |
| `containers[ ].command`            | Executable command found in the provided image                                                                                                                            |
| `containers[ ].args`               | List of command line arguments as strings                                                                                                                                 |
| `containers[ ].resources`          | Kubernetes [limits or requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                       |
| `containers[ ].imagePullPolicy`    | The container image pull policy                                                                                                                                           |
| `containers[ ].workingDir`         | The working directory for current container                                                                                                                               |
| `grpcEndpoint`                     | The [port](#endpoint-formats) for model management requests                                                                                                               |
| `grpcDataEndpoint`                 | The [port or unix socket](#endpoint-formats) for inferencing requests arriving to the model server over the gRPC protocol. May be set to the same value as `grpcEndpoint` |
| `supportedModelFormats`            | List of model types supported by the current runtime                                                                                                                      |
| `supportedModelFormats[ ].name`    | Name of the model type                                                                                                                                                    |
| `supportedModelFormats[ ].version` | Version of the model type. It is recommended to include only the major version here, for example "1" rather than "1.15.4"                                                 |
| `storageHelper.disabled`           | Disables the storage helper                                                                                                                                               |
| `nodeSelector`                     | Influence Kubernetes scheduling to [assign pods to nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/)                                       |
| `affinity`                         | Influence Kubernetes scheduling to [assign pods to nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)            |
| `tolerations`                      | Allow pods to be scheduled onto nodes [with matching taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration)                                |
| `replicas`                         | The number of replicas of the runtime to create. This overrides the `podsPerRuntime` [configuration](/docs/configuration/README.md)                                       |

### Endpoint formats

Several of the attributes (`grpcEndpoint`, `grpcDataEndpoint`) support either Unix Domain Sockets or TCP. The endpoint should be formatted as either `port:<number>` or `unix:<path>`. The provided container must be either listening on the specific TCP socket or at the provided path.

---

**Warning**

If a unix domain socket is specified for both `grpcEndpoint` and `grpcDataEndpoint` then it must either be the same socket (identical path) or reside in the same directory.

---

### Full Example

The following example demonstrates all of the possible attributes that can be set in the model serving runtime spec:

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: example-runtime
spec:
  supportedModelFormats:
    - name: my_model_format # name of the model
      version: "1"
      autoSelect: true
  containers:
    - args:
        - arg1
        - arg2
      command:
        - command
        - command2
      env:
        - name: name
          value: value
        - name: fromSecret
          valueFrom:
            secretKeyRef:
              key: mykey
      image: image
      name: name
      resources:
        limits:
          memory: 200Mi
      imagePullPolicy: IfNotPresent
      workingDir: "/container/working/dir"
  multiModel: true
  disabled: false
  storageHelper:
    disabled: true
  grpcEndpoint: port:1234 # or unix:/path
  grpcDataEndpoint: port:1234 # or unix:/path
  # To influence pod scheduling, one or more of the following can be used
  nodeSelector: # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
    kubernetes.io/arch: "amd64"
  affinity: # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: "kubernetes.io/arch"
                operator: In
                values:
                  - "amd64"
  tolerations: # https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
    - key: "example-key"
      operator: "Exists"
      effect: "NoSchedule"
```

### Storage Helper

Storage helper will download the model from the S3 bucket using the secret `storage-config` and place it in the local path. By default, storage helper is enabled in the serving runtime. Storage helper can be disabled by adding the configuration `storageHelper.disabled` set to `true` in serving runtime. If the storage helper is disabled, the custom runtime needs to handle access to and pulling model data from storage itself. Configuration can be passed to the runtime's pods through environment variables.

#### Example

Consider the custom runtime defined [above](#full-example) with the following `InferenceService`:

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: my-mnist-isvc
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: my_model_format
        version: "1"
      storage:
        key: my_storage
        path: my_models/mnist-model
        parameters:
          bucket: my_bucket
```

If the storage helper is enabled, the model serving container will receive the below model metadata in the `loadModel` call where `modelPath` will contain the path of the model in the local file system.

```json
{
  "modelId": "my-mnist-isvc-<suffix>",
  "modelType": "my_model_format",
  "modelPath": "/models/my-mnist-isvc-<suffix>/",
  "modelKey": "<serialized metadata as JSON, see below>"
}
```

The following metadata for the `InferenceService` predictor is serialized to a string and embedded as the `modelKey` field:

```json
{
  "bucket": "my_bucket",
  "disk_size_bytes": 2415,
  "model_type": {
    "name": "my_model_format",
    "version": "1"
  },
  "storage_key": "my_storage"
}
```

If the storage helper is disabled, the model serving container will receive the below model metadata in the `loadModel` call where `modelPath` is same as the `path` provided in the predictor storage spec.

```json
{
  "modelId": "my-mnist-isvc-<suffix>",
  "modelType": "my_model_format",
  "modelPath": "my_models/mnist-model",
  "modelKey": "<serialized metadata as JSON, see below>"
}
```

The following metadata for the `InferenceService` predictor is serialized to a string and embedded as the `modelKey` field:

```json
{
  "bucket": "my_bucket",
  "model_type": {
    "name": "my_model_format",
    "version": "1"
  },
  "storage_key": "my_storage"
}
```
