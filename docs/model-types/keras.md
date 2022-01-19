# Keras

## Format

Keras HDF5 models are supported via automatic conversion into TensorFlow SavedModels.

Refer to the TensorFlow documentation on
["Using the SavedModel format"](https://www.tensorflow.org/guide/saved_model).

## Configuration

The model data for Keras can be specified with a direct-to-file path.

The inputs and outputs to the model can be inferred from the model data. The
[model schema](../predictors/schema.md) is not required.

## Example Predictor

The following example is using the HDF5 file format, which typically uses a `.h5` extension.

**Storage Layout**

```
s3://modelmesh-serving-examples/
└── keras-models/mnist.h5
```

**Predictor**

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: keras-example
spec:
  modelType:
    name: keras
  path: keras-models/mnist.h5
  storage:
    s3:
      secretKey: modelStorage
      bucket: modelmesh-serving-examples
```
