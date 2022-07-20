# PyTorch (TorchScript)

## Format

The serving runtime uses the C++ distribution of PyTorch called
[`LibTorch`](https://pytorch.org/cppdocs/installing.html)
to support high performance inference. This library requires that models be
serialized as a `ScriptModule` composed with
[`TorchScript`](https://pytorch.org/cppdocs/#torchscript).
Refer to PyTorch's documentation on
[loading a `TorchScript` model in C++](https://pytorch.org/tutorials/advanced/cpp_export.html)
for details on converting a PyTorch model in Python to an exported
`ScriptModule`.

## Configuration

The schema for the inputs and outputs to the model must be specified in a
[model schema file](../predictors/schema.md).

For some advanced use-cases, it may be necessary to include runtime specific
configuration with the model. If the model schema and inferred configuration are
not sufficient, refer to the runtime specific
[options for advanced configuration](advanced-configuration.md#triton-server).

## Storage Layout

**Simple**

The storage path points to a directory containing the serialized `ScriptModule`
in a file called `model.pt` and the model schema file exists in the same storage:

```
<storage-path>/
└── model.pt
<schema-file.json>
```

The schema file can be in the `storage-path` directory or in another location.

## Example

**Storage Layout**

```
s3://modelmesh-serving-examples/
└── pytorch-model/
    ├── model.pt
    └── schema.json
```

**InferenceService**

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: pytorch-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: pytorch
      storage:
        key: modelStorage
        path: pytorch-model
        schemaPath: pytorch-model/schema.json
        parameters:
          bucket: modelmesh-serving-examples
```
