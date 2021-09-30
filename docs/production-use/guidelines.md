# Guidelines and Best Practices

## Deployment of ModelMesh Serving

## Considerations

##### Usage Considerations:

- Enable only serving runtimes required for your environment to reduce the footprint of resources
- Memory and/or CPU resource allocations can be reduced (or increased)
  _refer to_ [Deployed Components](../install#deployed-components)

#### Security considerations

##### Secured communication

- We recommend to enable TLS for secured communication between your application and modelmesh-serving, _refer to_ [Configuration](../configuration) on how to enable TLS.

###### Securing and encrypting data

Passive Encryption refers to encryption that is implemented within a storage device and outside of the application. Active Encryption refers to encryption that is implemented by the application. It is recommended to configure the cluster with cluster-wide passive encryption, and add active encryption where applicable.

The etcd which is a key/value data store configured with modelmesh-serving, used for storing internal meta data about the runtimes and predictors. It does not contain any sensitive data other than the meta data, but still you could consider encrypting the backing storage used for etcd cluster.

#### Performance and Scaling considerations

- The number of serving runtime PODs can be adjusted to control footprint/capacity for model deployments.
- Based on the inference requests for a model, number of copies will be increased to accommodate the load and maintain performance. Since there will be at most one copy of each model per deployed Pod, you can increase the number of runtime PODs to achieve a greater maximum request throughput

  _refer to_ `podsPerRuntime` in [Configuration](../configuration)

#### Backup considerations

- _refer to_ [Backup and Restore](backup-and-restore.md)
