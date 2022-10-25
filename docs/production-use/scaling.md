# Scaling

The main axis of scaling ModelMesh Serving is horizontal scaling of the number of replicas in the `Deployments` generated based on the `ServingRuntime`s. By default, the number of replicas for these deployments is controlled by the `podsPerRuntime` configuration. See the [Configuration](../configuration) page for details on how to change this from the default value.

If there is a need to scale runtimes independently, the `ServingRuntime` spec includes a `replicas` field that will override the value set by `podsPerRuntime`.

Increasing the number of runtime replicas has two important effects:

1. increases the amount of memory available to the model-mesh which allows it to have more predictors loaded and available for inference
2. increases the maximum number of instances of a given predictor that can be used to serve requests (a predictor is limited to one instance per runtime replica)

### Scale to Zero

If a given `ServingRuntime` has no `InferenceService`s that it supports, the `Deployment` for that runtime can safely be scaled to 0 replicas to save on resources. By enabling `ScaleToZero` in the configuration, ModelMesh Serving will perform this scaling automatically. If an `InferenceService` is later added that requires the runtime, it will be scaled back up.

To prevent unnecessary churn, the `ScaleToZero` behavior has a grace period that delays scaling down after the last `InferenceService` required by the runtime is deleted. If a new `InferenceService` is created in that window there will be no change to the scale.
