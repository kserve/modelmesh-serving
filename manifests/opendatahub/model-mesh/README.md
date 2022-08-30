# Model Mesh Serving

Model Mesh Serving comes with 1 components:

1. [modelmesh](#modelmesh)

## modelmesh

Contains deployment manifests for the model mesh service.

## Model Mesh Serving Architecture

A complete architecture can be found at https://github.com/kserve/modelmesh-serving

In general, Model Mesh Serving deploys a controller that works on the ServingRuntime and Predictor CRDs.  There are many
supported ServingRuntimes that support different model types.  When a ServingRuntime is created/installed, you can then
create a predictor instance to serve the model described in that predictor.  Briefly, the predictor definition includes
an S3 storage location for that model as well as the credentials to fetch it.  Also included in the predictor definition
is the model type, which is used by the controller to map to the appropriate serving runtime.

The models being served can be reached via both gRPC (natively) and REST (via provided proxy).

### Parameters

None


##### Examples

Example ServingRuntime and Predictors can be found at:  https://github.com/kserve/modelmesh-serving/blob/main/docs/quickstart.md

### Overlays

None

### Installation process

Following are the steps to install Model Mesh as a part of OpenDataHub install:

1. Install the OpenDataHub operator
2. Create a KfDef that includes the model-mesh component with the odh-model-controller overlay.
   
```
apiVersion: kfdef.apps.kubeflow.org/v1
kind: KfDef
metadata:
  name: opendatahub
  namespace: opendatahub
spec:
  applications:
    - kustomizeConfig:
        repoRef:
          name: manifests
          path: odh-common
      name: odh-common
    - kustomizeConfig:
        overlays:
          - odh-model-controller
        repoRef:
          name: manifests
          path: model-mesh
      name: model-mesh
  repos:
    - name: manifests
      uri: https://api.github.com/repos/opendatahub-io/odh-manifests/tarball/master
  version: master

```

3. You can now create a new project and create an InferenceService CR.
