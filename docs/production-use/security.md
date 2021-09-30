# Security Considerations

## Responsibilities of Applications Embedding ModelMesh Serving

ModelMesh Serving is meant to be embedded in other applications which require Machine Learning capabilities. ModelMesh Serving follows best practices for security; however, some aspects of security need to be handled by the embedding application.

Security considerations included in ModelMesh Serving:

- Limited RBAC roles
- Processes run as non-root with limited permissions
- Built-in runtimes listen only on localhost (127.0.0.1)

Security considerations left to the embedding application:

- Restricting the serving images allowed to be used
- Restricting the network traffic
- Securing and encrypting data
- Cluster configuration options

### ServingRuntime Images

ModelMesh Serving supports applications bringing their own runtime for serving models. It is the responsibility of embedding application to completely take care of all the security aspects for these images. The embedding application should restrict the images to a known list of vetted images. This will reduce the possibility of malicious code being executed.

### Network Traffic

Kubernetes [Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) allow restricting incoming and outgoing traffic. In order to restrict the traffic without blocking functionality the usage pattern must be known.

The ModelMesh Serving components usage is fixed and is documented below:

1. The ModelMesh Serving Controller has no incoming connections, but connects to the Kubernetes API Server and establishes watches on the etcd instance ModelMesh Serving is configured to use.

2. All `ServingRuntimes` form a network mesh using `etcd` for coordination. Every runtime pod must be allowed incoming connections from and outgoing connections to any other runtime on the configured inference port. Runtimes must also accept connections from the Controller on port 8033. Additionally runtimes need network access to `etcd` and model storage.

Network Policy Example:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: serving-network-policy
  namespace: NAMESPACE
spec:
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/managed-by: modelmesh-controller
      ports:
        - port: 8033
          protocol: TCP
        - port: 8001
          protocol: TCP
  egress:
    # allow to connect to only etcd and storage IPs
    - to:
        - ipBlock:
            cidr: 9.10.11.12/32
        - ipBlock:
            cidr: 13.12.11.10/32
      ports:
        - protocol: TCP
          port: 2379
        - protocol: TCP
          port: 8081
    # allow to connect to other runtime PODs
    - to:
        - podSelector:
            matchLabels:
              modelmesh-service: modelmesh-serving
  podSelector:
    matchLabels:
      modelmesh-service: modelmesh-serving
  policyTypes:
    - Ingress
    - Egress
```

On the other hand the Serving Image (built by the user) may need any number of incoming and outgoing network connections. To define Network Policies and restrict traffic the embedding application must be able to determine the complete list of valid targets for incoming and outgoing network connections in the Serving Image.

### Securing and Encrypting Data

When used in production, TLS should always be enabled for the incoming inferencing connections, refer to the `tls.*` parameters in [configuration](../configuration).

Ensure that TLS/HTTPS is used for the configured connections to etcd and any configured model storage instances.

Passive Encryption refers to encryption that is implemented within a storage device and outside of the application.
Active Encryption refers to encryption that is implemented by the application.
It is recommended to configure the cluster with cluster wide passive encryption, and add active encryption where applicable.

ModelMesh Serving stores metadata in persistent volumes configured for etcd. Though it does not contain any confidential data, we highly recommend the storage used for creating persistent volumes to be encrypted.

### Cluster Configuration Options

Examples of security related configurations that should be considered when deploying ModelMesh Serving include but are not limited to:

- Resource consumption limits on namespaces
- `PersistentVolume` and `PersistentVolumeClaim` types and configurations
- Permissions to work with Kubernetes resources

## Authentication and Authorization

[Custom Resources (CR)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) is the API used to interact with ModelMesh Serving for `ServingRuntime` and `Predictor` CRs. The API is part of the Kubernetes API Server and access is controlled through [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/). So in order for a user to create/read/update a ModelMesh Serving they need to be authenticated to the Kubernetes cluster and their account must have the necessary RBAC permissions associated.

**Note** : Currently there is no application-level authentication or authorization available for Inference requests performed using [gRPC Inference](../inference/kfs-v2-grpc) APIs, but TLS client authentication can be used (see `tls.clientAuth` in [configuration](../configuration)).

### RBAC

If RBAC is enabled, appropriate access must be granted to manage ModelMesh Serving custom resources. This access is not typically granted by default (refer to the [Kubernetes documentation](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#authentication-authorization-and-auditing) for details.

## Pod Security Context

If running on OpenShift, in a typical user project (namespace) the [OpenShift Security Context](https://docs.openshift.com/container-platform/4.7/authentication/managing-security-context-constraints.html) controls the user and group under which the container runs.

Other container security settings used:

- [drop capabilities](https://kubesec.io/basics/containers-securitycontext-capabilities-drop-index-all/)
- [prevent using the host network](https://kubernetes.io/docs/concepts/policy/pod-security-policy/#host-namespaces)

```yaml
securityContext:
  capabilities:
    drop:
      - ALL
      - KILL
      - MKNOD
      - SETGID
      - SETUID
  runAsUser: 1000620000
```

## Service Account

The ModelMesh components run under separate, dedicated Service Accounts, which have only the RBAC permissions required to function.

| Component  | Service Account        | Role                                       |
| ---------- | ---------------------- | ------------------------------------------ |
| Controller | `modelmesh-controller` | `Role/modelmesh-controller-restricted-scc` |
| Runtimes   | `modelmesh`            | No Roles or access defined                 |

The service accounts for the Controller cannot be changed. However, the service account for runtimes is configurable, refer to `serviceAccountName` in [configuration](../configuration)
