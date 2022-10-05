# Development

This document outlines some of the development practices with ModelMesh Serving.

## Setting up development environment

Local Kubernetes clusters can easily be set up using tools like [kind](https://kind.sigs.k8s.io/) and [minikube](https://minikube.sigs.k8s.io/docs/).

For example, using `kind`:

```shell
kind create cluster
```

Then ModelMesh Serving can be installed by using the `fvt` option of the install script:

```shell
kubectl create ns modelmesh-serving
./scripts/install.sh --namespace modelmesh-serving --fvt
```

This installs the `modelmesh-controller` and dependencies in the `modelmesh-serving` namespace. The `minio` pod that this deploys
contains special test images that are used in the functional tests.

If you already deployed ModelMesh Serving on a Kubernetes or OpenShift cluster before and are reconnecting to it now,
make sure to set the default namespace to `modelmesh-serving`.

```shell
kubectl config set-context --current --namespace=modelmesh-serving
```

## Building and updating controller image

If you made changes and want to build an image with your updated changes, simply run:

```shell
make build
```

Then you can tag and push it to a registry of your choice. For information on using local registries with `kind`, check
out their [documentation](https://kind.sigs.k8s.io/examples/kind-with-registry.sh).

```shell
docker tag kserve/modelmesh-controller:latest localhost:5000/modelmesh-controller:latest
docker push localhost:5000/modelmesh-controller:latest
```

To use your image in your local deployment of ModelMesh Serving, run the following:

```shell
kubectl set image deployment/modelmesh-controller manager=localhost:5000/modelmesh-controller:latest
```

This will update the controller image and will rollout a new controller pod. If you make changes to your custom image and re-push it,
you will need to restart the controller pod. This can be done through the following:

```shell
kubectl rollout restart deploy modelmesh-controller
```

## Building the developer image

A dockerized development environment is provided to help set up dependencies for testing, linting, and code generating.
Using this environment is suggested as this is what the GitHub Actions workflows use.
To create the development image, perform the following:

```shell
make build.develop
```

## Using the developer image for linting and testing

To use the dockerized development environment run:

```shell
make develop
```

Then, from inside the developer container, proceed to run the linting, code generation, and testing as described below.

## Formatting and linting code

After building the development image, you can lint and format the code with:

```shell
make run fmt
```

This will run both `golangci-lint` for Go linting and `prettier` for code formatting.
Anytime you make code changes or documentation/markdown changes, this should be run.

## Running unit tests

Similarly, after building the development image, you can run unit tests with:

```shell
make run test
```

## Code generation

If API changes were made, you can run the following to generate utility code and Kubernetes YAML with `controller-gen`.

```shell
make run generate
make run manifests
```

## Running FVTs (functional tests)

Running the functional tests can take a while and requires `kubectl` to be currently pointing to an accessible cluster.

To run them, do the following:

```shell
make fvt
```

**Note**: sometimes the tests can fail on the first run because pulling the serving runtime images can take a while,
causing a timeout. Just try again after the pulling is done.
