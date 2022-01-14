# Python Based Custom Runtime with MLServer

[MLServer](https://github.com/SeldonIO/MLServer) is a Python server that supports
[KServe’s V2 Data Plane](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)
with the goal of providing simple multi-model serving. It contains built in support for some frameworks and also has an extension point for adding additional frameworks. Extending MLServer makes building a Python based custom runtime simpler. MLServer provides the serving interface, you provide the framework, and ModelMesh Serving provides the glue to integrate it as a `ServingRuntime`.

The high-level steps to building a custom Runtime supporting a new framework are:

1. Implement a class that inherits from
   [MLServer's `MLModel` class](https://github.com/SeldonIO/MLServer/blob/0.3.2/mlserver/model.py)
   and implements the `load()` and `predict()` functions.

1. Package the class and all dependencies into a container image that can be executed in a manner compatible with ModelMesh Serving

1. Create a new `ServingRuntime` resource using that image

This page provides templates for each step of the process to use as a reference when building a Python-based custom Serving Runtime.

## Custom MLModel Template

MLServer can be extended by adding new implementations of the `MLModel` class. The two main functions are `load()` and `predict()`. Below is a template implementation of an `MLModel` class in MLServer that includes the suggested structure with TODOs where runtime specific changes will need to be made.

```python
from typing import List

from mlserver import MLModel, types
from mlserver.errors import InferenceError
from mlserver.utils import get_model_uri

# files with these names are searched for and assigned to model_uri with an
# absolute path (instead of using model URI in the model's settings)
# TODO: set wellknown names to support easier local testing
WELLKNOWN_MODEL_FILENAMES = ["model.json", "model.dat"]

class CustomMLModel(MLModel):
    async def load(self) -> bool:
        # get URI to model data
        model_uri = await get_model_uri(self._settings, wellknown_filenames=WELLKNOWN_MODEL_FILENAMES)

        # parse/process file and instantiate the model
        self._load_model_from_file(model_uri)

        # set ready to signal that model is loaded
        self.ready = True
        return self.ready

    async def predict(self, payload: types.InferenceRequest) -> types.InferenceResponse:
        payload = self._check_request(payload)

        return types.InferenceResponse(
            model_name=self.name,
            model_version=self.version,
            outputs=self._predict_outputs(payload),
        )

    def _load_model_from_file(self, file_uri):
        # assume that file_uri is an absolute path
        # TODO: load model from file and instantiate class data
        return

    def _check_request(self, payload: types.InferenceRequest) -> types.InferenceRequest:
        # TODO: validate request: number of inputs, input tensor names/types, etc.
        #   raise InferenceError on error
        return payload

    def _predict_outputs(self, payload: types.InferenceRequest) -> List[types.ResponseOutput]:
        # get inputs from the request
        inputs = payload.inputs

        # TODO: transform inputs into internal data structures
        # TODO: send data through the model's prediction logic

        outputs = []
        # TODO: construct the outputs

        return outputs
```

## Runtime Image Template

Given an `MLModel` implementation, we need to package it and all of its dependencies, including MLServer, into an image that supports being ran as a ModelMesh Serving `ServingRuntime`. There are a variety of ways to build such an image and there may be different requirements on the image. The below snippet shows the set of directives that could be in the `Dockerfile` to make the image compatible with ModelMesh Serving.

---

**Note**

The below snippet assumes the custom model module is called `custom_model.py` and the class is `CustomMLModel`. Make changes accordingly for the actual implementation.

---

```dockerfile
# TODO: choose appropriate base image, install Python, MLServer, and
# dependencies of your MLModel implementation
# FROM ...
# ...
# RUN pip install mlserver
# ...

# The custom `MLModel` implementation should be on the Python search path
# instead of relying on the working directory of the image. If using a
# single-file module, this can be accomplished with:
COPY --chown=${USER} ./custom_model.py /opt/custom_model.py
ENV PYTHONPATH=/opt/

# environment variables to be compatible with ModelMesh Serving
#  these can also be set in the ServingRuntime, but this is recommended for
#  consistency when building and testing
ENV MLSERVER_MODELS_DIR=/models/_mlserver_models \
    MLSERVER_GRPC_PORT=8001 \
    MLSERVER_HTTP_PORT=8002 \
    MLSERVER_LOAD_MODELS_AT_STARTUP=false \
    MLSERVER_MODEL_NAME=dummy-model

# With this setting, the implementation field is not required in the model
# settings which eases integration by allowing the built-in adapter to generate
# a basic model settings file
ENV MLSERVER_MODEL_IMPLEMENTATION=custom_model.CustomMLModel

RUN mlserver start $MLSERVER_MODELS_DIR
```

### Build on ModelMesh Serving MLServer Image

## Custom ServingRuntime Template

With a built container image containing MLServer, the custom runtime, and all required dependencies, the following template shows how to create a `ServingRuntime` using the image.

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: {{MODEL_TYPE}}
spec:
  supportedModelFormats:
    - name: {{MODEL_TYPE}}
      autoSelect: true
  builtInAdapter:
    memBufferBytes: 134217728
    modelLoadingTimeoutMillis: 90000
    runtimeManagementPort: 8001
    serverType: mlserver
  multiModel: true
  grpcDataEndpoint: port:8001
  grpcEndpoint: port:8085
  containers:
    - name: mlserver
      image: {{CUSTOM_RUNTIME_MLSERVER_IMAGE}}
      env:
        - name: MLSERVER_MODELS_DIR
          value: /models/_mlserver_models/
        - name: MLSERVER_GRPC_PORT
          value: "8001"
        - name: MLSERVER_HTTP_PORT
          value: "8002"
        - name: MLSERVER_LOAD_MODELS_AT_STARTUP
          value: "false"
        - name: MLSERVER_MODEL_NAME
          value: dummy-model
        # listen only on localhost
        - name: MLSERVER_HOST
          value: 127.0.0.1
      resources:
        limits:
          cpu: "5"
          memory: 1Gi
        requests:
          cpu: 500m
          memory: 1Gi
```

### Debugging

To enable easier debugging, add the environment variables `MLSERVER_DEBUG` and `MLSERVER_MODEL_PARALLEL_WORKERS` in the `ServingRuntime` as shown below.

```yaml
- name: MLSERVER_DEBUG
  value: "true"
- name: MLSERVER_MODEL_PARALLEL_WORKERS
  value: "0"
```
