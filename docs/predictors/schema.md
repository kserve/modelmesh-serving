# Model Schema (alpha)

The input and output schema of ML models can be provided via the `Predictor` CR along with the model files themselves. This must be a JSON file in the standard format described below, which currently must reside in the same storage instance as the corresponding model.

---

**Warning**

The generic model schema should be considered alpha. Breaking changes to how the schema is used are expected. Do not rely on this schema in production.

---

### Schema Format

The JSON for schema should be in **_KFS V2 format_**, fields are mapped to tensors.

```json
{
        "inputs": [{
                "name": "Tensor name",
                "datatype": "Tensor data type",
                "shape": [Dimension of the tensor]
        }],
        "outputs": [{
                "name": "Tensor name",
                "datatype": "Tensor data type",
                "shape": [Dimension of the tensor]
        }]
}
```

Refer to the [KServe V2 Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#tensor-data) docs for tensor data representation and supported tensor data types.

The `shape` parameter should match the number and size of the dimensions of the tensors for the served `Predictor`. A size of `-1` indicates a dimension with a variable length. Models trained with mini-batches may expect the batch dimension when served, typically this is the first dimension. If there is a batch dimension, it must be included in the shape of all inputs and outputs.

### Sample schema

This is a sample schema for an MNIST model that includes a batch dimension. The input is a batch of 28x28 images of 32-bit floating point numbers of the handwritten digits. The model's output is a batch of 10-element vectors, one per input image, with probabilities of the image being each digit 0 to 9.

```json
{
  "inputs": [
    {
      "name": "input",
      "datatype": "FP32",
      "shape": [-1, 28, 28]
    }
  ],
  "outputs": [
    {
      "name": "output",
      "datatype": "FP32",
      "shape": [-1, 10]
    }
  ]
}
```

The `schemaPath` field of the `Predictor` custom resource should be set to point to this JSON file within the predictor's specified storage instance.

#### Example `Predictor` CR with provided schema

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: tensorflow-pbnoschema
spec:
  modelType:
    name: tensorflow
  path: tfmnist-pb-noschema
  schemaPath: schema/tf-schema.json
  storage:
    s3:
      secretKey: myStorage
      bucket: modelmesh-serving-schema
```

Note that this field is optional. Not all model types require a schema to be provided - for example when the model serialization format incorporates equivalent schema information or it is otherwise not required by the corresponding runtime. In some cases the schema isn't required but will be used for additional payload validation when it is.
