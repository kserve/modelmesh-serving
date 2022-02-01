# InferenceService Deployment

ModelMesh Serving supports deploying models using KServe's
[InferenceService interface](https://github.com/kserve/kserve/blob/master/config/crd/serving.kserve.io_inferenceservices.yaml).

By default, the ModelMesh controller will reconcile InferenceService resources if the
`inferenceservices.serving.kserve.io` CRD is accessible on the cluster. This CRD comes with the installation of
[KServe](https://kserve.github.io/website/).

While both the KServe controller and ModelMesh controller will reconcile InferenceService resources, the ModelMesh controller will
only handle those InferenceServices with the `serving.kserve.io/deploymentMode: ModelMesh` annotation. Otherwise, the KServe controller will
handle reconciliation. Likewise, the KServe controller will not reconcile an InferenceService with the `serving.kserve.io/deploymentMode: ModelMesh`
annotation, and will defer under the assumption that the ModelMesh controller will handle it.

**Note:** the InferenceService CRD is currently evolving and ModelMesh support for the CRD is only preliminary. Many features like transformers, explainers, and canary rollouts do not currently apply to InferenceServices with `deploymentMode` set to `ModelMesh`. And `PodSpec` fields that are set in the InferenceService Predictor spec will be ignored. Please assume that the interface and how ModelMesh uses it are subject to change.

## Deploy an InferenceService

First, Set your namespace context to `modelmesh-serving` or whatever your modelmesh-enabled user namespace is.

```shell
kubectl config set-context --current --namespace=modelmesh-serving
```

Try applying an SKLearn MNIST model served from the local MinIO container:

```shell
kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: example-sklearn-isvc
  annotations:
    serving.kserve.io/deploymentMode: ModelMesh
    serving.kserve.io/secretKey: localMinIO
spec:
  predictor:
    sklearn:
      storageUri: s3://modelmesh-example-models/sklearn/mnist-svm.joblib
EOF
```

Currently, the following storage providers are supported: S3, GCS, and HTTP(S). However, with the HTTP storage provider, we don't yet support archive format extractions (e.g. tar.gz, zip), so its use is limited to all-in-one model files such as `.joblib` files for SKLearn.

Storage credentials are expected to be stored in a secret called [`storage-config`](https://github.com/kserve/modelmesh-serving/blob/main/config/default/storage-secret.yaml). This means that the `storage-config` secret can contain a map of several keys that correspond to various credentials.

Example secret keys for S3 and GCS:

```yaml
s3Key: |
  {
    "type": "s3",
    "access_key_id": "abc983f1182233445566778899d12345",
    "secret_access_key": "abcdff6a11223344aabbcc66ee231e6dd0c1122ff1234567",
    "endpoint_url": "https://s3.us-south.cloud-object-storage.appdomain.cloud",
    "region": "us-south",
    "bucket": "modelmesh-example-public"
  }
gcsKey: |
  {
    "type": "gcs",
    "private_key": "-----BEGIN PRIVATE KEY-----\nAABBCC1122----END PRIVATE KEY-----\n",
    "client_email": "storage-auth@secret-12345.gserviceaccount.com",
    "token_uri": "https://oauth2.googleapis.com/token"
  }
```

In the InferenceService metadata, the annotation `serving.kserve.io/secretKey` is used as a placeholder for this needed secret key field.
If the storage endpoint is publicly accessible, then this annotation can be omitted.

Some other optional annotations that can be used are:

- `serving.kserve.io/schemaPath`: The path within the object storage of a schema file. This allows specifying the input and output schema of ML models.
  - For example, if your model `storageURI` was `s3://modelmesh-example-models/pytorch/pytorch-cifar` the schema file would currently need to be in the
    same bucket (`modelmesh-example-models`). The path within this bucket is what would be specified in this annotation (e.g. `pytorch/schema/schema.json`)
- `serving.kserve.io/servingRuntime`: A ServingRuntime name can be specified explicitly to have the InferenceService use that.

You can find storage layout information for various model types [here](https://github.com/kserve/modelmesh-serving/tree/main/docs/model-types). This might come in handy when specifying a `storageUri` in the InferenceService.

## Check InferenceService

After deploying, list the InferenceServices to check the status.

```shell
kubectl get isvc
```

Depending on if a `ServingRuntime` was already deployed for another model or not, readiness for the newly deployed model may take a bit. Generally, deployment of models into existing `ServingRuntimes` is quick.

```shell
NAME                   URL                             READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION   AGE
example-sklearn-isvc   grpc://modelmesh-serving:8033   True                                                                  16s
```

You can describe the InferenceService to get more status information:

```shell
kubectl describe example-sklearn-isvc


Name:         example-sklearn-isvc
Namespace:    modelmesh-serving
...
...
Status:
  Active Model State:  Loaded
  Available:           true
  Conditions:
    Status:            True
    Type:              Ready
  Failed Copies:       0
  Grpc Endpoint:       grpc://modelmesh-serving:8033
  Http Endpoint:       http://modelmesh-serving:8008
  Target Model State:
  Transition Status:   UpToDate
  URL:                 grpc://modelmesh-serving:8033
...
```

## Perform inference

For instructions on how to perform inference on the newly deployed InferenceService, please refer back to the
[quickstart instructions](./quickstart.md#3-perform-an-inference-request).

```shell
kubectl port-forward --address 0.0.0.0 service/modelmesh-serving  8033 -n modelmesh-serving
```

```shell
MODEL_NAME=example-sklearn-isvc
grpcurl \
  -plaintext \
  -proto fvt/proto/kfs_inference_v2.proto \
  -d '{ "model_name": "'"${MODEL_NAME}"'", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' \
  localhost:8033 \
  inference.GRPCInferenceService.ModelInfer
```
