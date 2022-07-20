# scikit-learn

## Format

Scikit-learn model serialized using [joblib.dump](https://joblib.readthedocs.io/en/latest/generated/joblib.dump.html).
See the [model persistence](https://scikit-learn.org/stable/modules/model_persistence.html)
Scikit-learn documentation for details.

Scikit-learn models serialized using "pickle" library and "joblib" library are supported.

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

The storage path can point directly to an Sklearn model serialized using
`joblib`.

```
<storage-path/model.joblib>
```

The file does not need to be called `model.joblib`, it can have any name.

**Directory**

The storage path can point to a directory containing a single file that is
the Sklearn model serialized using `joblib`.

```
<storage-path>/
└── model.joblib
```

The file does not need to be called `model.joblib`, it can have any name.

**Explicit Configuration**

If the `model-settings.json` configuration file is provided, it must be in
the directory pointed to by the `InferenceService`'s storage path. The model files must also
be contained under this path.

```
<storage-path>/
├── model-settings.json
└── <model-files>
```

## Example

**Storage Layout**

```
s3://modelmesh-example-models/
└── sklearn/mnist-svm.joblib
```

**InferenceService**

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: sklearn-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: sklearn
      storage:
        key: localMinIO
        path: sklearn/mnist-svm.joblib
        parameters:
          bucket: modelmesh-example-models
```
