# ONNX

## Format

ONNX is an open format built to represent machine learning models. ONNX defines a common set of operators - the building blocks of machine learning and deep learning models - and a common file format to enable AI developers to use models with a variety of frameworks, tools, runtimes, and compilers.

ONNX defines a common file format that abstracts the building blocks of machine
learning and deep learning models. It is possible to convert models trained from
many different frameworks/tools to the ONNX format. See the
[ONNX tutorial documentation](https://github.com/onnx/tutorials#converting-to-onnx-format)
for some examples.

## Configuration

The inputs and outputs to the model can be inferred from the model data. The
[model schema](../predictors/schema.md)
is not required.

For some advanced use-cases, it may be necessary to include runtime specific
configuration with the model. If the model schema and inferred configuration are
not sufficient, refer to the runtime specific
[options for advanced configuration](advanced-configuration.md#triton-server).

## Storage Layout

ONNX models may consist of a single file or a directory, both are supported.

**Simple**

```
<storage-path/model-name>
```

## Example Predictor

**Storage Layout**

```
s3://modelmesh-serving-examples/
  onnx-models/example.onnx
```

**Predictor**

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: onnx-example
spec:
  modelType:
    name: onnx
  path: onnx-models/example.onnx
  storage:
    s3:
      secretKey: modelStorage
      bucket: modelmesh-serving-examples
```
