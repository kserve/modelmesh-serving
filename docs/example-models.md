# Sample Models

If ModelMesh Serving was deployed using `--quickstart`, a set of example models are shared via a MinIO instance to use when getting started with ModelMesh Serving and experimenting with the provided runtimes.

## Predictors

The `config/example-predictors` directory contains Predictor manifests for many of the example models. Assuming that the entry specified below is added to the storage configuration secret, the Predictors can be deployed and used for experimentation.

## Naming Conventions

Models are organized into virtual directories based on model type:

```
s3://modelmesh-example-models/
└── <model-type>/
    ├── <model-name-file>
    └── <model-name>/
        └── <model-data>
```

## Available Models

### Virtual Object Tree

```
s3://modelmesh-example-models/
├── keras
│   └── mnist.h5
├── lightgbm
│   └── mushroom.bst
├── onnx
│   └── mnist.onnx
├── pytorch
│   └── cifar
│       ├── 1
│       │   └── model.pt
│       └── config.pbtxt
├── sklearn
│   └── mnist-svm.joblib
├── tensorflow
│   └── mnist.savedmodel
│       ├── saved_model.pb
│       └── variables
│           ├── variables.data-00000-of-00001
│           └── variables.index
└── xgboost
    └── mushroom.json
```
