# Predictor Spec

> **:exclamation: Important** Please use the [InferenceService CR](./inferenceservice-cr.md) for deploying on ModelMesh.

Here is a complete example of a `Predictor` spec:

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: my-mnist-predictor
spec:
  modelType:
    name: tensorflow
    version: "1.15" # Optional
  runtime: # Optional
    name: triton-2.x
  path: my_models/mnist-tf
  schemaPath: my_schemas/mnist-schema.json # Optional
  storage:
    s3:
      secretKey: my_storage
      bucket: my_bucket # Optional
```

---

**Note**

- `runtime` is optional. If included, the model will be loaded/served using the `ServingRuntime` with the specified name, and the predictors `modelType` must match an entry in that runtime's `supportedModels` list (see [runtimes](../runtimes/))
- The CRD contains additional fields but they have been omitted here for now since they are not yet fully supported

---

## Predictor Status

The Status section of the `Predictor` custom resource reflects details about its current state and comprises the following fields.

`grpcEndpoint` - The gRPC target endpoint (host/port) corresponding to this predictor. Note that this will currently be the same for all predictors owned by a given ModelMesh Serving installation.

`httpEndpoint` - The HTTP (REST) endpoint corresponding to this predictor. This endpoint is provided through a REST proxy sidecar (if enabled), and this will also be the same for all predictors owned by a given ModelMesh Serving installation.

`available` - A boolean value indicating whether the predictor's endpoints are ready to serve inferencing requests. Note that this does not _necessarily_ mean requests will respond immediately, the corresponding model may or may not be loaded in memory. In the case that it isn't there may be some delay before the response comes back.

`activeModelState` - The state of the model currently being served by the predictor's endpoints. It may be one of:

- `Pending` - The ModelMesh Serving controller has not yet acknowledged/registered this (new) predictor/model.
- `Standby` - The model is currently not loaded in memory anywhere, but will be automatically upon first usage. This means the first requests to this predictor will likely take longer to respond.
- `Loading` - The model is in the process of loading. Requests may take longer to respond since they will be blocked until the loading completes.
- `Loaded` - The model is loaded in at least one pod and ready to respond immediately to inferencing requests.
- `FailedToLoad` - The model could not be loaded for some reason. See the `lastFailureInfo` field for more details.

`targetModelState` - This will be set only when `transitionStatus` is not `UpToDate`, meaning that the target model differs from the currently-active model. The target model always corresponds to the predictor's current [spec](#predictor-spec). The possible values are the same as `activeModelState` but should generally only be either `Loading` or `FailedToLoad`.

`transitionStatus` - Indicates state of the predictor relative to its current spec. It may be one of:

- `UpToDate` - The predictor's current model reflects its spec, that is, its active model matches its target model.
- `InProgress` - The predictor's currently active model configuration is older than its current spec reflects. This is usually the case immediately after the spec changes, while a new target model is loading (`targetModelState` should be `Loading`). Once the target model finishes loading successfully, the active model will become the target model and the `transitionStatus` will return to `UpToDate`.
- `BlockedByFailedLoad` - The predictor's currently active model configuration is older than its current spec reflects, because there was a problem loading the corresponding model. See the `lastFailureInfo` field for more details of the failure.
- `InvalidSpec` - The predictor's currently active model configuration was not transitioned to match its current spec because the current spec is invalid. There may be more details of the error in the `lastFailureInfo` field.

`failedCopies` - The number of copies of the active or target model that failed to load recently (there will be at most one of each per pod)

`lastFailureInfo` - Details about the most recent error associated with this predictor. Not all of the contained fields will necessarily have a value.

- `reason` - A high level code indicating the nature of the failure, may be one of:
  - `ModelLoadFailed` - The model failed to load within a serving runtime container. Loading is automatically retried in other pods of the same runtime if they exist, the `failedCopies` field indicates how many different pods the model _recently_ failed to load in.
  - `RuntimeUnhealthy` - Corresponding `ServingRuntime` pods failed to start or are unhealthy.
  - `NoSupportingRuntime` - There are no `ServingRuntime`s which support the specified model type.
  - `RuntimeNotRecognized` - There is no `ServingRuntime` defined with the specified runtime name.
  - `InvalidPredictorSpec` - The current `Predictor` spec is invalid or unsupported.
- `location` - Indication of the pod in which a loading failure most recently occurred, if applicable. Its value will be the last 12 digits of the pod's full name.
- `message` - A message containing more detail about the error/failure.
- `modelId` - The internal id of the model in question. This includes a hash of the `Predictor`'s spec.
- `time` - The time at which the failure occurred, if applicable.

Upon creation, Predictors will always transition to `Loaded` state (unless the loading fails), but later if unused it is possible that they end up in a `Standby` state which means they are still available to serve requests but the first request could incur a loading delay. Whether this happens is a function of the available capacity and usage pattern of other models. It's possible that models will transition from `Standby` back to `Loaded` "by themselves" if more capacity becomes available.

Model loading will be retried immediately in other pods if it fails, after which it will be re-attempted periodically (every ten minutes or so).
