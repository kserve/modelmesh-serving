#!/usr/bin/env bash
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
# limitations under the License.#

# Deletes any existing ModelMesh Serving CRDs, controller, and built-in runtimes into specified Kubernetes namespaces.

set -Eeuo pipefail

path_to_configs=config
namespace=

function showHelp() {
  echo "usage: $0 [flags]"
  echo
  echo "Flags:"
  echo "  -p, --local-config-path      Path to local model serve installation configs. Can be ModelMesh Serving tarfile or directory."
  echo "  -n, --namespace              Kubernetes namespace where ModelMesh Serving is deployed."
  echo
  echo "Deletes ModelMesh Serving CRDs, controller, and built-in runtimes into specified"
  echo "Kubernetes namespaces. Will use current Kube namespace and path if"
  echo "one is not given."
  echo
}

die() {
  color_red='\e[31m'
  color_yellow='\e[33m'
  color_reset='\e[0m'
  printf "${color_red}FATAL:${color_yellow} $*${color_reset}\n" 1>&2
  exit 10
}

while (($# > 0)); do
  case "$1" in
  -h | --h | --he | --hel | --help)
    showHelp
    exit 2
    ;;
  -n | --n | -namespace | --namespace)
    shift
    namespace="$1"
    ;;
  -p | --p | -local-path | --local-path | -local-config-path | --local-config-path)
    shift
    path_to_configs="$1"
    ;;
  -*)
    die "Unknown option: '${1}'"
    ;;
  esac
  shift
done

if [[ -n $path_to_configs ]]; then
  cd "$path_to_configs"
fi

if [[ -n $namespace ]]; then
  kubectl config set-context --current --namespace="$namespace"
fi

# Ensure the namespace is overridden for all the resources
cd default
kustomize edit set namespace "$namespace"
cd ..

kustomize build default | kubectl delete -f - --ignore-not-found=true
kustomize build runtimes --load-restrictor LoadRestrictionsNone | kubectl delete -f - --ignore-not-found=true
kubectl delete -f dependencies/quickstart.yaml --ignore-not-found=true
kubectl delete -f dependencies/fvt.yaml --ignore-not-found=true
