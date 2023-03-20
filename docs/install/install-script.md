# Installation

## Prerequisites

- **Kubernetes cluster** - A Kubernetes cluster is required. You will need
  `cluster-admin` authority in order to complete all of the prescribed steps.

- **Kubectl and Kustomize** - The installation will occur via the terminal using [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/).

- **etcd** - ModelMesh Serving requires an [etcd](https://etcd.io/) server in order to coordinate internal state which can be either dedicated or shared. More on this later.

- **Model storage** - The model files have to be stored in a compatible form of remote storage or on a Kubernetes Persistent Volume. For more information about supported storage options take a look at our [storage setup](/docs/predictors/setup-storage.md) page.

We provide an install script to quickly run ModelMesh Serving with a provisioned etcd server. This may be useful for experimentation or development but should not be used in production.

The install script has a `--quickstart` option for setting up a self-contained ModelMesh Serving instance. This will deploy and configure local etcd and MinIO servers in the same Kubernetes namespace. Note that this is only for experimentation and/or development use - in particular the connections to these datastores are not secure and the etcd cluster is a single member which is not highly available. Use of `--quickstart` also configures the `storage-config` secret to be able to pull from the [ModelMesh Serving example models bucket](../example-models.md) which contains the model data for the sample `InferenceService`s. For complete details on the manifests applied with `--quickstart` see [config/dependencies/quickstart.yaml](https://github.com/kserve/modelmesh-serving/blob/main/config/dependencies/quickstart.yaml).

## Set up the etcd connection information

If the `--quickstart` install option is **not** being used, details of an existing etcd cluster must be specified prior to installation. Otherwise, please skip this step and proceed to [Installation](#installation).

Create a file named etcd-config.json, populating the values based upon your etcd server. The same etcd server can be shared between environments and/or namespaces, but in this case _the `root_prefix` must be set differently in each namespace's respective secret_. The complete json schema for this configuration is documented [here](https://github.com/IBM/etcd-java/blob/master/etcd-json-schema.md).

```json
{
  "endpoints": "https://etcd-service-hostame:2379",
  "userid": "userid",
  "password": "password",
  "root_prefix": "unique-chroot-prefix"
}
```

Then create the secret using the file (note that the key name within the secret must be `etcd_connection`):

```shell
kubectl create secret generic model-serving-etcd --from-file=etcd_connection=etcd-config.json
```

A secret named `model-serving-etcd` will be created and passed to the controller.

## Installation

Install the latest release of [modelmesh-serving](https://github.com/kserve/modelmesh-serving/releases/latest) by first cloning the corresponding release branch:

```shell
RELEASE=release-0.10
git clone -b $RELEASE --depth 1 --single-branch https://github.com/kserve/modelmesh-serving.git
cd modelmesh-serving
```

Run the script to install ModelMesh Serving CRDs, controller, and built-in runtimes into the specified Kubernetes namespaces, after reviewing the command line flags below.

A Kubernetes `--namespace` is required, which must already exist. You must also have cluster-admin authority and cluster access must be configured prior to running the install script.

A list of Kubernetes namespaces `--user-namespaces` is optional to enable user namespaces for ModelMesh Serving. The script will skip the namespaces which don't already exist.

The `--quickstart` option can be specified to install and configure supporting datastores in the same namespace (etcd and MinIO) for experimental/development use. If this is not chosen, the namespace provided must have an Etcd secret named `model-serving-etcd` created which provides access to the Etcd cluster. See the [instructions above](#setup-the-etcd-connection-information) on this step.

```shell
kubectl create namespace modelmesh-serving
./scripts/install.sh --namespace modelmesh-serving --quickstart
```

See the installation help below for detail:

```shell
./scripts/install.sh --help
usage: ./scripts/install.sh [flags]

Flags:
  -n, --namespace                (required) Kubernetes namespace to deploy ModelMesh Serving to.
  -p, --install-config-path      Path to installation configs. Can be a local ModelMesh Serving config tarfile/directory or a URL to a config tarfile.
  -d, --delete                   Delete any existing instances of ModelMesh Serving in Kube namespace before running install, including CRDs, RBACs, controller, older CRD with serving.kserve.io api group name, etc.
  -u, --user-namespaces          Kubernetes namespaces to enable for ModelMesh Serving
  --quickstart                   Install and configure required supporting datastores in the same namespace (etcd and MinIO) - for experimentation/development
  --fvt                          Install and configure required supporting datastores in the same namespace (etcd and MinIO) - for development with fvt enabled
  -dev, --dev-mode-logging       Enable dev mode logging (stacktraces on warning and no sampling)
  --namespace-scope-mode         Run ModelMesh Serving in namespace scope mode

Installs ModelMesh Serving CRDs, controller, and built-in runtimes into specified
Kubernetes namespaces.

Expects cluster-admin authority and Kube cluster access to be configured prior to running.
Also requires etcd secret 'model-serving-etcd' to be created in namespace already.
```

You can optionally provide a local `--install-config-path` that points to a local ModelMesh Serving tar file or directory containing ModelMesh Serving configs to deploy. If not specified, the `config` directory from the root of the project will be used.

You can also optionally use `--delete` to delete any existing instances of ModelMesh Serving in the designated Kube namespace before running the install.

The installation will create a secret named `storage-config` if it does not already exist. If the `--quickstart` option was chosen, this will be populated with the connection details for the example models bucket in IBM Cloud Object Storage and the local MinIO; otherwise, it will be empty and ready for you to add your own entries.

The `--namespace-scope-mode` will deploy `ServingRuntime`s confined to the same namespace, instead of the default cluster-scoped runtimes `ClusterServingRuntime`s. These serving runtimes are accessible to any user/namespace in the cluster.

## Setup additional namespaces

To enable additional namespaces for ModelMesh after the initial installation, you need to add a label named `modelmesh-enabled`, and optionally setup the storage secret `storage-config` and built-in runtimes, in the user namespaces.

The following command will add the label to "your_namespace".

```shell
kubectl label namespace your_namespace modelmesh-enabled="true" --overwrite
```

You can also run a script to setup multiple user namespaces. See the setup help below for detail:

```shell
./scripts/setup_user_namespaces.sh --help
Run this script to enable user namespaces for ModelMesh Serving, and optionally add the storage secret
for example models and built-in serving runtimes to the target namespaces.

usage: ./scripts/setup_user_namespaces.sh [flags]
  Flags:
    -u, --user-namespaces         (required) Kubernetes user namespaces to enable for ModelMesh
    -c, --controller-namespace    Kubernetes ModelMesh controller namespace, default is modelmesh-serving
    --create-storage-secret       Create storage secret for example models
    --deploy-serving-runtimes     Deploy built-in serving runtimes
    --dev-mode                    Run in development mode meaning the configs are local, not release based
    -h, --help                    Display this help
```

The following command will setup two namespaces with the required label, optional storage secret, and built-in runtimes, so you can deploy sample `InferenceService`s into any of them immediately.

```shell
./scripts/setup_user_namespaces.sh -u "ns1 ns2" --create-storage-secret --deploy-serving-runtimes
```

## Delete the installation

To wipe out the ModelMesh Serving CRDs, controller, and built-in runtimes from the specified Kubernetes namespaces:

```shell
./scripts/delete.sh --namespace modelmesh-serving
```

(Optional) Delete the specified namespace `modelmesh-serving`

```
kubectl delete ns modelmesh-serving
```
