# Copyright 2022 IBM Corporation
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

From quay.io/cloudservices/minio:RELEASE.2021-06-17T00-10-46Z.hotfix.35a0912ff
EXPOSE 9000
RUN mkdir -p /data1/modelmesh-example-models
COPY sklearn /data1/modelmesh-example-models/sklearn/
COPY lightgbm /data1/modelmesh-example-models/lightgbm/
COPY onnx /data1/modelmesh-example-models/onnx/
COPY pytorch /data1/modelmesh-example-models/pytorch/
COPY xgboost /data1/modelmesh-example-models/xgboost/
COPY tensorflow /data1/modelmesh-example-models/tensorflow/
COPY keras /data1/modelmesh-example-models/keras/
