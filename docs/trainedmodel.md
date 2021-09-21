# TrainedModel Deployment

ModelMesh Serving supports deploying models using KFServing's
[TrainedModel interface](https://github.com/kubeflow/kfserving/blob/master/config/crd/serving.kubeflow.org_trainedmodels.yaml).
In KFServing's multi-model serving paradigm, a TrainedModel custom resource represents a machine learning
model that is placed inside a designated InferenceService pod. You can read more
about this [here](https://github.com/kubeflow/kfserving/blob/master/docs/MULTIMODELSERVING_GUIDE.md).

With the ModelMesh backend, no InferenceServices are needed, and instead the models represented by each TrainedModel resource are deployed into ModelMesh. Models served in this setup will be co-located with other models in the model serving containers.

## Enable TrainedModel reconciliation

By default, the ModelMesh controller will not reconcile TrainedModel resources unless the
`trainedmodels.serving.kubeflow.org` CRD is accessible on the cluster. This can cause issues if KFServing
is also installed on you cluster. Currently, it's best to try this on a cluster without the
KFServing controller running to avoid conflicting reconciliation.

To install the TrainedModel CRD, perform the following:

```shell
kubectl apply -f config/crd/kfserving-crd/trainedmodel.yaml
```

If the TrainedModel CRD was installed after the ModelMesh controller was deployed, then you will need to restart the controller to enable watching on TrainedModel resources.

```shell
kubectl rollout restart deploy modelmesh-controller
```

After restarting, give the controller a minute to ready itself.

## Deploy TrainedModel

Try applying an SKLearn MNIST model served from the local MinIO container:

```shell
kubectl apply -f - <<EOF
apiVersion: serving.kubeflow.org/v1alpha1
kind: TrainedModel
metadata:
  name: example-sklearn-tm
  annotations:
    serving.kserve.io/secret-key: localMinIO
spec:
  inferenceService: mlserver-0.x
  model:
    // Use either storageUri or storage spec
    // storageUri: s3://modelmesh-example-models/sklearn/mnist-svm.joblib
    framework: sklearn
    memory: 256Mi
    storage:
      key: localMinIO
      path: sklearn/mnist-svm.joblib
      parameters:
        bucket: modelmesh-example-models
EOF
```

Currently, only S3 based storage is supported. S3 credentials are expected to be stored in a secret called [`storage-config`](https://github.com/kserve/modelmesh-serving/blob/oss-staging/config/default/storage-secret.yaml). This means that the `storage-config` secret can contain a map of several keys that correspond to various credentials.

In the TrainedModel spec, the annotation `serving.kserve.io/secret-key` is used as a placeholder for this needed secret key field.

The `inferenceService` field can be left as an empty string to let ModelMesh Serving handle the model placement into a suitable serving runtime. However, a specific `ServingRuntime` can be passed in if desired.

Currently, the `model.memory` field is not used with the ModelMesh backend, but it is a required field in the TrainedModel CRD. Thus, any arbitrary memory amount can be passed in.

## Check TrainedModel

After deploying, list the TrainedModels to check the status.

```shell
kubectl get tm
```

Depending on if a `ServingRuntime` was already deployed for another model or not, readiness for the newly deployed model may take a bit. Generally, deployment of models into existing `ServingRuntimes` is quick.

```
NAME                 URL                             READY   AGE
example-sklearn-tm   grpc://modelmesh-serving:8033   True    30s
```

## Perform inference

For instructions on how to perform inference on the newly deployed TrainedModel, please refer back to the
[quickstart instructions](./quickstart.md#3-perform-a-grpc-inference-request).

```shell
MODEL_NAME=example-sklearn-tm
grpcurl \
  -plaintext \
  -proto fvt/proto/kfs_inference_v2.proto \
  -d '{ "model_name": "'"${MODEL_NAME}"'", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' \
  localhost:8033 \
  inference.GRPCInferenceService.ModelInfer
```
