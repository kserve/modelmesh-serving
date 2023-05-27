# ModelMesh Metrics

## Overview

Serving runtime pods expose the endpoint `/metrics` on port `2112` and scheme `https` and the metrics published by each pod are mostly disjoint, for aggregation by a monitoring framework (e.g. Grafana), except for [service-wide metrics](#Metrics) with the scope `Deployment`. These are published by only one of the pods at a given time.

Endpoints associated with a ModelMesh Serving service (`modelmesh-serving` by default) should be used to track the serving runtime pods' IPs from which Prometheus metrics should be scraped.

To override the default metrics port and scheme, you can set the variables `metrics.port` and `metrics.scheme` in the [`model-serving-config` configmap](configuration/README.md)).

### Metrics

ModelMesh Serving publishes a variety of metrics related to model request rates and timings, model loading/unloading rates, times and sizes, internal queuing delays, capacity/usage, cache state/LRU, etc. Each serving runtime pod exposes its own metrics that should be aggregated, except for service-wide metrics with the scope "Deployment" that are published by only one of the serving runtime pods at a given time.

Here is the list of metrics exposed by ModelMesh Serving:

| Name                                       | Type   | Scope      | Description                                                              |
| ------------------------------------------ | ------ | ---------- | ------------------------------------------------------------------------ |
| modelmesh_invoke_model_milliseconds        | Timing |            | Internal model server inference request time                             |
| modelmesh_api_request_milliseconds         | Timing |            | External inference request time                                          |
| modelmesh_request_size_bytes               | Size   |            | Inference request payload size                                           |
| modelmesh_response_size_bytes              | Size   |            | Inference response payload size                                          |
| modelmesh_cache_miss_milliseconds          | Timing |            | Cache miss delay                                                         |
| modelmesh_loadmodel_milliseconds           | Timing |            | Time taken to load model                                                 |
| modelmesh_loadmodel_failure                | Count  |            | Model load failures                                                      |
| modelmesh_unloadmodel_milliseconds         | Timing |            | Time taken to unload model                                               |
| modelmesh_unloadmodel_failure              | Count  |            | Unload model failures (not counting multiple attempts for same copy)     |
| modelmesh_unloadmodel_attempt_failure      | Count  |            | Unload model attempt failures                                            |
| modelmesh_req_queue_delay_milliseconds     | Timing |            | Time spent in inference request queue                                    |
| modelmesh_loading_queue_delay_milliseconds | Timing |            | Time spent in model loading queue                                        |
| modelmesh_model_sizing_milliseconds        | Timing |            | Time taken to perform model sizing                                       |
| modelmesh_age_at_eviction_milliseconds     | Age    |            | Time since model was last used when evicted                              |
| modelmesh_loaded_model_size_bytes          | Size   |            | Reported size of loaded model                                            |
| modelmesh_models_loaded_total              | Gauge  | Deployment | Total number of models with at least one loaded copy                     |
| modelmesh_models_with_failure_total        | Gauge  | Deployment | Total number of models with one or more recent load failures             |
| modelmesh_models_managed_total             | Gauge  | Deployment | Total number of models managed                                           |
| modelmesh_instance_lru_seconds             | Gauge  | Pod        | Last used time of least recently used model in pod (in secs since epoch) |
| modelmesh_instance_lru_age_seconds         | Gauge  | Pod        | Last used age of least recently used model in pod (secs ago)             |
| modelmesh_instance_capacity_bytes          | Gauge  | Pod        | Effective model capacity of pod excluding unload buffer                  |
| modelmesh_instance_used_bytes              | Gauge  | Pod        | Amount of capacity currently in use by loaded models                     |
| modelmesh_instance_used_bps                | Gauge  | Pod        | Model capacity utilization in basis points (100ths of percent)           |
| modelmesh_instance_models_total            | Gauge  | Pod        | Number of model copies loaded in pod                                     |

**Note**: The request metrics include labels `method` and `code` with the method name and gRPC response code respectively. The code for successful requests is `OK`.

The best way to visualize the metrics is to use Prometheus to collect them from targets by scraping the metrics HTTP endpoints coupled with a Grafana dashboard. Setup instructions are provided below and involve the following steps:

1. [Set up Prometheus Operator](#set-up-prometheus-operator)
2. [Create the ServiceMonitor CRD](#create-the-servicemonitor-resource)
3. [Import Grafana Dashboard](#import-the-grafana-dashboard)

## Monitoring Setup

### Set up Prometheus Operator

The [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) is the easiest way to set up both Prometheus and Grafana natively in a Kubernetes cluster. You can clone the [`kube-prometheus`](https://github.com/prometheus-operator/kube-prometheus) project and follow the [quickstart](https://github.com/prometheus-operator/kube-prometheus#quickstart) instructions to set it up.
By default, the operator sets RBAC rules to enable monitoring for the `default`, `monitoring`, and `kube-system` namespaces to collect Kubernetes and node metrics.

#### Monitor Additional Namespaces

To monitor the `modelmesh-serving` namespace, in the cloned `kube-prometheus` repository, add the following to `manifests/prometheus-roleBindingSpecificNamespaces.yaml`:

```yaml
- apiVersion: rbac.authorization.k8s.io/v1
  kind: RoleBinding
  metadata:
    labels:
      app.kubernetes.io/component: prometheus
      app.kubernetes.io/name: prometheus
      app.kubernetes.io/part-of: kube-prometheus
      app.kubernetes.io/version: 2.30.2
    name: prometheus-k8s
    namespace: modelmesh-serving
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: Role
    name: prometheus-k8s
  subjects:
    - kind: ServiceAccount
      name: prometheus-k8s
      namespace: monitoring
```

and to `manifests/prometheus-roleSpecificNamespaces.yaml`:

```yaml
- apiVersion: rbac.authorization.k8s.io/v1
  kind: Role
  metadata:
    labels:
      app.kubernetes.io/component: prometheus
      app.kubernetes.io/name: prometheus
      app.kubernetes.io/part-of: kube-prometheus
      app.kubernetes.io/version: 2.30.2
    name: prometheus-k8s
    namespace: modelmesh-serving
  rules:
    - apiGroups:
        - ""
      resources:
        - services
        - endpoints
        - pods
      verbs:
        - get
        - list
        - watch
    - apiGroups:
        - extensions
      resources:
        - ingresses
      verbs:
        - get
        - list
        - watch
    - apiGroups:
        - networking.k8s.io
      resources:
        - ingresses
      verbs:
        - get
        - list
        - watch
```

#### Increase Retention Period

By default, Prometheus only keeps a 24-hour history record. To increase the retention period, modify `manifests/prometheus-prometheus.yaml` by adding:

```yaml
spec:
  ...
  resources:
    requests:
      memory: 400Mi
  # To change the retention period to 7 days, add the line below
  retention: 7d
  ...
```

Other configurable Prometheus specification fields are listed [here](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#prometheusspec).

## Create the ServiceMonitor Resource

[ServiceMonitor](https://prometheus-operator.dev/docs/operator/design/#servicemonitor) is a custom resource definition provided by Prometheus Operator and is leveraged by ModelMesh for monitoring pods through the `modelmesh-serving` service.

Create a `ServiceMonitor` to monitor the `modelmesh-serving` service using the definition found [here](../config/prometheus/servicemonitor.yaml).

```bash
kubectl apply -f servicemonitor.yaml
```

After the `ServiceMonitor` is created, the Prometheus operator will dynamically discover the pods with the label `modelmesh-service: modelmesh-serving` and scrape the metrics endpoint exposed by those pods.

**Note**: By default, when the ModelMesh controller is started, the `ServiceMonitor` is checked. If it exists and `metrics.enabled` is `true`, a `ServiceMonitor` resource
will be created for monitoring `ServingRuntime` pods.

If you have an alternative solution to collect the metrics, you can disable the creation of `ServiceMonitor` by setting the configuration `metrics.disablePrometheusOperatorSupport` to `true` in the [`model-serving-config` configmap](configuration/README.md).

## Import the Grafana Dashboard

To access [Grafana](https://github.com/grafana/grafana) and visualize the Prometheus-monitored data, follow the instructions [here](https://github.com/prometheus-operator/kube-prometheus/blob/main/docs/access-ui.md#grafana) and import the [pre-built dashboard](/config/dashboard/ModelMeshMetricsDashboard.json) we provide using [this guide](https://grafana.com/docs/grafana/latest/dashboards/manage-dashboards/#import-a-dashboard).

## Troubleshooting

If the ModelMesh Serving metric(s) are missing in the monitoring UIs:

- Check if the Serving Runtime pod is up and running.

  - Check if the annotations are configured in the Serving Runtime deployment:

        kubectl describe deployment modelmesh-serving-mlserver-1.x -n $NAMESPACE | grep "prometheus.io"

          Annotations:  prometheus.io/path: /metrics
                        prometheus.io/port: 2112
                        prometheus.io/scheme: https
                        prometheus.io/scrape: true

    **Note:** The configured `metrics.port` must be listed in the annotation `prometheus.io/port`. The default port is `2112`.

- Check if the configured `metrics.port` is exposed through the service `modelmesh-serving`:

        kubectl get svc modelmesh-serving -n $NAMESPACE

        NAME               TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
        modelmesh-serving  ClusterIP   ..........      <none>        8033/TCP,2112/TCP   ...

  **Note:** The configured `metrics.port` must be listed in the annotation `prometheus.io/port`. The default port is `2112`.

- Check if the `ServiceMonitor` resource with name `modelmesh-metrics-monitor` exists.

Additional troubleshooting steps can be found [here](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/troubleshooting.md#troubleshooting-servicemonitor-changes).
