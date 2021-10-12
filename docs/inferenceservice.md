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

Currently, only S3 based storage is supported. S3 credentials are expected to be stored in a secret called [`storage-config`](https://github.com/kserve/modelmesh-serving/blob/main/config/default/storage-secret.yaml). This means that the `storage-config` secret can contain a map of several keys that correspond to various credentials.

In the InferenceService metadata, the annotation `serving.kserve.io/secretKey` is used as a placeholder for this needed secret key field.

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
