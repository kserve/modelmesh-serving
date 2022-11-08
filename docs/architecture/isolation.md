# Isolation

ModelMesh Serving exposes two main concepts through the Kubernetes resource API: Serving Runtimes which provide technology specific model serving capabilities and `InferenceService`s which represent the deployment of an individual model.

This guide explains how the associated resources (such as the pods) are created and the isolation concerns which should be considered.

### Serving Runtimes

Serving Runtimes are defined through a resource definition like this one:

```
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: sklearn-0.x
spec:
  supportedModelFormats:
    - name: sklearn
      version: "0"
```

When ModelMesh Serving is deployed, several default runtimes like this are created in the current namespace.

Once these resources are created and the controller processes them, a pod will be created for each runtime, plus additional replicas depending upon the setting. Each of these runtime pods will execute both ModelMesh Serving provided containers like ModelMesh as well as the containers found in the serving runtime spec definition. Due to the isolation of the pod, each runtime will have limited opportunity to influence another runtime directly through local environment. For example, a given runtime cannot access the disk contents including models which are associated with another runtime instance. However, since network policies are not provided out of the box, a serving runtime can attempt a network connection over GRPC or HTTP to another serving runtime.

Although the runtime pod provides some defense, serving runtimes should only be deployed when the associated container images are trusted. In addition, by deploying a Network Policy, the interactions with a given runtime can be more closely controlled.

### InferenceServices

When an `InferenceService` is deployed, it is assigned to an available runtime by evaluating the modelType found in the spec and cross referencing that against the available runtimes. For example, this model is an sklearn model:

```
spec:
  predictor:
    model:
      modelFormat:
        name: sklearn
```

This `InferenceService` would likely be matched against the serving runtime referenced previously. Once assigned to the runtime, the model is subject to loading on demand. A load request would cause the model data to be extracted to the runtime pods local disk, and the server process would be notified by the associated adapter process to load the model data.

Since the same container and pod are processing all of the `InferenceService` predictors with the same model format, there is no pod isolation between `InferenceService`s of a given model format.
