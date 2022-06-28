# Monitoring

ModelMesh Serving monitoring is designed to work with Prometheus and requires Prometheus and optionally Grafana to be available.

The [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) is recommended. Some instructions for setting these up for your cluster can be found in the ModelMesh Performance
repository located [here](https://github.com/kserve/modelmesh-performance/tree/main/docs/monitoring). To learn more about how the Prometheus Operator works, check out their
[Getting Started](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md) guide.

Once Prometheus is configured to scrape the user projects, you can see the ModelMesh Serving metrics in Grafana UI.

### `/metrics` endpoint

Serving runtime pods exposes the endpoint `/metrics` on port `2112` and scheme `https` and the metrics published by each pod are mostly disjoint, for aggregation by the monitoring framework e.g. Grafana. Except for service-wide [metrics](#Metrics) with the scope "Deployment" that are published by only one of the pods at a given time.

`Endpoint`s resource associated with ModelMesh Serving service(default name `modelmesh-serving` but could be different) should be used to track the serving runtime pod IPs from which Prometheus metrics should be scraped.

To override the default metrics port and scheme, you need to add the configuration `metrics.port` and `metrics.scheme` in the main configmap (see [configuration](./configuration/README.md)).

## Service Monitor

A [ServiceMonitor](https://prometheus-operator.dev/docs/operator/design/#servicemonitor) CRD is provided by the Prometheus Operator and is leveraged by ModelMesh for monitoring pods
through the `modelmesh-serving` service. By default, when the ModelMesh controller is started, the existence of this `ServiceMonitor` CRD is checked. If available and `metrics.enabled` is `true`, a `ServiceMonitor` resource
will be created for monitoring `ServingRuntime` pods.

If you have an alternative solution to collect the metrics, you can disable the creation of `ServiceMonitor` by adding the configuration `metrics.disablePrometheusOperatorSupport` set to `true` in the main configmap (see [configuration](./configuration/README.md)).

## Grafana Dashboard

We suggest using Grafana to visualize the Prometheus monitoring data. You can learn more about deploying/configuring both Prometheus and Grafana by checking out [this repo](https://github.com/prometheus-operator/kube-prometheus#quickstart). Also, check out [this page](https://github.com/kserve/modelmesh-performance/blob/main/docs/monitoring/README.md##Setup-Prometheus-Operator) for some tips on how to set it up.

When a Grafana instance is installed and running in the cluster, [this JSON file](https://github.com/kserve/modelmesh-performance/blob/main/docs/monitoring/modelmesh_grafana_dashboard_1634165844916.json) containing our Grafana Dashboard with ModelMesh metrics is suggested to view the metrics below.

## Metrics

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

Note that the request metrics include labels `method` and `code` with the method name and gRPC response code respectively. The code for successful requests is `OK`.

## Troubleshooting

If the ModelMesh Serving metric(s) are missing in the monitoring UIs:

- Check if the Serving Runtime pod is up and running

- Check if the following annotations are configured in the Serving Runtime deployment:

        kubectl describe deployment modelmesh-serving-mlserver-0.x -n $NAMESPACE | grep "prometheus.io"

  Expected output:

        prometheus.io/path: /metrics
        prometheus.io/port: 2112
        prometheus.io/scheme: https
        prometheus.io/scrape: true

  **Note:** Configured `metrics.port` must be listed in the annotation "prometheus.io/port". Default port is 2112.

- Check if the configured `metrics.port` is exposed through service `modelmesh-serving`

        kubectl get svc modelmesh-serving -n $NAMESPACE

  Expected output:

        NAME               TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
        modelmesh-serving  ClusterIP   ..........      <none>        8033/TCP,2112/TCP   ...

  **Note:** Configured `metrics.port` must be listed in the PORT(S). Default port is 2112.

- Check if `ServiceMonitor` resource with name `modelmesh-metrics-monitor` is created

Additional troubleshooting steps can be found [here](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/troubleshooting.md#troubleshooting-servicemonitor-changes).
