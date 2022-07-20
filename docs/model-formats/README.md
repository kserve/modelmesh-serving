# Supported Model Formats

By leveraging existing third-party model servers, we support a number of standard ML model formats out-of-the box, with more to follow. Currently supported model formats:

- [Keras](keras.md)
- [LightGBM](lightgbm.md)
- [ONNX](onnx.md)
- [OpenVINO IR](openvino-ir.md)
- [PyTorch ScriptModule](pytorch.md)
- [scikit-learn](sklearn.md)
- [TensorFlow](tensorflow.md)
- [XGBoost](xgboost.md)

| Model Type  | Framework        | Supported via ServingRuntime |
| ----------- | ---------------- | ---------------------------- |
| keras       | TensorFlow       | Triton (C++)                 |
| lightgbm    | LightGBM         | MLServer (python)            |
| onnx        | ONNX             | Triton (C++), OVMS (C++)     |
| openvino_ir | Intel OpenVINO\* | OVMS (C++)                   |
| pytorch     | PyTorch          | Triton (C++)                 |
| sklearn     | scikit-learn     | MLServer (python)            |
| tensorflow  | TensorFlow       | Triton (C++)                 |
| xgboost     | XGBoost          | MLServer (python)            |
| any         | Custom           | [Custom](../runtimes) (any)  |

(\*)Many ML frameworks can have models converted to the OpenVINO IR format, such as Caffe, TensorFlow, MXNet, PaddlePaddle and ONNX, doc [here](https://docs.openvino.ai/latest/ovms_what_is_openvino_model_server.html).
