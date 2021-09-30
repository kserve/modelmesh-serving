# Known Issues

### Delay in loading the first Predictor assigned to the Triton runtime

Predictors assigned to the Triton runtime may be stuck in the Pending state for some time while the Triton pods are being created. The Triton image is large and may take a while to download.

### Predictors with an unrecognized model type can remain in `Pending` state if there are no other valid Predictors created

Predictors might get stuck in `Pending` state if they do not have recognized model type or explicit runtime assignment and there are no other valid Predictors.

#### Workaround

Ensure that you specify a supported model type and/or runtime name in the `Predictor` CR.
