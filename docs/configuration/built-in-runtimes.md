# Customizing Built-In Runtime Pods

ModelMesh Serving currently supports three built-in `ServingRuntime`s:

- Nvidia Triton - onnx, pytorch, tensorflow (model types supported)
- MLServer - lightgbm, sklearn, xgboost
- OpenVINO Model Server OVMS - OpenVINO's Intermediate Representation (IR) format, and onnx models.

When an `InferenceService` using one of these model types is deployed, Pods corresponding to the supporting `ServingRuntime` will be started if they aren't already running. Most of the built-in `ServingRuntime` fields should not be modified, but some can be changed to customize the details of the corresponding Pods:

- `containers[ ].resources` - resource allocation of the model server container.
  - Note that adjusting the resource allocations along with the replicas will affect the model serving capacity and performance. If there is insufficient memory to hold all models, the least recently used ones will not remain loaded, which will impact latency if/when they are subsequently used.
  - Default values are:
    - Nvidia Triton:
      ```yaml
      limits:
        cpu: "5"
        memory: 1Gi
      requests:
        cpu: 500m
        memory: 1Gi
      ```
    - MLServer:
      ```yaml
      limits:
        cpu: "5"
        memory: 1Gi
      requests:
        cpu: 500m
        memory: 1Gi
      ```
    - OVMS:
      ```yaml
      limits:
        cpu: "5"
        memory: 1Gi
      requests:
        cpu: 500m
        memory: 1Gi
      ```
- `replicas` - if not set, the value defaults to the global config parameter `podsPerRuntime` with value of 2.
  - Remember that if [`ScaleToZero`](../production-use/scaling.md#scale-to-zero) is enabled which it is by default, runtimes will have 0 replicas until an `InferenceService` is created that uses that runtime. Once an `InferenceService` is assigned, the runtime pods will scale up to this number.
- `containers[ ].imagePullPolicy` - set to default `IfNotPresent`
- `nodeSelector`
- `affinity`
- `tolerations`

For more details on the fields, see the [spec reference](../runtimes/custom_runtimes.md#spec-attributes).
