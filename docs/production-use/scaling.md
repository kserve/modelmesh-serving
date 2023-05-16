# Scaling

The main axis of scaling ModelMesh Serving is horizontal scaling of the number of replicas in the `Deployments` generated based on the `ServingRuntime`s. By default, the number of replicas for these deployments is controlled by the `podsPerRuntime` configuration. See the [Configuration](../configuration) page for details on how to change this from the default value.

If there is a need to scale runtimes independently, the `ServingRuntime` spec includes a `replicas` field that will override the value set by `podsPerRuntime`.

Increasing the number of runtime replicas has two important effects:

1. increases the amount of memory available to the model-mesh which allows it to have more predictors loaded and available for inference
2. increases the maximum number of instances of a given predictor that can be used to serve requests (a predictor is limited to one instance per runtime replica)

### Scale to Zero

If a given `ServingRuntime` has no `InferenceService`s that it supports, the `Deployment` for that runtime can safely be scaled to 0 replicas to save on resources. By enabling `ScaleToZero` in the configuration, ModelMesh Serving will perform this scaling automatically. If an `InferenceService` is later added that requires the runtime, it will be scaled back up.

To prevent unnecessary churn, the `ScaleToZero` behavior has a grace period that delays scaling down after the last `InferenceService` required by the runtime is deleted. If a new `InferenceService` is created in that window there will be no change to the scale.

### Autoscaler

In addition to the `ScaleToZero` to Zero feature, runtime pods can be autoscaled through HPA. This feature is disabled by default, but it can be enabled at any time by annotating each ServingRuntime/ClusterServingRuntime.
To enable the Autoscaler feature, add the following annotation.

```shell
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  annotations:
    serving.kserve.io/autoscalerClass: hpa
```

Additional annotations:

```shell
metadata:
  annotations:
    serving.kserve.io/autoscalerClass: hpa
    serving.kserve.io/targetUtilizationPercentage: "75"
    serving.kserve.io/metrics: "cpu"
    serving.kserve.io/min-scale: "2"
    serving.kserve.io/max-scale: "3"
```

You can disable the Autoscaler feature even if a runtime pod created based on that ServingRuntime is running.

**NOTE**

- If `serving.kserve.io/autoscalerClass: hpa` is not set, the other annotations will be ignored.
- If `ScaleToZero` is enabled and there are no `InferenceService`s, HPA will be deleted and the ServingRuntime deployment will be scaled down to 0.
