# Model examples image

This MinIO docker image contains example models. When ModelMesh is deployed with `--quickstart`, example models are shared via this image.

## Build the image locally

```sh
docker build -t docker.io/kserve/modelmesh-minio-examples:latest .
```

Building the dev image for using with `--fvt` flag
```sh
docker build -f Dockerfile.dev -t kserve/modelmesh-minio-dev-examples:latest .
```

## Usage examples

Start an instance of image locally

```sh
docker run -p 9000:9000 -e MINIO_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE -e MINIO_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY docker.io/kserve/modelmesh-minio-examples:latest server /data1
```

After the instance started, create an alias to the instance
```sh
mc alias set localminio http://localhost:9000 AKIAIOSFODNN7EXAMPLE wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

List objects in the bucket in the instance
```sh
mc ls -r localminio/modelmesh-example-models/
```

Instruction to [install MinIO client (mc)](https://min.io/docs/minio/linux/reference/minio-mc.html#quickstart).
