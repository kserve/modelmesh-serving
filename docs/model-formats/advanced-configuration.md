# Advanced Configuration

ModelMesh Serving abstracts details of the backing runtimes for the supported model
types. For some advanced use-cases, it may be necessary to pass detailed model
configuration parameters through to the runtime. By including a runtime specific
configuration file with the model data, the data and configuration will be
passed to the backing runtime.

## Triton Server

To pass runtime specific configuration through to the Triton Inference Server,
include a non-empty `config.pbtxt` file with the model data and organize the
files into the Triton model repository layout:

```
<storage-path>/
├── config.pbtxt
└── <version>/
    └── <model-data>
```

where `<version>` is a numeric directory name.

For details on the contents of `config.pbtxt`, refer to
[Triton's Model Configuration documentation](https://github.com/triton-inference-server/server/blob/r21.05/docs/model_configuration.md).

For details on Triton's Model Repository structure, refer to
[Triton's Model Repository documentation](https://github.com/triton-inference-server/server/blob/r21.05/docs/model_repository.md).

---

**Note**

- The model's `name` field will be ignored by ModelMesh Serving.
- The native Triton file layout can contain multiple version directories, but
  only one version will be loaded because ModelMesh Serving handles versioning in a
  higher layer.

---

### Batching

Only a simple form of single-request batching is directly exposed in ModelMesh
Serving. This is supported by specifying the batch dimension in the schema with
a variable length of size `-1` and then sending a batch of inputs in a single
infer request. The Triton runtime supports more advanced batching algorithms,
including dynamic and sequence batching
(refer to [Triton's model configuration documentation](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/model_configuration.md#scheduling-and-batching) for details).
Use of these batching algorithms requires inclusion of a `config.pbtxt`, but there
are some caveats when using both the schema and `config.pbtxt` to configure the
`InferenceService` predictor.

In Triton, batching support is indicated with the
[`max_batch_size` model configuration parameter](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/model_configuration.md#maximum-batch-size).
Without any `config.pbtxt` the default value for `max_batch_size` is 0, though
single-request batching is still supported. Note that Triton and ModelMesh Serving
differ in how the batch dimension is handled. In
[Serving's schema](../predictors/schema.md#schema-format),
the batch dimension must be explicit in the input and output shapes. In Triton,
setting `max_batch_size > 0` implicitly changes the input and output shapes
specified in `config.pbtxt`:

> Input and output shapes are specified by a combination of max_batch_size and the dimensions specified by the input or output dims property. For models with max_batch_size greater-than 0, the full shape is formed as [ -1 ] + dims. For models with max_batch_size equal to 0, the full shape is formed as dims.
> ([REF](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/model_configuration.md#inputs-and-outputs))

To support the standard schema with a non-zero `max_batch_size`, Serving will
verify that all inputs and outputs have a batch dimension and remove that
dimension when writing schema information into `config.pbtxt` for Triton.

## MLServer

To pass runtime specific configuration through to MLServer, include a non-empty
`model-settings.json` file with the model data.

```
<storage-path>/
├── model-settings.json
└── <model-data>
```

For details on the specification of this configuration, refer to
[MLServer's Model Settings documentation](https://github.com/SeldonIO/MLServer/blob/1.3.2/docs/reference/model-settings.md).

---

**Note**

The model's `name` field will be overwritten by ModelMesh Serving.

---
