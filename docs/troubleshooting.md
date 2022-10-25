# Troubleshooting

### InferenceService CR in `FailedtoLoad` state

Check details in the `InferenceService` Model Status `lastFailureInfo`. The message field should indicate the problem or give a clue as to what the problem is.

Otherwise, the location field should have the runtime pod name suffix in which the error occurred. Check the logs of the adapter/puller, model server, and mm containers in this pod

### Inference requests fails

Start with the runtime container logs on `ERROR` responses.

- On connectivity related issue when TLS is enabled:
  - Check URL used on the application
  - Check if TLS is enabled for modelmesh-serving using openssl
  - Check if certificate meant for modelmesh-serving is used
  - Test the connection using openssl with certificate
- If error is “Cache churn threshold exceeded”, you should increase the capacity for the type of model involved by increasing either the number of replicas and/or the model server container's memory allocation for the corresponding `ServingRuntime`.

### InferenceService CR Active Model State: stuck in `Pending` State

Check presence/state of runtime pods. If at least one is running and ready, check the logs of the controller container for errors.
