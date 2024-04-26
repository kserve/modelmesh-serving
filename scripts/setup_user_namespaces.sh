#!/usr/bin/env bash
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
# limitations under the License.#

USAGE="$(
  cat <<EOF
Run this script to enable user namespaces for ModelMesh Serving, and optionally add the storage secret
for example models and built-in serving runtimes to the target namespaces.

usage: $0 [flags]
  Flags:
    -u, --user-namespaces         (required) Kubernetes user namespaces to enable for ModelMesh
    -c, --controller-namespace    Kubernetes ModelMesh controller namespace, default is modelmesh-serving
    --create-storage-secret       Create storage secret for example models
    --deploy-serving-runtimes     Deploy built-in serving runtimes
    --dev-mode                    Run in development mode meaning the configs are local, not release based
    -h, --help                    Display this help
EOF
)"

ctrl_ns="modelmesh-serving"
user_ns_array=()
modelmesh_release="v0.12.0"       # The latest release is the default
create_storage_secret=false
deploy_serving_runtimes=false
dev_mode=false                    # Set to true to use locally cloned files instead of from a release

while (($# > 0)); do
  case "$1" in
  -h | --help)
   echo "$USAGE" >&2
   exit 1
    ;;
  -c | --controller-namespace)
    shift
    ctrl_ns="$1"
    ;;
  -u | --user-namespaces)
    shift
    user_ns_array=($1)
    ;;
  --create-storage-secret)
    create_storage_secret=true
    ;;
  --deploy-serving-runtimes)
    deploy_serving_runtimes=true
    ;;
  --dev-mode)
    dev_mode=true
    ;;
  -*)
    die "Unknown option: '${1}'"
    ;;
  esac
  shift
done

if [[ ! -z $user_ns_array ]]; then
  runtime_source="https://github.com/kserve/modelmesh-serving/releases/download/${modelmesh_release}/modelmesh-runtimes.yaml"
  if [[ $dev_mode == "true" ]]; then

    # Older versions of kustomize have different load restrictor flag formats.
    # Can be removed once Kubeflow installation stops requiring v3.2.
    kustomize_load_restrictor_arg=$( kustomize build --help | grep -o -E "\-\-load.restrictor[^,]+" | sed -E "s/(--load.restrictor).+'(.*none)'/\1 \2/I" )

    cp config/dependencies/minio-storage-secret.yaml .
    kustomize build config/runtimes ${kustomize_load_restrictor_arg} > runtimes.yaml
    runtime_source="runtimes.yaml"
  else
    wget https://raw.githubusercontent.com/kserve/modelmesh-serving/${modelmesh_release}/config/dependencies/minio-storage-secret.yaml
  fi
  sed -i.bak "s/controller_namespace/${ctrl_ns}/g" minio-storage-secret.yaml

  for USER_NS in "${user_ns_array[@]}"; do
    if ! kubectl get namespaces $USER_NS >/dev/null; then
      echo "Kube namespace does not exist: $USER_NS. Will skip."
    else
      kubectl label namespace ${USER_NS} modelmesh-enabled="true" --overwrite
      if [[ $create_storage_secret == "true" ]]; then
        kubectl apply -f minio-storage-secret.yaml -n ${USER_NS}
      fi
      if [[ $deploy_serving_runtimes == "true" ]]; then
        kubectl apply -f ${runtime_source} -n ${USER_NS}
      fi
    fi
  done
  rm minio-storage-secret.yaml
  rm minio-storage-secret.yaml.bak
  if [[ $dev_mode == "true" ]]; then
    rm runtimes.yaml
  fi
else
  echo "User namespaces are required, which can be specified as -u \"user1 user2\"."
fi
