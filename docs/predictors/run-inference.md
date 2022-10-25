# Send an inference request to your InferenceService

Currently, only gRPC requests are supported by ModelMesh. However, if the `restProxy` is `enabled` in the ModelMesh Serving [config](../configuration) (which it is by default), then REST inference requests are enabled via [KServe V2 REST proxies](https://github.com/kserve/rest-proxy). This allows sending requests using the KServe V2 REST Predict Protocol to ModelMesh models. However, this proxy does not work in conjunction with custom serving runtimes that expose different gRPC protobuf APIs.

1. [Inference using gRPC](#inference-using-grpc)
2. [Inference using REST](#inference-using-rest)

## Inference using gRPC

### Configure gRPC client

Configure your gRPC client to point to address `modelmesh-serving:8033`, which is based on the kube-dns address and port corresponding to the service. Use the protobuf-based gRPC inference service defined [here](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#grpc) to make inference requests to the model using the `ModelInfer` RPC, setting the name of the `InferenceService` as the `model_name` field in the `ModelInferRequest` message.

Configure the gRPC clients which talk to your service to explicitly use:

- The `round_robin` loadbalancer policy
- A target URI string starting with `dns://` and based on the kube-dns address and port corresponding to the service, for example `dns:///model-mesh-test.modelmesh-serving:8033` where `modelmesh-serving` is the namespace, or just `dns:///model-mesh-test:8033` if the client resides in the same namespace. Note that you end up needing three consecutive `/`'s in total.

Not all languages have built-in support for this but most of the primary ones do. It's recommended to use the latest version of gRPC regardless. Here are some examples for specific languages (note other config such as TLS is omitted):

#### Java

```java
ManagedChannel channel = NettyChannelBuilder.forTarget("modelmesh-serving:8033")
    .defaultLoadBalancingPolicy("round_robin").build();
```

Note that this was done differently in earlier versions of grpc-java - if this does not compile ensure you upgrade.

#### Go

```go
ctx, cancel := context.WithTimeout(context.Background(), 5  * time.Second)
defer cancel()
grpc.DialContext(ctx, "modelmesh-serving:8033", grpc.WithBlock(), grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
```

#### Python

```python
credentials = grpc.ssl_channel_credentials(certificate_data_bytes)
channel_options = (("grpc.lb_policy_name", "round_robin"),)
channel = grpc.secure_channel(target, credentials, options=channel_options)
```

#### NodeJS

Using: https://www.npmjs.com/package/grpc

```javascript
// Read certificate
const cert = readFileSync(sslCertPath);

credentials = grpc.credentials.createSsl(cert);
// For insecure
credentials = grpc.credentials.createInsecure();

// Create client
const clientOptions = {
  "grpc.lb_policy_name": "round_robin",
};
// Get ModelMeshClient from grpc protobuf file
const client = ModelMeshClient(model_mesh_uri, credentials, clientOptions);

// Get rpc prototype for server
const response = await rpcProtoType.call(client, message);
```

### Adjust maximum gRPC payload size

If the gRPC request payloads larger than 16MiB are to be accepted, configure the max message size by setting the `grpcMaxMessageSizeBytes` in the [ConfigMap](../configuration). The default is 16MiB.

However, the max number of bytes for the GRPC request payloads depends on both this setting and adjusting the model serving runtimes' max message limit. For Triton, the message size is effectively uncapped.

### How to access service from outside the cluster without a NodePort

Using [`kubectl port-forward`](https://kubernetes.io/docs/tasks/access-application-cluster/port-forward-access-application-cluster/):

```shell
kubectl port-forward service/modelmesh-serving 8033:8033
```

This assumes you are using port 8033, change the source and/or destination ports as appropriate.

Then change your client target string to localhost:8033, where 8033 is the chosen source port.

### grpcurl Example

Here is an example of how to do this using the command-line based [grpcurl](https://github.com/fullstorydev/grpcurl):

#### 1. Install grpcurl:

```shell
$ grpcurl --version
grpcurl 1.8.1

# If it doesn't exist
$ brew install grpcurl
```

#### 2. Port-forward to access the runtime service:

```shell
# access via localhost:8033
$ kubectl port-forward service/modelmesh-serving 8033
Forwarding from 127.0.0.1:8033 -> 8033
Forwarding from [::1]:8033 -> 8033
```

#### 3. In a separate terminal window, send an inference request using the proto file from `fvt/proto` or one that you have locally:

```shell
$ grpcurl -plaintext -proto fvt/proto/kfs_inference_v2.proto localhost:8033 list
inference.GRPCInferenceService

# run inference
# with below input, expect output to be 8
$ grpcurl \
  -plaintext \
  -proto fvt/proto/kfs_inference_v2.proto \
  -d '{ "model_name": "example-mnist-isvc", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' \
  localhost:8033 \
  inference.GRPCInferenceService.ModelInfer

{
  "modelName": "example-mnist-isvc-725d74f061",
  "outputs": [
    {
      "name": "predict",
      "datatype": "FP32",
      "shape": [
        "1"
      ],
      "contents": {
        "fp32Contents": [
          8
        ]
      }
    }
  ]
}
```

Note that you have to provide the `model_name` in the data load, which is the name of the `InferenceService` deployed.

If a custom serving runtime which doesn't use the KFS V2 API is being used, the `mm-vmodel-id` header must be set to the `InferenceService` name.

If you are sure the requests from your client are being routed in such a way that balances evenly across the cluster (as described [above](#configure-grpc-client)), you should include an additional metadata parameter `mm-balanced = true`. This allows some internal performance optimizations but should not be included if the source if the requests is not properly balanced.

For example adding these headers to the above grpcurl command:

```shell
grpcurl \
  -plaintext \
  -proto fvt/proto/kfs_inference_v2.proto \
  -rpc-header mm-vmodel-id:example-sklearn-mnist-svm \
  -rpc-header mm-balanced:true \
  -d '{ "model_name": "example-sklearn-mnist-svm", "inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "contents": { "fp32_contents": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0] }}]}' \
  localhost:8033 \
  inference.GRPCInferenceService.ModelInfer
```

## Inference using REST

> **Note**: The [REST proxy](https://github.com/kserve/rest-proxy) is currently in an alpha state and may still have issues with certain usage scenarios. When the use case is more performance or resource intensive, consider disabling the REST proxy (it is enabled by default), and using gRPC instead. With the REST proxy enabled, an extra container is deployed in each serving runtime pod which increases resource usage and inference request performance is reduced. See [Configuration](../configuration/README.md) for how to disable/enable the REST inferencing endpoint for ModelMesh `ServingRuntime`s.

By default, REST requests will go through the `modelmesh-serving` service using port `8008`.

Since the service is also headless by default, you can access this service by using [`kubectl port-forward`](https://kubernetes.io/docs/tasks/access-application-cluster/port-forward-access-application-cluster/):

```shell
kubectl port-forward service/modelmesh-serving 8008:8008
```

This assumes you are using port 8008 for REST. Change the source and/or destination ports as appropriate.

REST inference requests can then be sent using the [KServe V2 REST Predict Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md#httprest).
For example, with `curl`, a request can be sent to a model like the following:

```shell
curl -X POST -k http://localhost:8008/v2/models/example-mnist-isvc/infer -d '{"inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "data": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0]}]}'
```

This would give a response similar to the following:

```json
{
  "model_name": "example-mnist-isvc__ksp-7702c1b55a",
  "outputs": [
    {
      "name": "predict",
      "datatype": "FP32",
      "shape": [1],
      "data": [8]
    }
  ]
}
```

### Creating your own service for REST

If you want to use a `LoadBalancer` or `NodePort` service, you can always create another service with the respective type. For example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: modelmesh-rest
spec:
  type: LoadBalancer
  selector:
    modelmesh-service: modelmesh-serving
  ports:
    - name: http
      port: 8008
      protocol: TCP
      targetPort: http
```

Then a sample inference might look like:

```shell
curl -X POST -k http://<external-ip>:8008/v2/models/example-mnist-isvc/infer -d '{"inputs": [{ "name": "predict", "shape": [1, 64], "datatype": "FP32", "data": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0]}]}'
```
