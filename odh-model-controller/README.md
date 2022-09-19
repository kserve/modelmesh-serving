# ODH Model Controller

The controller will watch the Predictor custom resource events to
extend the KServe modelmesh-serving controller behavior with the following
capabilities:

- Openshift ingress controller integration.

It has been developed using **Golang** and
**[Kubebuilder](https://book.kubebuilder.io/quick-start.html)**.

## Implementation detail



## Developer docs

Follow the instructions below if you want to extend the controller
functionality:

### Run unit tests

Unit tests have been developed using the [**Kubernetes envtest
framework**](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest).

Run the following command to execute them:

```shell
make test
```

### Run locally

Install the CRD from the [KServe modelmesh-serving](../modelmesh-serving-controller) repository as a requirement.

When running the controller locally, the [admission webhook](./config/webhook)
will be running in your local machine. The requests made by the Openshift API
have to be redirected to the local port.

This will be solved by deploying the [Ktunnel
application](https://github.com/omrikiei/ktunnel) in your cluster instead of the
controller manager, it will create a reverse tunnel between the cluster and your
local machine:

```shell
make deploy-dev -e K8S_NAMESPACE=<YOUR_NAMESPACE>
```

Run the controller locally:

```shell
make run -e K8S_NAMESPACE=<YOUR_NAMESPACE>
```

### Deploy local changes

Build a new image with your local changes and push it to `<YOUR_IMAGE>` (by
default `quay.io/opendatahub/odh-model-controller`).

```shell
make image -e IMG=<YOUR_IMAGE>
```

Deploy the manager using the image in your registry:

```shell
make deploy -e K8S_NAMESPACE=<YOUR_NAMESPACE> -e IMG=<YOUR_IMAGE>
```