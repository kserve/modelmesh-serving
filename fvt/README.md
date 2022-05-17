# Functional Verification Test (FVT) suite

Functional Verification Test (FVT) suite for ModelMesh Serving using [Ginkgo](https://onsi.github.io/ginkgo/).

## How the tests are structured

- The entry points for FVT suite are located in `predictor/predictor_suite_test.go` and `scaleToZero/scaleToZero_suite_test.go`.
- Framework, utility, and helper functions for all suites are in the `fvt` package in this directory.
- Manifests used to create predictors, inference services, and runtimes are in the `testdata` folder.

## How to run the FVT suite

### Install ModelMesh Serving

The FVTs rely on a set of models existing in a configured `localMinIO` storage. The easiest way to get these models is to use a quick-start install with an instance of MinIO running the `kserve/modelmesh-minio-dev-examples` image.

If starting with a fresh namespace, install ModelMesh Serving configured for the FVTs with:

```
./scripts/install.sh --namespace modelmesh-serving --fvt --dev-mode-logging
```

To re-configure an existing quick-start instance for FVTs, run:

```
kubectl apply -f dependencies/fvt.yaml
```

### Development Environment

The FVTs run using the `ginkgo` CLI tool and need `kubectl` configuration to the cluster. It is recommended to use the containerized development environment to run the FVTs. First build the environment with:

```
make develop
```

This will drop you in a shell in the container. The next step is to configure this environment to communicate to the Kubernetes cluster. If using an OpenShift cluster, you can run:

```
oc login --token=${YOUR_TOKEN} --server=https://${YOUR_ADDRESS}
```

Another method is to can export a functioning `kubectl` configuration and copy it into the container at `/root/.kube/config`.

```
# in shell that is has `kubectl` configured with the desired cluster as the
# current context, the following command will print a portable kubeconfig file
kubectl config view --minify --flatten
```

### Run the FVTs

With a suitable development environment and ModelMesh Serving installation as described above, FVTs can be executed with a `make` target:

```
make fvt
# use the command below if you installed to a namespace other than modelmesh-serving
# NAMESPACE=your-namespace make fvt`
```

## Running or not running specific tests

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
