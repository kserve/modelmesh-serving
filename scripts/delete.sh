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
user_ns_array=

function showHelp() {
  echo "usage: $0 [flags]"
  echo
  echo "Flags:"
  echo "  -p, --local-config-path      Path to local model serve installation configs. Can be ModelMesh Serving tarfile or directory."
  echo "  -n, --namespace              Kubernetes namespace where ModelMesh Serving is deployed."
  echo "  -u, --user-namespaces        Kubernetes namespaces where ModelMesh Serving is enabled"
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
  -u | --u | -user-namespaces | --user-namespaces)
    shift
    user_ns_array=($1)
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

old_namespace=$(kubectl config  get-contexts $(kubectl config current-context) |tail -1|awk '{ print $5 }')
if [[ ! -n $old_namespace ]]; then
  old_namespace="default"
fi
echo "current namespace: $old_namespace"
if [[ -n $namespace ]]; then
  kubectl config set-context --current --namespace="$namespace"
else
  namespace=$old_namespace
fi

echo "deleting in namespace: $namespace"

# Ensure the namespace is overridden for all the resources
pushd default
kustomize edit set namespace "$namespace"
popd
pushd rbac/namespace-scope
kustomize edit set namespace "$namespace"
popd

# Older versions of kustomize have different load restrictor flag formats.
# Can be removed once Kubeflow installation stops requiring v3.2.
kustomize_version=$(kustomize version --short | grep -o -E "[0-9]\.[0-9]\.[0-9]")
kustomize_load_restrictor_arg="--load-restrictor LoadRestrictionsNone"
if [[ -n "$kustomize_version" && "$kustomize_version" < "3.4.0" ]]; then
    kustomize_load_restrictor_arg="--load_restrictor none"
elif [[ -n "$kustomize_version" && "$kustomize_version" < "4.0.1" ]]; then
    kustomize_load_restrictor_arg="--load_restrictor LoadRestrictionsNone"
fi

if [[ ! -z $user_ns_array ]]; then
  kustomize build runtimes ${kustomize_load_restrictor_arg} > runtimes.yaml
  cp dependencies/minio-storage-secret.yaml .
  sed -i.bak "s/controller_namespace/${namespace}/g" minio-storage-secret.yaml

  for user_ns in "${user_ns_array[@]}"; do
    if ! kubectl get namespaces $user_ns >/dev/null; then
      echo "Kube namespace does not exist: $user_ns. Will skip."
    else
      kubectl label namespace ${user_ns} modelmesh-enabled-
      kubectl delete -f minio-storage-secret.yaml -n ${user_ns}
      kubectl delete -f runtimes.yaml -n ${user_ns}
    fi
  done
  rm minio-storage-secret.yaml
  rm minio-storage-secret.yaml.bak
  rm runtimes.yaml
fi

# Determine whether a modelmesh-controller-rolebinding clusterrolebinding exists and is
# associated with the service account in this namespace. If not, don't delete the cluster level RBAC.
set +e
crb_ns=$(kubectl get clusterrolebinding modelmesh-controller-rolebinding -o json | jq -r .subjects[0].namespace)
set -e
if [[ "$crb_ns" == "$namespace" ]]; then
  echo "deleting cluster scope RBAC"
  kustomize build rbac/cluster-scope | kubectl delete -f - --ignore-not-found=true
fi
kustomize build default | kubectl delete -f - --ignore-not-found=true
kustomize build rbac/namespace-scope | kubectl delete -f - --ignore-not-found=true
kustomize build runtimes ${kustomize_load_restrictor_arg} | kubectl delete -f - --ignore-not-found=true
kubectl delete -f dependencies/quickstart.yaml --ignore-not-found=true
kubectl delete -f dependencies/fvt.yaml --ignore-not-found=true

# Roll back to previous status
if [[ "$namespace" != "$old_namespace" ]]; then
  kubectl config set-context --current --namespace=${old_namespace}
fi
