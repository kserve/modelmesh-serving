# XGBoost

## Format

XGBoost models must be serialised using the
[booster.save_model() method](https://xgboost.readthedocs.io/en/latest/tutorials/saving_model.html).
It can be serialized as JSON or in the binary `.bst` format.

## Configuration

The inputs and outputs to the model can be inferred from the model data. The
[model schema](../predictors/schema.md)
is not required.

For some advanced use-cases, it may be necessary to include runtime specific
configuration with the model. If the model schema and inferred configuration are
not sufficient, refer to the runtime specific
[options for advanced configuration](advanced-configuration.md#mlserver).

## Storage Layout

**Simple**

The storage path can point directly to a serialized model

```
<storage-path/model-name.json>
```

## Example

**Storage Layout**

```
s3://modelmesh-example-models/
└── xgboost/mushroom.json
```

**InferenceService**

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: xgboost-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: xgboost
      storage:
        key: localMinIO
        path: xgboost/mushroom.json
        parameters:
          bucket: modelmesh-example-models
```
