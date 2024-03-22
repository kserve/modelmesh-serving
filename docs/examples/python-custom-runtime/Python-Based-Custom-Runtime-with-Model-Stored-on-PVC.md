# The Python-Based Custom Runtime with Model Stored on Persistent Volume Claim

This document provides step-by-step instructions to demonstrate how to write a custom Python-based `ServingRuntime` inheriting from [MLServer's MLModel class](https://github.com/SeldonIO/MLServer/blob/master/mlserver/model.py) and deploy a model stored on persistent volume claims with it.

This example assumes that ModelMesh Serving was deployed using the [quickstart guide](https://github.com/kserve/modelmesh-serving/blob/main/docs/quickstart.md).

# Deploy a model stored on a Persistent Volume Claim

Let's use namespace `modelmesh-serving` here:

```shell
kubectl config set-context --current --namespace=modelmesh-serving
```

## 1. Create PV and PVC for storing model file

```shell
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-models-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  hostPath:
    path: "/mnt/models"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: "my-models-pvc"
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
EOF
```

## 2. Create a pod to access the PVC

```shell
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: Pod
metadata:
  name: "pvc-access"
spec:
  containers:
    - name: main
      image: ubuntu
      command: ["/bin/sh", "-ec", "sleep 10000"]
      volumeMounts:
        - name: "my-pvc"
          mountPath: "/mnt/models"
  volumes:
    - name: "my-pvc"
      persistentVolumeClaim:
        claimName: "my-models-pvc"
EOF
```

## 3. Store the model on this persistent volume

The sample model file we used in this doc is `sklearn/mnist-svm.joblib`.

```shell
curl -sOL https://github.com/kserve/modelmesh-minio-examples/raw/main/sklearn/mnist-svm.joblib
```

Copy this model file to the `pvc-access` pod:

```shell
kubectl cp mnist-svm.joblib pvc-access:/mnt/models/
```

Verify the model exists on the persistent volume：

```shell
kubectl exec -it pvc-access -- ls -alr /mnt/models/

# total 348
# -rw-rw-r-- 1 1000 1000 344817 Mar 19 08:37 mnist-svm.joblib
# drwxr-xr-x 1 root root   4096 Mar 19 08:34 ..
# drwxr-xr-x 2 root root   4096 Mar 19 08:37 .
```

## 4. Configure ModelMesh Serving to use the persistent volume claim

Create the `model-serving-config` ConfigMap with the setting allowAnyPVC: true:

```shell
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: model-serving-config
data:
  config.yaml: |
    allowAnyPVC: true
EOF
```

Verify the configuration setting:

```shell
kubectl get cm "model-serving-config" -o jsonpath="{.data['config\.yaml']}"
```

# Implement the Python-based Custom Runtime on MLServer

All of the necessary resources are contained in [custom-model](./custom-model), including the model code, and the Dockerfile.

## 1. Implement the API of MLModel

Both `load` and `predict` must be implemented to support this custom `ServingRuntime`. The code file [custom_model.py](./custom-model/custom_model.py) provides a simplified implementation of `CustomMLModel` for model `mnist-svm.joblib`. You can read more about it [here](https://github.com/kserve/modelmesh-serving/blob/main/docs/runtimes/mlserver_custom.md).

## 2. Build the custom ServingRuntime image

You can use [`mlserver`](https://mlserver.readthedocs.io/en/stable/examples/custom/README.html#building-a-custom-image) or `docker` to help to build the custom `ServingRuntime` image, and the latter is done in [Dockerfile](./custom-model/Dockerfile).

To build the image, execute the following command from within the [custom-model](./custom-model) directory.

```shell
docker build -t <DOCKER-HUB-ORG>/custom-model-server:0.1 .

```

> **Note**: Please use the `--build-arg` to add the http proxy if there is proxy in user's environment, such as:

```shell
docker build --build-arg HTTP_PROXY=http://<DOMAIN-OR-IP>:PORT --build-arg HTTPS_PROXY=http://<DOMAIN-OR-IP>:PORT -t <DOCKER-HUB-ORG>/custom-model-server:0.1 .
```

## 3. Define and Apply Custom ServingRuntime

Below, you will create a ServingRuntime using the image built above. You can learn more about the custom `ServingRuntime` template [here](https://github.com/kserve/modelmesh-serving/blob/main/docs/runtimes/mlserver_custom.md#custom-servingruntime-template).

```shell
kubectl apply -f - <<EOF
---
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: my-custom-model-0.x
spec:
  supportedModelFormats:
    - name: custom_model
      version: "1"
      autoSelect: true
  multiModel: true
  grpcDataEndpoint: port:8001
  grpcEndpoint: port:8085
  containers:
    - name: mlserver
      image: <DOCKER-HUB-ORG>/custom-model-server:0.1
      env:
        - name: MLSERVER_MODELS_DIR
          value: "/models/_mlserver_models/"
        - name: MLSERVER_GRPC_PORT
          value: "8001"
        - name: MLSERVER_HTTP_PORT
          value: "8002"
        - name: MLSERVER_LOAD_MODELS_AT_STARTUP
          value: "false"
        - name: MLSERVER_MODEL_NAME
          value: dummy-model
        - name: MLSERVER_HOST
          value: "127.0.0.1"
        - name: MLSERVER_GRPC_MAX_MESSAGE_LENGTH
          value: "-1"
      resources:
        requests:
          cpu: 500m
          memory: 1Gi
        limits:
          cpu: "5"
          memory: 1Gi
  builtInAdapter:
    serverType: mlserver
    runtimeManagementPort: 8001
    memBufferBytes: 134217728
    modelLoadingTimeoutMillis: 90000
EOF
```

Verify the available `ServingRuntime`, including the custom one:

```shell
kubectl get servingruntimes

NAME                  DISABLED   MODELTYPE      CONTAINERS   AGE
mlserver-1.x                     sklearn        mlserver     10m
my-custom-model-0.x              custom_model   mlserver     10m
ovms-1.x                         openvino_ir    ovms         10m
torchserve-0.x                   pytorch-mar    torchserve   10m
triton-2.x                       keras          triton       10m
```

## 4. Deploy the InferenceService using the custom ServingRuntime

```shell
kubectl apply -f - <<EOF
---
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: sklearn-pvc-example
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
spec:
  predictor:
    model:
      modelFormat:
        name: custom-model
      runtime: my-custom-model-0.x
      storage:
        parameters:
          type: pvc
          name: my-models-pvc
        path: mnist-svm.joblib
EOF
```

After a few seconds, this InferenceService named `sklearn-pvc-example` should be ready:

```shell
kubectl get isvc
NAME                  URL                                               READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION   AGE
sklearn-pvc-example   grpc://modelmesh-serving.modelmesh-serving:8033   True                                                                  69s
```

## 5. Run an inference request for this InferenceService

Firstly, set up a port-forward to facilitate REST requests:

```shell
kubectl port-forward --address 0.0.0.0 service/modelmesh-serving 8008 &

# [1] running kubectl port-forward in the background
# Forwarding from 0.0.0.0:8008 -> 8008
```

Performing an inference request to the SKLearn MNIST model via `curl`. Make sure the `MODEL_NAME` variable is set correctly.

```shell
MODEL_NAME="sklearn-pvc-example"
curl -s -X POST -k "http://localhost:8008/v2/models/${MODEL_NAME}/infer" -d '{"inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "data": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0]}]}' | jq .
{
  "model_name": "sklearn-pvc-example__isvc-72fbffc584",
  "outputs": [
    {
      "name": "predict",
      "datatype": "INT64",
      "shape": [1],
      "data": [8]
    }
  ]
}
```

> **Note**: `jq` is optional, it is used to format the output of the InferenceService.

To delete the resources created in this example, run the following commands:

```shell
kubectl delete isvc "sklearn-pvc-example"
kubectl delete pod "pvc-access"
kubectl delete pvc "my-models-pvc"
```
