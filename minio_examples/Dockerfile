# Copyright 2023 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Using specific tag for now, there was some reason newer minio versions didn't work
FROM quay.io/cloudservices/minio:RELEASE.2021-06-17T00-10-46Z.hotfix.35a0912ff as minio-examples

EXPOSE 9000

RUN useradd -u 1000 -g 0 && mkdir -p /data1/modelmesh-example-models && chown -R 1000:0 /data1

COPY --chown 1000:0 sklearn /data1/modelmesh-example-models/sklearn/
COPY --chown 1000:0 lightgbm /data1/modelmesh-example-models/lightgbm/
COPY --chown 1000:0 onnx /data1/modelmesh-example-models/onnx/
COPY --chown 1000:0 pytorch /data1/modelmesh-example-models/pytorch/
COPY --chown 1000:0 xgboost /data1/modelmesh-example-models/xgboost/
COPY --chown 1000:0 tensorflow /data1/modelmesh-example-models/tensorflow/
COPY --chown 1000:0 keras /data1/modelmesh-example-models/keras/


# Image with additional models used in the FVTs
FROM minio-examples as minio-fvt

COPY --chown 1000:0 fvt /data1/modelmesh-example-models/fvt/