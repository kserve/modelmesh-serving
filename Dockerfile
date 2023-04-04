# Copyright 2021 IBM Corporation
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

ARG DEV_IMAGE
ARG BUILDPLATFORM="linux/amd64"

###############################################################################
# Stage 1: Run the go build with go compiler native to the build platform
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
###############################################################################
FROM --platform=${BUILDPLATFORM} ${DEV_IMAGE} AS build

# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETARCH=amd64

LABEL image="build"

# Copy the go sources
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY generated/ generated/
COPY pkg/ pkg/

# Build using native go compiler from BUILDPLATFORM but compiled output for TARGETPLATFORM
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o manager main.go

###############################################################################
# Stage 2: Copy build assets to create the smallest final runtime image
###############################################################################
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.7 AS runtime

ARG USER=2000
ARG IMAGE_VERSION
ARG COMMIT_SHA

LABEL name="modelmesh-serving-controller" \
      version="${IMAGE_VERSION}" \
      release="${COMMIT_SHA}" \
      summary="Kubernetes controller for ModelMesh Serving components" \
      description="Manages lifecycle of ModelMesh Serving Custom Resources and associated Kubernetes resources"

USER ${USER}

WORKDIR /
COPY --from=build /workspace/manager .

COPY config/internal config/internal

ENTRYPOINT ["/manager"]
