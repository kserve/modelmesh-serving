# Supported Model Formats

By leveraging existing third-party model servers, we support a number of standard ML model formats out-of-the box, with more to follow. Currently supported model types:

- [LightGBM](lightgbm.md)
- [ONNX](onnx.md)
- [PyTorch ScriptModule](pytorch.md)
- [scikit-learn](sklearn.md)
- [TensorFlow](tensorflow.md)
- [XGBoost](xgboost.md)

| Model Type | Framework    | Versions        | Supported via ServingRuntime |
| ---------- | ------------ | --------------- | ---------------------------- |
| lightgbm   | LightGBM     | 3.2.1           | MLServer (python)            |
| onnx       | ONNX         | 1.5.3           | Triton (C++)                 |
| pytorch    | PyTorch      | 1.8.0a0+1606899 | Triton (C++)                 |
| sklearn    | scikit-learn | 0.23.1          | MLServer (python)            |
| tensorflow | TensorFlow   | 1.15.4, 2.3.1   | Triton (C++)                 |
| xgboost    | XGBoost      | 1.1.1           | MLServer (python)            |
| \*         | Custom       |                 | [Custom](../runtimes) (any)  |
