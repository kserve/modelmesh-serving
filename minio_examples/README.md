# ModelMesh MinIO Examples

This MinIO Docker image contains example models. When ModelMesh is deployed with
the `--quickstart` flag, the example models are deployed via this image.

## Build the image locally

```sh
docker build --target minio-examples -t kserve/modelmesh-minio-examples:latest .
```

To build the `dev` image, use the `--fvt` flag:

```sh
docker build --target minio-fvt -t kserve/modelmesh-minio-dev-examples:latest .
```

## Image usage examples

Start an instance of the image named `modelmesh-minio-examples` locally:

```sh
docker run --rm --name "modelmesh-minio-examples" \
  -u "1000" \
  -p "9000:9000" \
  -e "MINIO_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE" \
  -e "MINIO_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" \
  kserve/modelmesh-minio-examples:latest server /data1
```

Shutdown the "modelmesh-minio-examples" docker container:

```sh
docker stop "modelmesh-minio-examples"
docker rm "modelmesh-minio-examples"
```

### MinIO client usage examples

Install the [MinIO client](https://min.io/docs/minio/linux/reference/minio-mc.html#quickstart), `mc`.

Create an alias `localminio` for an local instance:

```sh
mc alias set localminio http://localhost:9000 AKIAIOSFODNN7EXAMPLE wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

List objects in the instance's bucket:

```sh
mc ls -r localminio/modelmesh-example-models/
```


