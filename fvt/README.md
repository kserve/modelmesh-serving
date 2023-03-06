# Functional Verification Test (FVT) suite

Functional Verification Test (FVT) suite for ModelMesh Serving using [Ginkgo](https://onsi.github.io/ginkgo/).

## How the tests are structured

- The entry points for FVT suite are located in `predictor/predictor_suite_test.go` and `scaleToZero/scaleToZero_suite_test.go`.
- Framework, utility, and helper functions for all suites are in the `fvt` package in this directory.
- Manifests used to create predictors, inference services, and runtimes are in the `testdata` folder.

## How to run the FVT suite

### Prerequisites

- CLIs:
  - [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
  - [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) (v3.2.0+)
- A Kubernetes or OpenShift cluster:
  - Kubernetes version 1.16+
  - Cluster-administrative privileges
  - 12 vCPUs (3-nodes a 4 vCPU, or, 2-nodes a 8 vCPU)
  - 16 GB memory

For more details on cluster sizing, please see [here](/docs/install/README.md#deployed-components)

### Install ModelMesh Serving

The FVTs rely on a set of models existing in a configured `localMinIO` storage. The easiest way to get these models is to use a quick-start install with an instance of MinIO running the `kserve/modelmesh-minio-dev-examples` image.

If starting with a fresh namespace, install ModelMesh Serving configured for the FVTs with:

```Shell
./scripts/install.sh --namespace modelmesh-serving --fvt --dev-mode-logging
```

To re-configure an existing "quickstart" deployment for FVTs, run:

```Shell
kubectl apply -f config/dependencies/fvt.yaml
```

### Development Environment

The FVTs run using the `ginkgo` CLI tool and need `kubectl` configured to communicate
to a Kubernetes cluster. It is recommended to use the containerized development environment
to run the FVTs.

First, verify that you have access to your Kubernetes cluster:

```Shell
kubectl config current-context
```

If you are using an OpenShift cluster, you can run:

```Shell
oc login --token=${YOUR_TOKEN} --server=https://${YOUR_ADDRESS}
```

Then build and start the development environment with:

```Shell
make develop
```

This will drop you in a shell in the development container where the `./kube/config` is mounted to `root` so
that you should be able to communicate to your Kubernetes cluster from inside the container.
If not, you can manually export a functioning `kubectl` configuration and copy it into the container
at `/root/.kube/config` or, for OpenShift, run the `oc login ...` command inside the development
container.

```Shell
# in shell that is has `kubectl` configured with the desired cluster as the
# current context, the following command will print a portable kubeconfig file
kubectl config view --minify --flatten
```

### Run the FVTs

With a suitable development environment and ModelMesh Serving installation as described above,
the FVTs can be executed with a `make` target:

```Shell
make fvt
```

Set the `NAMESPACE` environment variable, if you installed to a **namespace** other than `modelmesh-serving`

```Shell
NAMESPACE="<your-namespace>" make fvt
```

## Enabling or disabling specific tests

Thanks to the Ginkgo framework, we have the ability to run or not run specific tests. See [this doc](https://onsi.github.io/ginkgo/#filtering-specs) for details.
This is useful when you'd like to skip failing tests or want to debug specific test(s).

You can exclude a test by adding `X` or `P` in front of `Describe` or `It`:

```
XDescribe("some behavior", func() { ... })
XIt("some assertion", func() {...})
```

And to run only specific tests, add `F` in front `Describe` or `It`:

```
FDescribe("some behavior", func() { ... })
FIt("some assertion", func() { ... })
```
