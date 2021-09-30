# LightGBM

## Format

LightGBM models must be serialised using the
[Booster.save_model() method](https://lightgbm.readthedocs.io/en/latest/pythonapi/lightgbm.Booster.html#lightgbm.Booster.save_model).

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
<storage-path/model-name.bst>
```

## Example

**Storage Layout**

```
s3://modelmesh-serving-examples/
└── lightgbm-models/example.bst
```

**Predictor**

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: lightgbm-example
spec:
  modelType:
    name: lightgbm
  path: lightgbm-models/example.bst
  storage:
    s3:
      secretKey: modelStorage
      bucket: modelmesh-serving-examples
```
