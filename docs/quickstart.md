# Quick Start Guide

To quickly get started using ModelMesh Serving, here is a brief guide.

## Prerequisites

- A Kubernetes cluster v 1.16+ with cluster administrative privileges
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) (v4.0.0+)
- At least 4 vCPU and 8 GB memory. For more details, please see [here](install/environment.md#deployed-components).

## 1. Install ModelMesh Serving

### Get the latest release

```shell
RELEASE=release-0.8
git clone -b $RELEASE --depth 1 --single-branch https://github.com/kserve/modelmesh-serving.git
cd modelmesh-serving
```

### Run install script

```shell
kubectl create namespace modelmesh-serving
./scripts/install.sh --namespace modelmesh-serving --quickstart
```

This will install ModelMesh serving in the `modelmesh-serving` namespace, along with an etcd and MinIO instances.
Eventually after running this script, you should see a `Successfully installed ModelMesh Serving!` message.

To see more details about installation, click [here](./install/install-script.md).

### Verify installation

Check that the pods are running:

```shell
kubectl get pods

NAME                                        READY   STATUS    RESTARTS   AGE
pod/etcd                                    1/1     Running   0          5m
pod/minio                                   1/1     Running   0          5m
pod/modelmesh-controller-547bfb64dc-mrgrq   1/1     Running   0          5m
```

Check that the `ServingRuntimes` are available:

```shell
kubectl get servingruntimes

NAME           DISABLED   MODELTYPE    CONTAINERS   AGE
mlserver-0.x              sklearn      mlserver     5m
triton-2.x                tensorflow   triton       5m
```

`ServingRuntimes` are automatically provisioned based on the framework of the model deployed.
Two `ServingRuntimes` are included with ModelMesh Serving by default. The current mappings for these
are:

| ServingRuntime | Supported Frameworks                |
| -------------- | ----------------------------------- |
| triton-2.x     | tensorflow, pytorch, onnx, tensorrt |
| mlserver-0.x   | sklearn, xgboost, lightgbm          |

## 2. Deploy a model

With ModelMesh Serving now installed, try deploying a model using the `Predictor` CRD.

> **Note**: ModelMesh Serving also supports deployment using KServe's InferenceService interface.
> Please refer to [these instructions](./inferenceservice.md) for information on alternatively using InferenceServices.

Here, we deploy an SKLearn MNIST model which is served from the local MinIO container:

```shell
kubectl apply -f - <<EOF
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
```

After applying this predictor, you should see it in the `Loading` state:

```
kubectl get predictors

NAME                      TYPE      AVAILABLE   ACTIVEMODEL   TARGETMODEL   TRANSITION   AGE
example-mnist-predictor   sklearn   false       Loading                     UpToDate     7s
```

Eventually, you should see the ServingRuntime pods that will hold the SKLearn model become `Running`.

```shell
kubectl get pods

...
modelmesh-serving-mlserver-0.x-7db675f677-twrwd   3/3     Running   0          2m
modelmesh-serving-mlserver-0.x-7db675f677-xvd8q   3/3     Running   0          2m
```

Then, checking on the `predictors` again, you should see that it is now available:

```shell
kubectl get predictors

NAME                      TYPE      AVAILABLE   ACTIVEMODEL   TARGETMODEL   TRANSITION   AGE
example-mnist-predictor   sklearn   true        Loaded                      UpToDate     2m
```

To see more detailed instructions and information, click [here](./predictors/).

## 3. Perform an inference request

Now that a model is loaded and available, you can then perform inference.
Currently, only gRPC inference requests are supported by ModelMesh, but REST support is enabled via a [REST proxy](https://github.com/kserve/rest-proxy) container. By default, ModelMesh Serving uses a
[headless Service](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services)
since a normal Service has issues load balancing gRPC requests. See more info
[here](https://kubernetes.io/blog/2018/11/07/grpc-load-balancing-on-kubernetes-without-tears/).

### gRPC request

To test out **gRPC** inference requests, you can port-forward the headless service _in a separate terminal window_:

```shell
kubectl port-forward --address 0.0.0.0 service/modelmesh-serving  8033 -n modelmesh-serving
```

Then a gRPC client generated from the KServe [grpc_predict_v2.proto](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/grpc_predict_v2.proto)
file can be used with `localhost:8033`. A ready-to-use Python example of this can be found [here](https://github.com/pvaneck/model-serving-sandbox/tree/main/grpc-predict).

Alternatively, you can test inference with [grpcurl](https://github.com/fullstorydev/grpcurl). This can easily be installed with `brew install grpcurl` if on macOS.

With `grpcurl`, a request can be sent to the SKLearn MNIST model like the following. Make sure that the `MODEL_NAME`
variable below is set to the name of your Predictor/InferenceService.

```shell
MODEL_NAME=example-mnist-predictor
grpcurl \
  -plaintext \
  -proto fvt/proto/kfs_inference_v2.proto \
  -d '{ "model_name": "'"${MODEL_NAME}"'", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' \
  localhost:8033 \
  inference.GRPCInferenceService.ModelInfer
```

This should give you output like the following:

```json
{
  "modelName": "example-mnist-predictor__ksp-7702c1b55a",
  "outputs": [
    {
      "name": "predict",
      "datatype": "FP32",
      "shape": ["1"],
      "contents": {
        "fp32Contents": [8]
      }
    }
  ]
}
```

### REST request

> **Note**: The REST proxy is currently in an alpha state and may still have issues with certain usage scenarios.

You will need to port-forward a different port for REST.

```shell
kubectl port-forward --address 0.0.0.0 service/modelmesh-serving 8008 -n modelmesh-serving
```

With `curl`, a request can be sent to the SKLearn MNIST model like the following. Make sure that the `MODEL_NAME`
variable below is set to the name of your Predictor/InferenceService.

```shell
MODEL_NAME=example-mnist-predictor
curl -X POST -k http://localhost:8008/v2/models/${MODEL_NAME}/infer -d '{"inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "data": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0]}]}'
```

This should give you a response like the following:

```json
{
  "model_name": "example-mnist-predictor__ksp-7702c1b55a",
  "outputs": [
    {
      "name": "predict",
      "datatype": "FP32",
      "shape": [1],
      "data": [8]
    }
  ]
}
```

To see more detailed instructions and information, click [here](./predictors/run-inference.md).

## 4. (Optional) Deleting your ModelMesh Serving installation

To delete all ModelMesh Serving resources that were installed, run the following from the root of the project:

```shell
./scripts/delete.sh --namespace modelmesh-serving
```
