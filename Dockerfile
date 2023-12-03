# syntax=docker/dockerfile:1.3

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

# NOTE: for syntax, either use "experimental" or "1.3" (or later) to enable multi-arch build with mount option
# see https://hub.docker.com/r/docker/dockerfile (https://github.com/moby/buildkit/releases/tag/dockerfile%2F1.3.0)

###############################################################################
# Stage 1: Run the go build with go compiler native to the build platform
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
###############################################################################
ARG DEV_IMAGE
FROM --platform=$BUILDPLATFORM $DEV_IMAGE AS build

# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
# don't provide "default" values (e.g. 'ARG TARGETARCH=amd64') for non-buildx environments,
# see https://github.com/docker/buildx/issues/510
ARG TARGETOS
ARG TARGETARCH

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
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH:-amd64} \
    CGO_ENABLED=0 \
    GO111MODULE=on \
    go build -a -o manager main.go

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
