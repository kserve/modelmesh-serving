# Installation

## Prerequisites

- **Kubernetes cluster** - A Kubernetes cluster is required. You will need `cluster-admin` authority in order to complete all of the prescribed steps.

- **Kubectl and Kustomize** - The installation will occur via the terminal using [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/).

- **etcd** - ModelMesh Serving requires an [etcd](https://etcd.io/) server in order to coordinate internal state which can be either dedicated or shared. More on this later.

- **S3-compatible object storage** - Before models can be deployed, a remote S3-compatible datastore is needed from which to pull the model data. This could be for example an [IBM Cloud Object Storage](https://www.ibm.com/cloud/object-storage) instance, or a locally running [MinIO](https://github.com/minio/minio) deployment. Note that this is not required to be in place prior to the initial installation.

We provide an install script to quickly run ModelMesh Serving with a provisioned etcd server. This may be useful for experimentation or development but should not be used in production.

The install script has a `--quickstart` option for setting up a self-contained ModelMesh Serving instance. This will deploy and configure local etcd and MinIO servers in the same Kubernetes namespace. Note that this is only for experimentation and/or development use - in particular the connections to these datastores are not secure and the etcd cluster is a single member which is not highly available. Use of `--quickstart` also configures the `storage-config` secret to be able to pull from the [ModelMesh Serving example models bucket](../example-models.md) which contains the model data for the sample Predictors. For complete details on the manfiests applied with `--quickstart` see [config/dependencies/quickstart.yaml](https://github.com/kserve/modelmesh-serving/blob/main/config/dependencies/quickstart.yaml).

## Setup the etcd connection information

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

Download and extract the latest modelmesh-serving from [GitHub](https://github.com/kserve/modelmesh-serving):

```shell
git clone git@github.com:kserve/modelmesh-serving.git
cd modelmesh-serving
```

Run the script to install ModelMesh Serving CRDs, controller, and built-in runtimes into the specified Kubernetes namespaces, after reviewing the command line flags below.

A Kubernetes `--namespace` is required, which must already exist. You must also have cluster-admin authority and cluster access must be configured prior to running the install script.

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
  -p, --install-config-path      Path to local model serve installation configs. Can be ModelMesh Serving tarfile or directory.
  -d, --delete                   Delete any existing instances of ModelMesh Serving in Kube namespace before running install, including CRDs, RBACs, controller, older CRD with serving.kserve.io api group name, etc.
  --quickstart                   Install and configure required supporting datastores in the same namespace (etcd and MinIO) - for experimentation/development

Installs ModelMesh Serving CRDs, controller, and built-in runtimes into specified
Kubernetes namespaces.

Expects cluster-admin authority and Kube cluster access to be configured prior to running.
Also requires Etcd secret 'model-serving-etcd' to be created in namespace already.
```

You can optionally provide a local `--install-config-path` that points to a local ModelMesh Serving tar file or directory containing ModelMesh Serving configs to deploy. If not specified, the `config` directory from the root of the project will be used.

You can also optionally use `--delete` to delete any existing instances of ModelMesh Serving in the designated Kube namespace before running the install.

The installation will create a secret named `storage-config` if it does not already exist. If the `--quickstart` option was chosen, this will be populated with the connection details for the example models bucket in IBM Cloud Object Storage and the local MinIO; otherwise, it will be empty and ready for you to add your own entries.

## Delete the installation

To wipe out the ModelMesh Serving CRDs, controller, and built-in runtimes from the specified Kubernetes namespaces:

```shell
./scripts/delete.sh --namespace modelmesh-serving
```

(Optional) Delete the specified namespace `modelmesh-serving`

```
kubectl delete ns modelmesh-serving
```
