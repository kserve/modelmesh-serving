# Serving Runtimes

ModelMesh Serving includes some built-in `ServingRuntime`s for common ML frameworks, but also supports custom runtimes. Custom runtimes are created by building a new container image with support for the desired framework and then creating a `ServingRuntime` custom resource using that image.

If the desired custom runtime uses an ML framework with Python bindings, there is a simplified process to build and integrate a custom cuntime. This approach is detailed in the [Python-based Custom Runtime on MLServer](./mlserver_custom.md) page.

In general, the implementation of a complete runtime requires integration with the Model Mesh API [as detailed on the Custom Runtimes page](./custom_runtimes.md).
