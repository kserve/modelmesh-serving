# OpenVINO IR

## Format

Full documentation on OpenVINO IR format can be found [here](https://docs.openvino.ai/2022.1/openvino_docs_MO_DG_IR_and_opsets.html#intermediate-representation-used-in-openvino).

OpenVINO™ toolkit introduces its own format of graph representation and its own operation set. A graph is represented with two files: an XML file and a binary file. This representation is commonly referred to as the Intermediate Representation or IR.

An example of a small IR XML file can be found in the same [link above](https://docs.openvino.ai/2022.1/openvino_docs_MO_DG_IR_and_opsets.html#intermediate-representation-used-in-openvino). The XML file doesn’t have big constant values, like convolution weights. Instead, it refers to a part of the accompanying binary file that stores such values in a binary format.

Models trained in other formats (Caffe, TensorFlow, MXNet, PaddlePaddle and ONNX) can be converted to OpenVINO IR format. To do so, use OpenVINO’s [Model Optimizer](https://docs.openvino.ai/2022.1/openvino_docs_MO_DG_Deep_Learning_Model_Optimizer_DevGuide.html).

## Configuration

Each model defines input and output tensors in the AI graph. The client passes data to model input tensors by filling appropriate entries in the request input map. Prediction results can be read from the response output map. By default, OpenVINO™ Model Server uses model tensor names as input and output names in prediction requests and responses. The client passes the input values to the request and reads the results by referring to the corresponding output names.

Here is an example of client code:

```python
input_tensorname = 'input'
request.inputs[input_tensorname].CopyFrom(make_tensor_proto(img, shape=(1, 3, 224, 224)))

.....

output_tensorname = 'resnet_v1_50/predictions/Reshape_1'
predictions = make_ndarray(result.outputs[output_tensorname])
```

It is possible to adjust this behavior by adding an optional .json file named `mapping_config.json`. It can map the input and output keys to the appropriate tensors. This extra mapping can be used to enable user-friendly names for models with difficult tensor names. Here is an example of `mapping_config.json`:

```json
{
  "inputs": {
    "tensor_name": "grpc_custom_input_name"
  },
  "outputs": {
    "tensor_name1": "grpc_output_key_name1",
    "tensor_name2": "grpc_output_key_name2"
  }
}
```

More details on model configuration can be found [here](https://docs.openvino.ai/latest/ovms_docs_models_repository.html#doxid-ovms-docs-models-repository).

## Storage Layout

The OpenVINO models need to be placed and mounted in a particular directory structure:

```
tree models/
models/
├── model1
│   ├── 1
│   │   ├── ir_model.bin
│   │   └── ir_model.xml
│   └── 2
│       ├── ir_model.bin
│       └── ir_model.xml
└── model2
│   └── 1
│       ├── ir_model.bin
│       ├── ir_model.xml
│       └── mapping_config.json
└── model3
    └── 1
        └── model.onnx
```

and according to the following rules:

- Each model should be stored in a dedicated directory, e.g. model1 and model2.
- Each model directory should include a sub-folder for each of its versions (1,2, etc). The versions and their folder names should be positive integer values.
  **Note**: In execution, the versions are enabled according to a pre-defined version policy. If the client does not specify the version number in parameters, by default, the latest version is served.
- Every version folder must include model files, that is, .bin and .xml. The file name can be arbitrary.

## Example

**Storage Layout**

```
s3://modelmesh-serving-examples/
  openvino/mnist
```

**InferenceService**

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: openvino-ir-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: openvino_ir
      storage:
        key: modelStorage
        path: openvino/mnist
        parameters:
          bucket: modelmesh-serving-examples
```
