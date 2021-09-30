# Backup and Restore

This page documents the steps required for backup and restore of an installation of ModelMesh Serving, within overall procedures for backup and restore.

ModelMesh Serving does not have any persistent data that must be maintained during backup and restore, but, to ensure a consistent backup, do not create or modify ModelMesh Serving custom resources during the backup.

#### Controller

Either include these in the backup, or exclude them from the backup and use the installer into the target namespace before running a restore.

#### Custom Resources

Include any created `ServingRuntime` and `Predictor` resources in the backup, and then restore them into the target namespace and cluster. Also include any `ConfigMaps`, `Secrets`, or other resources required by the `ServingRuntimes`.

#### Model Storage

Model data is stored in external storage and should be backed up according to backup and restore procedures for the storage backend.

#### Etcd

The etcd instance used by ModelMesh Serving contains only dynamic runtime information and is not required to be in any backup. All information stored in etcd can be regenerated from the ModelMesh Serving resources. Backing up of the etcd instance is not recommended.

#### Configuration

Include the `model-serving-config` `ConfigMap` and `storage-config` `Secret` in the backup.

### Backup and Restore using Velero

The [`Velero`](https://velero.io/) tool enables to backup and restore of Kubernetes resources and persistent volumes.`Velero` can be used to backup and restore ModelMesh Serving resources.
