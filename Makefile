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

# collect args from `make run` so that they don't run twice
ifeq (run,$(firstword $(MAKECMDGOALS)))
  RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  ifneq ("$(wildcard /.dockerenv)","")
    $(error Inside docker container, run 'make $(RUN_ARGS)')
  endif
endif

# Container Engine to be used for building images
ENGINE ?= "docker"

# Image URL to use all building/pushing image targets
IMG ?= kserve/modelmesh-controller:latest
# Namespace to deploy model-serve into
NAMESPACE ?= "model-serving"

CONTROLLER_GEN_VERSION ?= "v0.8.0"

CRD_OPTIONS ?= "crd:maxDescLen=0"

# Model Mesh gRPC API Proto Generation
PROTO_FILES = $(shell find proto/ -iname "*.proto")
GENERATED_GO_FILES = $(shell find generated/ -iname "*.pb.go")

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: manager

# Run unit tests
.PHONY: test
test:
	go test -coverprofile cover.out `go list ./... | grep -v fvt`

# Run fvt tests. This requires an etcd, kubernetes connection, and model serving installation. Ginkgo CLI is used to run them in parallel
.PHONY: fvt
fvt:
	ginkgo -v -procs=2 --progress --fail-fast fvt/predictor fvt/scaleToZero fvt/storage fvt/hpa --timeout=50m


# Command to regenerate the grpc go files from the proto files
.PHONY: fvt-protoc
fvt-protoc:
	rm -rf fvt/generated
	protoc -I=fvt/proto --go_out=plugins=grpc:. --go_opt=module=github.com/kserve/modelmesh-serving $(shell find fvt/proto -iname "*.proto")

.PHONY: fvt-with-deploy
fvt-with-deploy: oc-login deploy-release-dev-mode fvt

.PHONY: oc-login
oc-login:
	oc login --token=${OCP_TOKEN} --server=https://${OCP_ADDRESS} --insecure-skip-tls-verify=true

# Build manager binary
.PHONY: manager
manager: generate fmt
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: start
start: generate fmt manifests
	go run ./main.go

# Install CRDs into a cluster
.PHONY: install
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# artifactory creds set via env var
.PHONY: deploy-release
deploy-release:
	./scripts/install.sh --namespace ${NAMESPACE} --install-config-path config

.PHONY: deploy-release-dev-mode
deploy-release-dev-mode:
	./scripts/install.sh --namespace ${NAMESPACE} --install-config-path config --dev-mode-logging

.PHONY: deploy-release-dev-mode-fvt
deploy-release-dev-mode-fvt:
ifdef MODELMESH_SERVING_IMAGE
	$(eval extra_options += --modelmesh-serving-image ${MODELMESH_SERVING_IMAGE})
endif
ifdef NAMESPACE_SCOPE_MODE
	$(eval extra_options += --namespace-scope-mode)
endif
	./scripts/install.sh --namespace ${NAMESPACE} --install-config-path config --dev-mode-logging --fvt ${extra_options}

.PHONY: delete
delete: oc-login
	./scripts/delete.sh --namespace ${NAMESPACE} --local-config-path config

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
		# NOTE: We're currently copying the CRD manifests from KServe rather than using this target to regenerate those
		# that are common (all apart from predictors) because the formatting ends up different depending on the version
		# of controller-gen and yq used. The KServe make manifests also includes a bunch of yaml post-processing which
		# would need to be replicated here.
		# HACK: ignore errors from generating the TrainedModel CRD from KServe, which is removed below
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role crd paths="github.com/kserve/kserve/pkg/apis/serving/v1alpha1" output:crd:dir=config/crd/bases
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role crd paths="github.com/kserve/kserve/pkg/apis/serving/v1beta1" output:crd:dir=config/crd/bases
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role crd paths="./..." output:crd:dir=config/crd/bases
	rm -f ./config/crd/bases/serving.kserve.io_trainedmodels.yaml
	pre-commit run --all-files prettier > /dev/null || true

# Run go fmt against code
.PHONY: fmt
fmt:
	./scripts/fmt.sh || (echo "Linter failed: $$?"; exit 1)

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="scripts/controller-gen-header.go.tmpl" paths="./..."
	pre-commit run --all-files prettier > /dev/null || true

# Build the final runtime docker image
.PHONY: build
build: build.develop
	./scripts/build_docker.sh --target runtime --engine $(ENGINE)

# Build the develop docker image
.PHONY: build.develop
build.develop:
	./scripts/build_devimage.sh $(ENGINE)

# Start a terminal session in the develop docker container
.PHONY: develop
develop: build.develop
	./scripts/develop.sh

# Run make commands from within the develop docker container
# For example, `make run fmt` will execute `make fmt` within the docker container
.PHONY: run
run: build.develop
	./scripts/develop.sh make $(RUN_ARGS)

# Build the docker image
.PHONY: docker-build
docker-build: build

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION} ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Model Mesh gRPC codegen
.PHONY: mmesh-codegen
mmesh-codegen:
	protoc -I proto/ --go_out=plugins=grpc:generated/ $(PROTO_FILES)

# Check markdown files for invalid links
.PHONY: check-doc-links
check-doc-links:
	@python3 scripts/verify_doc_links.py && echo "$@: OK"

# Override targets if they are included in RUN_ARGs so it doesn't run them twice
$(eval $(RUN_ARGS):;@:)
