# Using ModelMesh Serving

Trained models are deployed in ModelMesh Serving via `Predictor`s. These represent a stable service endpoint behind which the underlying model can change.

Models must reside on shared storage. Currently, only S3-based storage is supported but support for other types will follow. Note that model data residing at a particular path within a given storage instance is **assumed to be immutable**. Different versions of the same logical model are treated at the base level as independent models and must reside at different paths. In particular, where a given model server/runtime natively supports the notion of versioning (such as Nvidia Triton, TensorFlow Serving, etc), the provided path should not point to the top of a (pseudo-)directory structure containing multiple versions. Instead, point to the subdirectory which corresponds to a specific version.

## Deploying a scikit-learn model

### Prerequisites

The ModelMesh Serving instance should be installed in the desired namespace. See [install docs](../install/install-script.md) for more details.

### Deploy a sample model directly from the pre-installed local MinIO service

If installed using the install script and the `--quickstart` argument, a locally deployed MinIO should be available.

#### 1. Verify the `storage-config` secret for access to the pre-configured MinIO service

```
kubectl get secret storage-config -o json
```

There should be secret key called `localMinIO` that looks like:

```json
{
  "type": "s3",
  "access_key": "AKIAIOSFODNN7EXAMPLE",
  "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "endpoint_url": "http://minio:9000",
  "default_bucket": "modelmesh-example-models"
}
```

#### 2. Create a Predictor Custom Resource to serve the sample model

The `config/example-predictors` directory contains Predictor manifests for many of the example models. For a list of available models, see the [example models documentation](../example-models.md#available-models).

Here we are deploying an sklearn model located at `sklearn/mnist-svm.joblib` within the MinIO storage.

```shell
# Pulled from sample config/example-predictors/example-mlserver-sklearn-mnist-predictor.yaml
$ kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: example-mnist-predictor
spec:
  modelType:
    name: sklearn
  path: sklearn/mnist-svm.joblib
  storage:
    s3:
      secretKey: localMinIO
EOF
predictor.serving.kserve.io/example-mnist-predictor created
```

Note that `localMinIO` is the name of the secret key verified in the previous step.

For more details go to the [Predictor Spec page](predictor-cr.md).

Once the `Predictor` is created, mlserver runtime pods are automatically started to load and serve it.

```shell
$ kubectl get pods

NAME                                         READY   STATUS              RESTARTS   AGE
modelmesh-serving-mlserver-0.x-658b7dd689-46nwm    0/3     ContainerCreating   0          2s
modelmesh-serving-mlserver-0.x-658b7dd689-46nwm    0/3     ContainerCreating   0          2s
modelmesh-controller-568c45b959-nl88c       1/1     Running             0          11m
```

#### 3. Check the status of your Predictor:

```shell
$ kubectl get predictors
NAME                      TYPE      AVAILABLE   ACTIVEMODEL   TARGETMODEL   TRANSITION   AGE
example-mnist-predictor   sklearn   true        Loading                     UpToDate     60s

$ kubectl get predictor example-mnist-predictor -o=jsonpath='{.status.grpcEndpoint}'
grpc://modelmesh-serving:8033
```

The states should reflect immediate availability, but may take some seconds to move from `Loading` to `Loaded`.
Inferencing requests for this Predictor received prior to loading completion will block until it completes.

See the [Predictor Status](predictor-cr.md#predictor-status) section for details of how to interpret the different states.

---

**Note**

When `ScaleToZero` is enabled, the first Predictor assigned to the Triton runtime may be stuck in the `Pending` state for some time while the Triton pods are being created. The Triton image is large and may take a while to download.

---

## Using the deployed model

The built-in runtimes implement the gRPC protocol of the [KServe Predict API Version 2](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#grpc).
The `.proto` file for this API can be downloaded from [KServe's repo](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/grpc_predict_v2.proto)
or from the [`modelmesh-serving` repository at `fvt/proto/kfs_inference_v2.proto`](https://github.com/kserve/modelmesh-serving/blob/main/fvt/proto/kfs_inference_v2.proto).

To send an inference request, configure your gRPC client to point to address `modelmesh-serving:8033` and construct a request to the model using the `ModelInfer` RPC, setting the name of the Predictor as the `model_name` field in the `ModelInferRequest` message.

Here is an example of how to do this using the command-line based [grpcurl](https://github.com/fullstorydev/grpcurl):

Port-forward to access the runtime service:

```shell
# access via localhost:8033
$ kubectl port-forward service/modelmesh-serving 8033
Forwarding from 127.0.0.1:8033 -> 8033
Forwarding from [::1]:8033 -> 8033
```

In a separate terminal window, send an inference request using the proto file from `fvt/proto` or one that you have locally. Note that you have to provide the `model_name` in the data load, which is the name of the Predictor deployed.
Note that you have to set the `model_name` in the data payload to the name of the Predictor.

```shell
$ grpcurl -plaintext -proto fvt/proto/kfs_inference_v2.proto localhost:8033 list
inference.GRPCInferenceService

# run inference
# with below input, expect output to be 8
$ grpcurl -plaintext -proto fvt/proto/kfs_inference_v2.proto -d '{ "model_name": "example-mnist-predictor", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' localhost:8033 inference.GRPCInferenceService.ModelInfer

{
  "modelName": "example-mnist-predictor__ksp-7702c1b55a",
  "outputs": [
    {
      "name": "predict",
      "datatype": "FP32",
      "shape": [
        "1"
      ],
      "contents": {
        "fp32Contents": [
          8
        ]
      }
    }
  ]
}
```

## Updating the model

Changes can be made to the Predictor's Spec, such as changing the target storage and/or model, without interrupting the inferencing service.
The predictor will continue to use the prior spec/model until the new one is loaded and ready.

Below, we are changing the Predictor to use a completely different model, in practice the schema of the Predictor's model would be consistent across updates even if the type of model or ML framework changes.

```shell
$ kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1alpha1
kind: Predictor
metadata:
  name: example-mnist-predictor
spec:
  modelType:
    name: tensorflow
  # Note updated model type and location
  path: tensorflow/mnist.savedmodel
  storage:
    s3:
      secretKey: localMinIO
EOF
predictor.serving.kserve.io/example-mnist-predictor configured

$ kubectl get predictors
NAME                      TYPE         AVAILABLE   ACTIVEMODEL   TARGETMODEL   TRANSITION   AGE
example-mnist-predictor   tensorflow   true        Loaded        Loading       InProgress   10m
```

The "transition" state of the Predictor will be `InProgress` while waiting for the new backing model to be ready,
and return to `UpToDate` once the transition is complete.

```shell
$ kubectl get predictors
NAME                      TYPE         AVAILABLE   ACTIVEMODEL   TARGETMODEL   TRANSITION   AGE
example-mnist-predictor   tensorflow   true        Loaded                      UpToDate     31m
```

If there is a problem loading the new model (for example it does not exist at the specified path), the transition state will
change to `BlockedByFailedLoad`, but the service will remain available. The active model state will still show as `Loaded`, and the
Predictor remains available.

```shell
$ kubectl get predictors
NAME                      TYPE         AVAILABLE   ACTIVEMODEL   TARGETMODEL   TRANSITION             AGE
example-mnist-predictor   tensorflow   true        Loaded        Failed        BlockedByFailedLoad    20m
```

## For More Details

- [Setup Storage](setup-storage.md)
- [Inferencing](run-inference.md)
- [Predictor Spec](predictor-cr.md)
