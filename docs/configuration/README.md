# Configuration

System-wide configuration parameters can be set by creating a ConfigMap with name `model-serving-config`. It should contain a single key named `config.yaml`, whose value is a yaml doc containing the configuration. All parameters have defaults and are optional. If the ConfigMap does not exist, all parameters will take their defaults.

The configuration can be updated at runtime and will take effect immediately. Be aware however that certain changes could cause temporary disruption to the service - in particular changing the service name, port, TLS configuration, and/or headlessness, or enabling/disabling the REST inferencing port.

Example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: model-serving-config
data:
  config.yaml: |
    #Sample config overrides
    inferenceServiceName: "modelmesh-serving"
    inferenceServicePort: 8033
    podsPerRuntime: 2
    metrics:
      enabled: true
    restProxy:
      enabled: true
    runtimePodLabels:
      app: myApp
      network-policy: allow-ingress
    runtimePodAnnotations:
      app: myApp
```

The following parameters are currently supported. _Note_ the keys are expressed here in camel case but are in fact case-insensitive.

| Variable                                   | Description                                                                                           | Default                                    |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `inferenceServiceName`                     | The service name which is used for communication with the serving server                              | `modelmesh-serving`                        |
| `inferenceServicePort`                     | The port number for communication with the inferencing service                                        | `8033`                                     |
| `storageSecretName`                        | The secret containing entries for each storage backend from which models can be loaded (\* see below) | `storage-config`                           |
| `podsPerRuntime`                           | Number of server Pods to run per enabled Serving Runtime (\*\* see below)                             | `2`                                        |
| `tls.secretName`                           | Kubernetes TLS type secret to use for securing the Service; no TLS if empty (\*\*\* see below)        |                                            |
| `tls.clientAuth`                           | Enables mutual TLS authentication. Supported values are `require` and `optional`, disabled if empty   |                                            |
| `headlessService`                          | Whether the Service should be headless (recommended)                                                  | `true`                                     |
| `enableAccessLogging`                      | Enables logging of each request to the model server                                                   | `false`                                    |
| `serviceAccountName`                       | The service account to use for runtime Pods                                                           | `modelmesh`                                |
| `metrics.enabled`                          | Enables serving of Prometheus metrics                                                                 | `true`                                     |
| `metrics.port`                             | Port on which to serve metrics via the `/metrics` endpoint                                            | `2112`                                     |
| `metrics.scheme`                           | Scheme to use for the `/metrics` endpoint (`http` or `https`)                                         | `https`                                    |
| `metrics.disablePrometheusOperatorSupport` | Disable the support of Prometheus operator for metrics only if `metrics.enabled` is true              | `false`                                    |
| `scaleToZero.enabled`                      | Whether to scale down `ServingRuntime`s that have no `InferenceService`s                              | `true`                                     |
| `scaleToZero.gracePeriodSeconds`           | The number of seconds to wait after `InferenceService`s are deleted before scaling to zero            | `60`                                       |
| `grpcMaxMessageSizeBytes`                  | The max number of bytes for the gRPC request payloads (\*\*\*\* see below)                            | `16777216` (16MiB)                         |
| `restProxy.enabled`                        | Enables the provided REST proxy container being deployed in each `ServingRuntime` deployment          | `true`                                     |
| `restProxy.port`                           | Port on which the REST proxy to serve REST requests                                                   | `8008`                                     |
| `runtimePodLabels`                         | `metadata.labels` to be added to all `ServingRuntime` pods                                            | (\*\*\*\*\*) See default labels below      |
| `runtimePodAnnotations`                    | `metadata.annotations` to be added to all `ServingRuntime` pods                                       | (\*\*\*\*\*) See default annotations below |

(\*) Currently requires a controller restart to take effect

(\*\*) This parameter will likely be removed in a future release; the Pod replica counts will become more dynamic.

(\*\*\*) The TLS configuration secret allows for keys:

- `tls.crt` - path to TLS secret certificate
- `tls.key` - path to TLS secret key
- `ca.crt` (optional) - single path or comma-separated list of paths to trusted certificates

All certificates must be encoded in PEM PKCS8 format. See the [dedicated page](./tls.md) for more information on TLS configuration.

(\*\*\*\*) The max gRPC request payload size depends on both this setting and adjusting the model serving runtimes' max message limit. See [inference docs](/docs/predictors/run-inference.md) for details.

(\*\*\*\*\*) Default `ServingRuntime` Pod labels and annotations

Labels:

```
app.kubernetes.io/instance:modelmesh-controller
app.kubernetes.io/managed-by=modelmesh-controller
app.kubernetes.io/name=modelmesh-controller
modelmesh-service=modelmesh-serving
name: {{.ServiceName}}-{{.Name}}
```

Annotations:

```
prometheus.io/path: /metrics
prometheus.io/port: 2112
prometheus.io/scheme: https
prometheus.io/scrape: true
```

## Enabling REST inferencing endpoint

REST inferencing support is enabled by default, but it requires slightly larger overall resource allocations due to the current proxy implementation. When enabled, the default port is 8008, and ModelMesh Serving will accept both REST and gRPC inferencing requests.

See the [Deployed Components section](../install/README.md#deployed-components) for more information on the additional CPU and Memory footprint when REST inferencing is enabled.

## Exposing an external endpoint using an OpenShift route

If using OpenShift, you can expose `modelmesh-service` using Openshift routes, which will enable clients to talk to it without using port-forwarding.

1.  Enable HTTP/2 for the Cluster (since gRPC is based on HTTP/2)

        oc annotate ingresses.config/cluster ingress.operator.openshift.io/default-enable-http2=true

2.  Expose the ModelMesh Serving route for external access

        oc create route passthrough modelmesh-serving --service=modelmesh-serving

If using TLS, make sure to include the hostname from the route as described on the [TLS configuration page](./tls.md#creating-tls-certificates-using-certmanager)

**Note**: HTTP/2 uses a fixed port 443, so any calls made to the route exposed above should be to `$HOSTNAME:443`

## Logging

By default, the internal logging of the controller component is set to log stacktraces on errors and sampling, which is the [Zap](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/log/zap#Options) production configuration. To enable the development mode for logging (stacktraces on warnings, no sampling, prettier log outputs), set the environment variable `DEV_MODE_LOGGING=true` on the ModelMesh Serving controller:

```sh
kubectl set env deploy/modelmesh-controller DEV_MODE_LOGGING=true
```
