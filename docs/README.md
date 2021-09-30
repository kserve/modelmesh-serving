# ModelMesh Serving Documentation

ModelMesh Serving is a Kubernetes-based platform for realtime serving of ML/DL models, optimized for high volume/density use cases. Utilization of available system resources is maximized via intelligent management of in-memory model data across clusters of deployed Pods, based on usage of those models over time.

Leveraging existing third-party model servers, a number of standard ML/DL [model formats](model-types/) are supported out-of-the box with more to follow: TensorFlow, PyTorch ScriptModule, ONNX, scikit-learn, XGBoost, LightGBM. It's also possible to extend with custom runtimes to support arbitrary model formats.

The architecture comprises a controller Pod that orchestrates one or more Kubernetes "model runtime" Deployments which load/serve the models, and a Service that accepts inferencing requests. A routing layer spanning the runtime pods ensures that models are loaded in the right places at the right times and handles forwarding of those requests.

The model data itself is pulled from one or more external [storage instances](predictors/setup-storage.md) which must be configured in a Secret. We currently support only S3-based object storage (self-managed storage is also an option for custom runtimes), but more options will be supported soon.

ModelMesh Serving makes use of two core Kubernetes Custom Resource types:

- `ServingRuntime` - Templates for Pods that can serve one or more particular model formats. There are two "built in" runtimes that cover the out-of-the-box model types, [custom runtimes](runtimes/) can be defined by creating additional ones.
- [`Predictor`](predictors/) - This represents a logical endpoint for serving predictions using a particular model. The Predictor spec specifies the model type, the storage in which it resides and the path to the model within that storage. The corresponding endpoint is "stable" and will seamlessly transition between different model versions or types when the spec is updated.

The Pods that correspond to a particular `ServingRuntime` are started only if/when there are one or more defined `Predictor`s that require them.

We have standardized on the [KServe v2 data plane API](inference/ks-v2-grpc.md) for inferencing, this is supported for all of the built-in model types. Only the gRPC version of this API is supported in this version of ModelMesh Serving, REST support will be coming soon. Custom runtimes are free to use gRPC Service APIs for inferencing, including the KSv2 API.

System-wide configuration parameters can be set by [creating a ConfigMap](configuration/) with name `model-serving-config`.

## Getting Started

To quickly get started with ModelMesh Serving, check out the [Quick Start Guide](quickstart.md).
