# Using ModelMesh Serving

Trained models are deployed in ModelMesh Serving via `InferenceService`s. The `predictor` component of an `InferenceService` represents a stable service endpoint behind which the underlying model can change.

Models must reside on shared storage. Currently, S3, GCS, and Azure Blob Storage are supported with limited supported for HTTP(S). Note that model data residing at a particular path within a given storage instance is **assumed to be immutable**. Different versions of the same logical model are treated at the base level as independent models and must reside at different paths. In particular, where a given model server/runtime natively supports the notion of versioning (such as Nvidia Triton, TensorFlow Serving, etc), the provided path should not point to the top of a (pseudo-)directory structure containing multiple versions. Instead, point to the subdirectory which corresponds to a specific version.

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
  "bucket": "modelmesh-example-models"
}
```

#### 2. Create an InferenceService to serve the sample model

The `config/example-isvcs` directory contains `InferenceService` manifests for many of the example models. For a list of available models, see the [example models documentation](../example-models.md#available-models).

Here we are deploying an sklearn model located at `sklearn/mnist-svm.joblib` within the MinIO storage.

```shell
# Pulled from sample config/example-isvcs/example-mlserver-sklearn-mnist-isvc.yaml
$ kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: example-mnist-isvc
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: sklearn
      storage:
        key: localMinIO
        path: sklearn/mnist-svm.joblib
EOF
inferenceservice.serving.kserve.io/example-mnist-isvc created
```

Note that `localMinIO` is the name of the secret key verified in the previous step.

For more details go to the [InferenceService Spec page](inferenceservice-cr.md).

Once the `InferenceService` is created, mlserver runtime pods are automatically started to load and serve it.

```shell
$ kubectl get pods

NAME                                               READY   STATUS              RESTARTS   AGE
modelmesh-serving-mlserver-1.x-658b7dd689-46nwm    0/3     ContainerCreating   0          2s
modelmesh-serving-mlserver-1.x-658b7dd689-46nwm    0/3     ContainerCreating   0          2s
modelmesh-controller-568c45b959-nl88c              1/1     Running             0          11m
```

#### 3. Check the status of your InferenceService:

```shell
$ kubectl describe isvc example-mnist-isvc
...
Status:
  Conditions:
    Last Transition Time:  2022-07-15T04:59:45Z
    Status:                False
    Type:                  PredictorReady
    Last Transition Time:  2022-07-15T04:59:45Z
    Status:                False
    Type:                  Ready
  Model Status:
    Copies:
      Failed Copies:  0
      Total Copies:   1
    Last Failure Info:
      Message:              Waiting for runtime Pod to become available
      Model Revision Name:  example-mnist-isvc__isvc-6b2eb0b8bf
      Reason:               RuntimeUnhealthy
    States:
      Active Model State:  Loading
      Target Model State:
    Transition Status:     UpToDate

```

```shell
$ kubectl get isvc example-mnist-isvc -o=jsonpath='{.status.components.predictor.grpcUrl}'
grpc://modelmesh-serving.modelmesh-serving:8033
```

The active model state should reflect immediate availability, but may take some seconds to move from `Loading` to `Loaded`.
Inferencing requests for this `InferenceService` received prior to loading completion will block until it completes.

See the [InferenceService Status](inferenceservice-cr.md#predictor-status) section for details of how to interpret the different states.

---

**Note**

When `ScaleToZero` is enabled, the first `InferenceService` assigned to the Triton runtime may be stuck in the `Pending` state for some time while the Triton pods are being created. The Triton image is large and may take a while to download.

---

## Using the deployed model

The built-in runtimes implement the gRPC protocol of the [KServe Predict API Version 2](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#grpc).
The `.proto` file for this API can be downloaded from [KServe's repo](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/grpc_predict_v2.proto)
or from the [`modelmesh-serving` repository at `fvt/proto/kfs_inference_v2.proto`](https://github.com/kserve/modelmesh-serving/blob/main/fvt/proto/kfs_inference_v2.proto).

To send an inference request, configure your gRPC client to point to address `modelmesh-serving:8033` and construct a request to the model using the `ModelInfer` RPC, setting the name of the `InferenceService` as the `model_name` field in the `ModelInferRequest` message.

Here is an example of how to do this using the command-line based [grpcurl](https://github.com/fullstorydev/grpcurl):

Port-forward to access the runtime service:

```shell
# access via localhost:8033
$ kubectl port-forward service/modelmesh-serving 8033
Forwarding from 127.0.0.1:8033 -> 8033
Forwarding from [::1]:8033 -> 8033
```

In a separate terminal window, send an inference request using the proto file from `fvt/proto` or one that you have locally. Note that you have to provide the `model_name` in the data load, which is the name of the `InferenceService` deployed.
Note that you have to set the `model_name` in the data payload to the name of the `InferenceService`.

```shell
$ grpcurl -plaintext -proto fvt/proto/kfs_inference_v2.proto localhost:8033 list
inference.GRPCInferenceService

# run inference
# with below input, expect output to be 8
$ grpcurl -plaintext -proto fvt/proto/kfs_inference_v2.proto -d '{ "model_name": "example-mnist-isvc", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' localhost:8033 inference.GRPCInferenceService.ModelInfer

{
  "modelName": "example-mnist-isvc___isvc-3642375d03",
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

Changes can be made to the `InferenceService` predictor spec, such as changing the target storage and/or model, without interrupting the inferencing service.
The `InferenceService` will continue to use the prior spec/model until the new one is loaded and ready.

Below, we are changing the `InferenceService` to use a completely different model, in practice the schema of the `InferenceService`'s model would be consistent across updates even if the type of model or ML framework changes.

```shell
$ kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name:  example-mnist-isvc
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
EOF
inferenceservice.serving.kserve.io/example-mnist-isvc configured

$ kubectl describe isvc example-mnist-isvc
...
 Model Status:
    Copies:
      Failed Copies:  0
      Total Copies:   2
    States:
      Active Model State:  Loaded
      Target Model State:  Loading
    Transition Status:     InProgress
...
```

The "transition" status of the `InferenceService` will be `InProgress` while waiting for the new backing model to be ready,
and return to `UpToDate` once the transition is complete.

```shell
$ kubectl get isvc
NAME                 URL                                               READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION   AGE
example-mnist-isvc   grpc://modelmesh-serving.modelmesh-serving:8033   True                                                                  15m

```

If there is a problem loading the new model (for example it does not exist at the specified path), the transition state will
change to `BlockedByFailedLoad`, but the service will remain available. The active model state will still show as `Loaded`, and the
`InferenceService` remains available.

## For More Details

- [Setup Storage](setup-storage.md)
- [Inferencing](run-inference.md)
- [InferenceService Spec](inferenceservice-cr.md)
