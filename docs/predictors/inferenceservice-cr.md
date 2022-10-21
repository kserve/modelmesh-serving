# InferenceService Spec

The KServe `InferenceService` CRD is the primary user interface that ModelMesh Serving uses for deploying models. An `InferenceService` is comprised of three components: a Predictor, a Transformer, and an Explainer. Currently, ModelMesh Serving primarily only supports the Predictor component for deploying models. There is preliminary support for Transformers, however, transformer deployment is handled by the KServe controller. As such, each `InferenceService`'s transformer will require its own pod.

Here is an example of an `InferenceService` spec containing fields that would typically be used with ModelMesh:

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: my-tensorflow-predictor
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: tensorflow
        version: "1.15" # Optional
      runtime: triton-2.x #Optional
      storage:
        key: my_storage
        path: my_models/mnist-tf
        schemaPath: my_schemas/mnist-schema.json # Optional
        parameters:
          bucket: my_bucket # Optional if bucket specified in secret
```

**Note**

- While both the KServe controller and ModelMesh controller will reconcile `InferenceService` resources, the ModelMesh controller will
  only handle those `InferenceServices` with the `serving.kserve.io/deploymentMode: ModelMesh` annotation. Otherwise, the KServe controller will
  handle reconciliation. Likewise, the KServe controller will not reconcile an `InferenceService` with the `serving.kserve.io/deploymentMode: ModelMesh`
  annotation, and will defer under the assumption that the ModelMesh controller will handle it.
- `runtime` is optional. If included, the model will be loaded/served using the `ServingRuntime` with the specified name, and the predictors `modelFormat` must match an entry
  in that runtime's `supportedModelFormats` list (see [runtimes](../runtimes/)).
- The above spec makes use the `InferenceService` predictor [storage spec interface](https://github.com/kserve/kserve/tree/master/docs/samples/storage/storageSpec) for passing
  in storage related information.

Users can alternatively continue to use `storageUri` to pass in storage information:

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: my-tensorflow-predictor
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
    serving.kserve.io/secretKey: my_storage
    serving.kserve.io/schemaPath: my_schemas/mnist-schema.json
spec:
  predictor:
    model:
      modelFormat:
        name: tensorflow
        version: "1.15"
      runtime: triton-2.x
      storageUri: s3://my_bucket/my_models/mnist-tf
```

When using the `storageUri` field instead of the storage spec, additional information can be passed in as annotations:

- `serving.kserve.io/secretKey` for specifying storage secret key (if needed).
- `serving.kserve.io/schemaPath`: The path within the object storage of a schema file. This allows specifying the input and output schema of ML models.
  - For example, if your model `storageURI` was `s3://modelmesh-example-models/pytorch/pytorch-cifar` the schema file would currently need to be in the
    same bucket (`modelmesh-example-models`). The path within this bucket is what would be specified in this annotation (e.g. `pytorch/schema/schema.json`)

**InferenceService Status**

The Status section of the `InferenceService` custom resource reflects details about its current state. Here are fields relevant to ModelMesh:

`components.predictor` - predictor related endpoint information.

- `url` - URL holds the primary url that will distribute traffic over the provided traffic targets. This will be one the REST or gRPC endpoints that are available.
- `restUrl` - REST endpoint of the component if available. This endpoint is provided through a REST proxy sidecar (if enabled), and this will also be the same for all predictors owned by a given ModelMesh Serving installation.
- `grpcUrl` - gRPC endpoint of the component if available. Note that this will currently be the same for all `InferenceService` owned by a given ModelMesh Serving installation.

`conditions` - Various condition entries. Pertinent entries are:

- `PredictorReady`: predictor readiness condition. Status is `true` when the predictor's endpoints are ready to serve inferencing requests. Note that this does not _necessarily_ mean requests will respond immediately, the corresponding model may or may not be loaded in memory. In the case that it isn't there may be some delay before the response comes back.
- `Ready`: aggregated condition of all conditions.

`modelStatus` - Model related statuses.

- `states` - State information of the predictor's model.

  - `activeModelState` - The state of the model currently being served by the predictor's endpoints. It may be one of:

    - `Pending` - The ModelMesh Serving controller has not yet acknowledged/registered this (new) predictor/model.
    - `Standby` - The model is currently not loaded in memory anywhere, but will be automatically upon first usage. This means the first requests to this predictor will likely take longer to respond.
    - `Loading` - The model is in the process of loading. Requests may take longer to respond since they will be blocked until the loading completes.
    - `Loaded` - The model is loaded in at least one pod and ready to respond immediately to inferencing requests.
    - `FailedToLoad` - The model could not be loaded for some reason. See the `lastFailureInfo` field for more details.

  - `targetModelState` - This will be set only when `transitionStatus` is not `UpToDate`, meaning that the target model differs from the currently-active model. The target model always corresponds to the `InferenceService` predictor's current [spec](#inferenceservice-spec). The possible values are the same as `activeModelState` but should generally only be either `Loading` or `FailedToLoad`.

- `transitionStatus` - Indicates state of the predictor relative to its current spec. It may be one of:

  - `UpToDate` - The predictor's current model reflects its spec, that is, its active model matches its target model.
  - `InProgress` - The predictor's currently active model configuration is older than its current spec reflects. This is usually the case immediately after the spec changes, while a new target model is loading (`targetModelState` should be `Loading`). Once the target model finishes loading successfully, the active model will become the target model and the `transitionStatus` will return to `UpToDate`.
  - `BlockedByFailedLoad` - The predictor's currently active model configuration is older than its current spec reflects, because there was a problem loading the corresponding model. See the `lastFailureInfo` field for more details of the failure.
  - `InvalidSpec` - The predictor's currently active model configuration was not transitioned to match its current spec because the current spec is invalid. There may be more details of the error in the `lastFailureInfo` field.

- `modelCopies` - Model copy information of the predictor's model.

  - `failedCopies` - The number of copies of the active or target model that failed to load recently (there will be at most one of each per pod).
  - `totalCopies` - The total number of copies of this predictor's models that are currently loaded.

- `lastFailureInfo` - Details about the most recent error associated with this predictor. Not all of the contained fields will necessarily have a value.

  - `reason` - A high level code indicating the nature of the failure, may be one of:
    - `ModelLoadFailed` - The model failed to load within a serving runtime container. Loading is automatically retried in other pods of the same runtime if they exist, the `failedCopies` field indicates how many different pods the model _recently_ failed to load in.
    - `RuntimeUnhealthy` - Corresponding `ServingRuntime` pods failed to start or are unhealthy.
    - `NoSupportingRuntime` - There are no `ServingRuntime`s which support the specified model type.
    - `RuntimeNotRecognized` - There is no `ServingRuntime` defined with the specified runtime name.
    - `InvalidPredictorSpec` - The current `InferenceService` predictor spec is invalid or unsupported.
  - `location` - Indication of the pod in which a loading failure most recently occurred, if applicable. Its value will be the last 12 digits of the pod's full name.
  - `message` - A message containing more detail about the error/failure.
  - `modelId` - The internal id of the model in question. This includes a hash of the `InferenceService`'s predictor spec.
  - `time` - The time at which the failure occurred, if applicable.

Upon creation, the active model status of an `InferenceService` will always transition to `Loaded` state (unless the loading fails), but later if unused, it is possible that the active model status ends up in a `Standby` state which means the model is still available to serve requests but the first request could incur a loading delay. Whether this happens is a function of the available capacity and usage pattern of other models. It's possible that models will transition from `Standby` back to `Loaded` "by themselves" if more capacity becomes available.

Model loading will be retried immediately in other pods if it fails, after which it will be re-attempted periodically (every ten minutes or so).
