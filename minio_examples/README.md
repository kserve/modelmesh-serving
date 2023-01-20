# ModelMesh MinIO Examples

This MinIO Docker image contains example models. When ModelMesh is deployed with
the `--quickstart` flag, the example models are deployed via this image.

## Build the image

From inside the `minio_examples` directory build the docker image:

```sh
docker build --target minio-examples -t kserve/modelmesh-minio-examples:latest .
```

**Note**: When ModelMesh is deployed with the `--fvt` flag then the `modelmesh-minio-dev-examples`
image will be deployed instead. To build it, run the docker build command with the
`minio-fvt` target:

```sh
docker build --target minio-fvt -t kserve/modelmesh-minio-dev-examples:latest .
```

Push the newly built images to DockerHub:

```shell
docker push kserve/modelmesh-minio-examples:latest
docker push kserve/modelmesh-minio-dev-examples:latest
```

## Start the container

Start a "modelmesh-minio-examples" container:

```sh
docker run --rm --name "modelmesh-minio-examples" \
  -u "1000" \
  -p "9000:9000" \
  -e "MINIO_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE" \
  -e "MINIO_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" \
  kserve/modelmesh-minio-examples:latest server /data1
```

## Test the image using the MinIO client

Install the [MinIO client](https://min.io/docs/minio/linux/reference/minio-mc.html#quickstart), `mc`.

Create an alias `localminio` for an local instance:

```sh
mc alias set localminio http://localhost:9000 AKIAIOSFODNN7EXAMPLE wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

List objects in the instance's bucket:

```sh
mc ls -r localminio/modelmesh-example-models/
```

### Stop and remove the docker container

To shut down the "modelmesh-minio-examples" docker container run the following
commands:

```sh
docker stop "modelmesh-minio-examples"
docker rm "modelmesh-minio-examples"
```
