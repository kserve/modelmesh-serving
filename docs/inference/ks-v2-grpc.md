# KServe V2 gRPC API

The model runtime server can expose any number of protobuf-based gRPC services on the grpcDataEndpoint to use for inferencing requests. ModelMesh Serving is agnostic to specific service definitions (request/response message content), but for tensor-in/tensor-out based services it is recommended to conform to the KServe V2 dataplane API spec. All of our built-in serving runtimes expose this for their inferencing API.

In the KServe V2 dataplane API, the `ModelInfer` is the only service method of API currently supported to perform inference using the specified model.

```protobuf
//
// Inference Server GRPC endpoints.
//
service GRPCInferenceService
{
   // ...

   // Perform inference using a specific model.
   rpc ModelInfer(ModelInferRequest) returns (ModelInferResponse) {}
}
```

Refer here for more information on the [KServe V2 gRPC API](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#grpc)

Data types of tensor elements are defined in [Tensor Data Types](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#tensor-data-types)

> In all representations tensor data must be flattened to a one-dimensional, row-major order of the tensor elements. Element values must be given in "linear" order without any stride or padding between elements.

([source](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#tensor-data-1))
