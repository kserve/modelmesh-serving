# Tensorflow

## Format

Both v1 and v2 TensorFlow models are supported as either a SavedModel or a
serialized `GraphDef`. The SavedModel format is the default for TF2.x and is
preferred because it includes the model weights and configuration. Refer to
the TensorFlow documentation on
["Using the SavedModel format"](https://www.tensorflow.org/guide/saved_model)
or the [specification of the `GraphDef` protocol buffer message](https://www.tensorflow.org/api_docs/python/tf/compat/v1/GraphDef)
for details on these formats and their serialization.

## Configuration

**SavedModel Format**

The inputs and outputs to the model can be inferred from the model data. The
[model schema](../predictors/schema.md)
is not required.

For some advanced use-cases, it may be necessary to include runtime specific
configuration with the model. If the model schema and inferred configuration are
not sufficient, refer to the runtime specific
[options for advanced configuration](advanced-configuration.md#triton-server).

**`GraphDef` Format**

The schema for the inputs and outputs to the model must be specified in a
[model schema file](../predictors/schema.md).

For some advanced use-cases, it may be necessary to include runtime specific
configuration with the model. If the model schema and inferred configuration are
not sufficient, refer to the runtime specific
[options for advanced configuration](advanced-configuration.md#triton-server).

## Storage Layout - SavedModel

If the optional configuration file is not needed, there are multiple
supported storage layouts as referenced below.

**Simple**

The storage path can point directly to the contents of the `SavedModel`:

```
<storage-path>/
└── <saved-model-files>
```

**Directory**

The storage path can also point to a directory containing the `SavedModel` directory:

```
<storage-path>/
└── model.savedmodel/
    └── <saved-model-files>
```

The directory does not have to be called `model.savedmodel`.

## Storage Layout - GraphDef

The configuration file is required for a `GraphDef` model, so the repository
structure options are more limited.

**Simple**

The storage path points to a directory containing the serialized `GraphDef`
in a file called `model.graphdef` and the model schema file exists in the same storage:

```
<storage-path>/
└── model.graphdef
<schema-file.json>
```

The schema file can be in the `storage-path` directory or in another location.

## Example

The following example is using the `SavedModel` format with the simple
repository layout.

**Storage Layout**

```
s3://modelmesh-example-models/
└──tensorflow/mnist.savedmodel/
   ├── variables/
   └── saved_model.pb
```

**InferenceService**

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: tensorflow-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: tensorflow
      storage:
        key: localMinIO
        path: tensorflow/mnist.savedmodel
        parameters:
          bucket: modelmesh-example-models
```
