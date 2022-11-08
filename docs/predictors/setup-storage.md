# Set up Storage for Loading Models

You will need access to an S3-compatible object storage, for example [MinIO](https://github.com/minio/minio). To provide access to the object storage, use the `storage-config` secret.

## Deploy a model from your own object storage

1. Download sample model or use an existing model

Here we show an example using an [ONNX model for MNIST](https://github.com/onnx/models/raw/ad5c181f1646225f034fba1862233ecb4c262e04/vision/classification/mnist/model/mnist-8.onnx).

2. Add your ONNX saved model to S3-based object storage

A bucket in MinIO needs to be created to copy the model into, which either requires [MinIO Client](https://docs.min.io/docs/minio-client-quickstart-guide.html) or port-forwarding the minio service and logging in using the web interface.

```shell
# Install minio client
$ brew install minio/stable/mc
$ mc --help
NAME:
  mc - MinIO Client for cloud storage and filesystems.
....

# test setup - mc is pre-configured with https://play.min.io, aliased as "play".
# list all buckets in play
$ mc ls play

[2021-06-10 21:04:25 EDT]     0B 2063b651-92a3-4a20-a4a5-03a96e7c5a89/
[2021-06-11 02:40:33 EDT]     0B 5ddfe44282319c500c3a4f9b/
[2021-06-11 05:15:45 EDT]     0B 6dkmmiqcdho1zoloomsj3620cocs6iij/
[2021-06-11 02:39:54 EDT]     0B 9jo5omejcyyr62iizn02ex982eapipjr/
[2021-06-11 02:33:53 EDT]     0B a-test-zip/
[2021-06-11 09:14:28 EDT]     0B aio-ato/
[2021-06-11 09:14:29 EDT]     0B aio-ato-art/
...

# add cloud storage service
$ mc alias set <ALIAS> <YOUR-S3-ENDPOINT> [YOUR-ACCESS-KEY] [YOUR-SECRET-KEY]
# for example if you installed with --quickstart
$ mc alias set myminio http://localhost:9000 EXAMPLE_ACESS_KEY example/secret/EXAMPLEKEY
Added `myminio` successfully.

# create bucket
$ mc mb myminio/models/onnx
Bucket created successfully myminio/models/onnx.

$ mc tree myminio
myminio
└─ models
   └─ onnx

# copy object -- must copy into an existing bucket
$ mc cp ~/Downloads/mnist-8.onnx myminio/models/onnx
...model.lr.zip:  26.45 KiB / 26.45 KiB  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  2.74 MiB/s 0s

$ mc ls myminio/models/onnx
[2021-06-11 11:55:48 EDT]  26KiB mnist-8.onnx
```

3. Add a storage entry to the `storage-config` secret

Ensure there is a key defined in the common `storage-config` secret corresponding to the S3-based storage instance holding your model. The value of this secret key should be JSON like the following, `default_bucket` is optional.

Users can specify use of a custom certificate via the storage config `certificate` parameter. The custom certificate should be in the form of an embedded Certificate Authority (CA) bundle in PEM format.

Using MinIO the JSON contents look like:

```json
{
  "type": "s3",
  "access_key_id": "minioadmin",
  "secret_access_key": "minioadmin/K7JTCMP/EXAMPLEKEY",
  "endpoint_url": "http://127.0.0.1:9000:9000",
  "default_bucket": "",
  "region": "us-east"
}
```

Example secret key contents for GCS and Azure Blob Storage are:

```yaml
gcsKey: |
  {
    "type": "gcs",
    "private_key": "-----BEGIN PRIVATE KEY-----\nAABBCC1122----END PRIVATE KEY-----\n",
    "client_email": "storage-auth@secret-12345.gserviceaccount.com",
    "token_uri": "https://oauth2.googleapis.com/token"
  }
azureKey: |
  {
    "type": "azure",
    "account_name": "az-account",
    "container": "az-container",
    "connection_string": "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=Yabc983f11822334455;EndpointSuffix=core.windows.net"
  }
```

Remember that after updating the storage config secret, there may be a delay of up to 2 minutes until the change is picked up. You should take this into account when creating/updating `InferenceService`s that use storage keys which have just been added or updated - they may fail to load otherwise.
