# Keras

## Format

Keras HDF5 models are supported via automatic conversion into TensorFlow SavedModels.

Refer to the TensorFlow documentation on
["Using the SavedModel format"](https://www.tensorflow.org/guide/saved_model).

## Configuration

The model data for Keras can be specified with a direct-to-file path.

The inputs and outputs to the model can be inferred from the model data. The
[model schema](../predictors/schema.md) is not required.

## Example

The following example is using the HDF5 file format, which typically uses a `.h5` extension.

**Storage Layout**

```
s3://modelmesh-example-models/
└── keras/mnist.h5
```

**InferenceService**

```yaml
kind: InferenceService
metadata:
  name: keras-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: keras
      storage:
        key: localMinIO
        path: keras/mnist.h5
        parameters:
          bucket: modelmesh-example-models
```
