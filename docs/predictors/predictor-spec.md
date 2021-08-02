## Predictor Spec

Here is a complete example of a Predictor spec:

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
  storage:
    s3:
      secretKey: my_storage
      bucket: my_bucket
```

Notes:

- `runtime` is optional. If included, the model will be loaded/served using the `ServingRuntime` with the specified name, and the predictors `modelType` must match an entry in that runtime's `supportedModels` list (see [runtimes](../runtimes.md))
- The CRD contains additional fields but they have been omitted here for now since they are not yet fully supported
