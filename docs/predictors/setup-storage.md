# Set up Storage for Loading Models

You will need access to an S3-compatible object storage, for example [MinIO](https://github.com/minio/minio). To configure access to the object storage, use the `storage-config` secret.

Alternatively, models can be stored on a Kubernetes Persistent Volume. Persistent Volume Claims can either be pre-configured in the `storage-config` secret, or, the `allowAnyPVC` configuration flag can be enabled, so that any PVC can be mounted dynamically at the time a predictor or inference service is deployed.

## Deploy a model from your own S3 compatible object storage

### 1. Download sample model or use an existing model

Here we show an example using an [ONNX model for MNIST](https://github.com/onnx/models/raw/ad5c181f1646225f034fba1862233ecb4c262e04/vision/classification/mnist/model/mnist-8.onnx).

### 2. Add your saved model to S3-based object storage

A bucket in MinIO needs to be created to copy the model into, which either requires [MinIO Client](https://docs.min.io/docs/minio-client-quickstart-guide.html) or port-forwarding the minio service and logging in using the web interface.

```shell
# Install minio client
$ brew install minio/stable/mc
$ mc --help
NAME:
  mc - MinIO Client for cloud storage and filesystems.
....

# test setup - mc is pre-configured with https://play.min.io, aliased as "play".
# list all buckets in play
$ mc ls play

[2021-06-10 21:04:25 EDT]     0B 2063b651-92a3-4a20-a4a5-03a96e7c5a89/
[2021-06-11 02:40:33 EDT]     0B 5ddfe44282319c500c3a4f9b/
[2021-06-11 05:15:45 EDT]     0B 6dkmmiqcdho1zoloomsj3620cocs6iij/
[2021-06-11 02:39:54 EDT]     0B 9jo5omejcyyr62iizn02ex982eapipjr/
[2021-06-11 02:33:53 EDT]     0B a-test-zip/
[2021-06-11 09:14:28 EDT]     0B aio-ato/
[2021-06-11 09:14:29 EDT]     0B aio-ato-art/
...

# add cloud storage service
$ mc alias set <ALIAS> <YOUR-S3-ENDPOINT> [YOUR-ACCESS-KEY] [YOUR-SECRET-KEY]
# for example if you installed with --quickstart
$ mc alias set myminio http://localhost:9000 EXAMPLE_ACESS_KEY example/secret/EXAMPLEKEY
Added `myminio` successfully.

# create bucket
$ mc mb myminio/models/onnx
Bucket created successfully myminio/models/onnx.

$ mc tree myminio
myminio
└─ models
   └─ onnx

# copy object -- must copy into an existing bucket
$ mc cp ~/Downloads/mnist-8.onnx myminio/models/onnx
...model.lr.zip:  26.45 KiB / 26.45 KiB  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  2.74 MiB/s 0s

$ mc ls myminio/models/onnx
[2021-06-11 11:55:48 EDT]  26KiB mnist-8.onnx
```

### 3. Add a storage entry to the `storage-config` secret

Ensure there is a key defined in the common `storage-config` secret corresponding to the S3-based storage instance holding your model. The value of this secret key should be JSON like the following, `bucket` is optional.

Users can specify use of a custom certificate via the storage config `certificate` parameter. The custom certificate should be in the form of an embedded Certificate Authority (CA) bundle in PEM format.

Using MinIO the JSON contents look like:

```json
{
  "type": "s3",
  "access_key_id": "minioadmin",
  "secret_access_key": "minioadmin/K7JTCMP/EXAMPLEKEY",
  "endpoint_url": "http://127.0.0.1:9000:9000",
  "bucket": "",
  "region": "us-east"
}
```

Example secret key contents for GCS and Azure Blob Storage are:

```yaml
gcsKey: |
  {
    "type": "gcs",
    "private_key": "-----BEGIN PRIVATE KEY-----\nAABBCC1122----END PRIVATE KEY-----\n",
    "client_email": "storage-auth@secret-12345.gserviceaccount.com",
    "token_uri": "https://oauth2.googleapis.com/token"
  }
azureKey: |
  {
    "type": "azure",
    "account_name": "az-account",
    "container": "az-container",
    "connection_string": "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=Yabc983f11822334455;EndpointSuffix=core.windows.net"
  }
```

Remember that after updating the storage config secret, there may be a delay of up to 2 minutes until the change is picked up. You should take this into account when creating/updating `InferenceService`s that use storage keys which have just been added or updated - they may fail to load otherwise.

## Deploy a model stored on a Persistent Volume Claim

Models can be stored on [Kubernetes Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).

There are two ways to enable PVC support in ModelMesh:

1. The Persistent Volume Claims can be added in the `storage-config` secret. This way all PVCs will be mounted to all serving runtime pods.
2. The `allowAnyPVC` configuration flag can be set to `true`. This way the ModelMesh controller will dynamically mount the PVC to a runtime pod at the time a predictor or inference service requiring it is being deployed.

Follow the example instructions below to create a PVC, store a model on it, and configure ModelMesh to mount the PVC to the runtime serving pods so that the model can be loaded for inferencing.

### 1. Create a Persistent Volume Claim

Persistent Volumes are namespace-scoped, so we have to create it in the same namespace as the ModelMesh serving deployment. We are using namespace `modelmesh-serving` here.

```shell
kubectl config set-context --current --namespace=modelmesh-serving
```

Now we create the Persistent Volume Claim `my-models-pvc`. Along with it, we deploy a `pvc-access` pod in order to copy our model to the Persistent Volume later.

```shell
kubectl apply -f - <<EOF
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

### 2. Add the model to the Persistent Volume

For this example we are using the MNIST SVM scikit-learn model from the [kserve/modelmesh-minio-examples](https://github.com/kserve/modelmesh-minio-examples) repo.

```shell
# create a temp directory and download the scikit-learn MNIST SVM model
mkdir -p temp/sklearn && cd temp/sklearn && \
  curl -sOL https://github.com/kserve/modelmesh-minio-examples/raw/main/sklearn/mnist-svm.joblib && \
  cd -

# verify the sklearn model exists
ls -al temp/sklearn/

# total 680
# drwxr-xr-x  3 owner  group      96 Mar 16 01:18 .
# drwxr-xr-x  9 owner  group     288 Mar 16 01:18 ..
# -rw-r--r--  1 owner  group  344817 Mar 16 01:18 mnist-svm.joblib
```

Copy the sklearn model onto the PVC via the `pvc-access` pod that we deployed alongside the `my-models-pvc`.

```shell
# create a sub-folder 'sklearn' on the persistent volume
kubectl exec -it pvc-access -- mkdir -p /mnt/models/sklearn

# copy the sklearn/mnist-svm.joblib file we downloaded earlier onto the PV which is mounted to the pvc-access pod
kubectl cp temp/sklearn/mnist-svm.joblib pvc-access:/mnt/models/sklearn/mnist-svm.joblib

# verify the model exists on the PV
kubectl exec -it pvc-access -- ls -alr /mnt/models/sklearn/

# total 352
# -rw-r--r-- 1    501 staff      344817 Mar 16 08:55 mnist-svm.joblib
# drwxr-xr-x 3 nobody 4294967294   4096 Mar 16 08:55 ..
# drwxr-xr-x 2 nobody 4294967294   4096 Mar 16 08:55 .
```

### 3. (a) Add a PVC entry to the `storage-config` secret

The `storage-config` secret is part of the ModelMesh [Quickstart](/docs/quickstart.md) deployment. If you deployed ModelMesh without it, you can create it using the YAML spec outlined below.

To configure ModelMesh to mount the PVC to the runtime serving pods, we need to add an entry of type `pvc` to the secret's `stringData`. The chosen key `pvc1` is of no consequence. Note that the `localMinIO` and the `pvc2` entries are only for illustration.

```YAML
apiVersion: v1
kind: Secret
metadata:
  name: storage-config
stringData:
#  localMinIO: |
#    {
#      "type": "s3",
#      "access_key_id": "AKIAIOSFODNN7EXAMPLE",
#      "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
#      "endpoint_url": "http://minio:9000",
#      "bucket": "modelmesh-example-models",
#      "region": "us-south"
#    }
  pvc1: |
    {
      "type": "pvc",
      "name": "my-models-pvc"
    }
#  pvc2: |
#    {
#      "type": "pvc",
#      "name": "some-other-pvc"
#    }
```

After updating or creating the `storage-config` secret, the `modelmesh-serving` deployment will get updated and the serving runtime pods will get restarted to mount the Persistent Volumes. Depending on the number of replicas and deployed predictors, this update may take a few minutes.

### 3. (b) Enable `allowAnyPVC` in the `model-serving-config` ConfigMap

As an alternative to preconfiguring all _allowed_ PVCs in the `storage-config` secret, you can set the `allowAnyPVC` configuration flag to `true`. With `allowAnyPVC` enabled, users can deploy Predictors or InferenceServices with models stored on _any_ PVC in the model serving namespace.

Let's update (or create) the `model-serving-config` ConfigMap.

**Note**, if you already have a `model-serving-config` ConfigMap, you might want to retain the existing config overrides. You can check your current configuration flags by running:

```shell
kubectl get cm "model-serving-config" -o jsonpath="{.data['config\.yaml']}"`
```

The minimal `model-serving-config` for our example requires the settings `allowAnyPVC` and `restProxy` to be enabled:

```shell
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: model-serving-config
data:
  config.yaml: |
    # check which other config overrides should be here:
    #  kubectl get cm "model-serving-config" -o jsonpath="{.data['config\.yaml']}"
    allowAnyPVC: true
    restProxy:
      enabled: true
EOF
```

After applying the new configuration, the `modelmesh-serving` deployment might get updated and the serving runtime pods may get restarted.

### 4. Deploy a new Inference Service

In order to use the model from the PVC, we need to set the `storage` `parameters` of the predictor
as `type: pvc` and `name: my-models-pvc` like this:

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
        name: sklearn
      storage:
        parameters:
          type: pvc
          name: my-models-pvc
        path: sklearn/mnist-svm.joblib
EOF
```

After a few seconds, the new InferenceService `sklearn-pvc-example` should be ready:

```shell
kubectl get isvc

# NAME                  URL                                               READY   PREV   LATEST   AGE
# sklearn-pvc-example   grpc://modelmesh-serving.modelmesh-serving:8033   True                    23s
```

### 5. Run an inference request

We need to set up a port-forward to facilitate REST requests.

```shell
kubectl port-forward --address 0.0.0.0 service/modelmesh-serving 8008 &

# [1] running kubectl port-forward in the background
# Forwarding from 0.0.0.0:8008 -> 8008
```

With `curl` we can perform an inference request to the SKLearn MNIST model. Make sure the `MODEL_NAME`
variable is set to the name of your `InferenceService`.

```shell
MODEL_NAME="sklearn-pvc-example"

curl -X POST -k "http://localhost:8008/v2/models/${MODEL_NAME}/infer" -d '{"inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "data": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0]}]}'
```

The response should look like the following:

```json
{
  "model_name": "sklearn-pvc-example__isvc-3d2daa3370",
  "outputs": [
    {"name": "predict", "datatype": "INT64", "shape": [1, 1], "data": [8]}
  ]
}
```

You can find more detailed information about running inference requests [here](run-inference.md).

To delete the resources created in this example, run the following commands:

```shell
kubectl delete isvc "sklearn-pvc-example"
kubectl delete pod "pvc-access"
kubectl delete pvc "my-models-pvc"
```
