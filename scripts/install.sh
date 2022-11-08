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

# Install ModelMesh Serving CRDs, controller, and built-in runtimes into specified Kubernetes namespaces.
# Expect cluster-admin authority and Kube cluster access to be configured prior to running.

set -Eeuo pipefail

namespace=
install_config_path=
delete=false
dev_mode_logging=false
quickstart=false
fvt=false
user_ns_array=
namespace_scope_mode=false # change to true to run in namespace scope

function showHelp() {
  echo "usage: $0 [flags]"
  echo
  echo "Flags:"
  echo "  -n, --namespace                (required) Kubernetes namespace to deploy ModelMesh Serving to."
  echo "  -p, --install-config-path      Path to installation configs. Can be a local ModelMesh Serving config tarfile/directory or a URL to a config tarfile."
  echo "  -d, --delete                   Delete any existing instances of ModelMesh Serving in Kube namespace before running install, including CRDs, RBACs, controller, older CRD with serving.kserve.io api group name, etc."
  echo "  -u, --user-namespaces          Kubernetes namespaces to enable for ModelMesh Serving"
  echo "  --quickstart                   Install and configure required supporting datastores in the same namespace (etcd and MinIO) - for experimentation/development"
  echo "  --fvt                          Install and configure required supporting datastores in the same namespace (etcd and MinIO) - for development with fvt enabled"
  echo "  -dev, --dev-mode-logging       Enable dev mode logging (stacktraces on warning and no sampling)"
  echo "  --namespace-scope-mode         Run ModelMesh Serving in namespace scope mode"
  echo
  echo "Installs ModelMesh Serving CRDs, controller, and built-in runtimes into specified"
  echo "Kubernetes namespaces."
  echo
  echo "Expects cluster-admin authority and Kube cluster access to be configured prior to running."
  echo "Also requires etcd secret 'model-serving-etcd' to be created in namespace already."
}

die() {
  color_red='\e[31m'
  color_yellow='\e[33m'
  color_reset='\e[0m'
  printf "${color_red}FATAL:${color_yellow} $*${color_reset}\n" 1>&2
  exit 10
}

info() {
  color_blue='\e[34m'
  color_reset='\e[0m'
  printf "${color_blue}$*${color_reset}\n" 1>&2
}

success() {
  color_green='\e[32m'
  color_reset='\e[0m'
  printf "${color_green}$*${color_reset}\n" 1>&2
}

check_pod_status() {
  local -r JSONPATH="{range .items[*]}{'\n'}{@.metadata.name}:{@.status.phase}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}"
  local -r pod_selector="$1"
  local pod_status
  local pod_entry

  pod_status=$(kubectl get pods $pod_selector -o jsonpath="$JSONPATH") || kubectl_exit_code=$? # capture the exit code instead of failing

  if [[ $kubectl_exit_code -ne 0 ]]; then
    # kubectl command failed. print the error then wait and retry
    echo "Error running kubectl command."
    echo $pod_status
    return 1
  elif [[ ${#pod_status} -eq 0 ]]; then
    echo -n "No pods found with selector $pod_selector. Pods may not be up yet."
    return 1
  else
    # split string by newline into array
    IFS=$'\n' read -r -d '' -a pod_status_array <<<"$pod_status"

    for pod_entry in "${pod_status_array[@]}"; do
      local pod=$(echo $pod_entry | cut -d ':' -f1)
      local phase=$(echo $pod_entry | cut -d ':' -f2)
      local conditions=$(echo $pod_entry | cut -d ':' -f3)
      if [ "$phase" != "Running" ] && [ "$phase" != "Succeeded" ]; then
        return 1
      fi
      if [[ $conditions != *"Ready=True"* ]]; then
        return 1
      fi
    done
  fi
  return 0
}

wait_for_pods_ready() {
  local -r JSONPATH="{.items[*]}"
  local -r pod_selector="$1"
  local wait_counter=0
  local kubectl_exit_code=0
  local pod_status

  while true; do
    pod_status=$(kubectl get pods $pod_selector -o jsonpath="$JSONPATH") || kubectl_exit_code=$? # capture the exit code instead of failing

    if [[ $kubectl_exit_code -ne 0 ]]; then
      # kubectl command failed. print the error then wait and retry
      echo $pod_status
      echo -n "Error running kubectl command."
    elif [[ ${#pod_status} -eq 0 ]]; then
      echo -n "No pods found with selector '$pod_selector'. Pods may not be up yet."
    elif check_pod_status "$pod_selector"; then
      echo "All $pod_selector pods are running and ready."
      return
    else
      echo -n "Pods found with selector '$pod_selector' are not ready yet."
    fi

    if [[ $wait_counter -ge 60 ]]; then
      echo
      kubectl get pods $pod_selector
      die "Timed out after $((10 * wait_counter / 60)) minutes waiting for pod with selector: $pod_selector"
    fi

    wait_counter=$((wait_counter + 1))
    echo " Waiting 10 secs..."
    sleep 10
  done
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
  -p | --p | -install-path | --install-path | -install-config-path | --install-config-path)
    shift
    install_config_path="$1"
    ;;
  -d | --d | -delete | --delete)
    delete=true
    ;;
  -dev | --dev | -dev-mode | --dev-mode | -dev-mode-logging | --dev-mode-logging)
    dev_mode_logging=true
    ;;
  --quickstart)
    quickstart=true
    ;;
  --fvt)
    fvt=true
    ;;
  --namespace-scope-mode)
    namespace_scope_mode=true
    ;;
  -*)
    die "Unknown option: '${1}'"
    ;;
  esac
  shift
done

#################      PREREQUISITES      #################
if [[ -z $namespace ]]; then
  showHelp
  die "Kubernetes namespace needs to be set."
fi

# /dev/null will hide output if it exists but show errors if it does not.
if ! type kustomize >/dev/null; then
  die "kustomize is not installed. Go to https://kubectl.docs.kubernetes.io/installation/kustomize/ to install it."
fi

if ! kubectl get namespaces $namespace >/dev/null; then
  die "Kube namespace does not exist: $namespace"
fi

info "Setting kube context to use namespace: $namespace"
kubectl config set-context --current --namespace="$namespace"

info "Getting ModelMesh Serving configs"
if [[ -n $install_config_path ]]; then
  if [[ -f $install_config_path ]] && [[ $install_config_path =~ \.t?gz$ ]]; then
    tar -xf "$install_config_path"
    cd "$(basename "$(basename $install_config_path .tgz)" .tar.gz)"
  elif [[ $install_config_path =~ ^http.+ ]]; then
    if [[ $install_config_path =~ \.t?gz$ ]]; then
      curl -L $install_config_path -O
      filename=$(basename $install_config_path)
      tar -xf "$filename"
      cd "$(basename "$(basename $filename .tgz)" .tar.gz)"
    else
      die "Provided URL should be a .tgz or tar.gz archive file"
    fi
  elif [[ -d $install_config_path ]]; then
    cd "$install_config_path"
  else
    die "Could not find provided path to ModelMesh Serving install configs: $install_config_path"
  fi
else
  echo "Using config directory at root of project."
  cd config
fi

# Ensure the namespace is overridden for all the resources
pushd default
kustomize edit set namespace "$namespace"
popd
pushd rbac/namespace-scope
kustomize edit set namespace "$namespace"
popd
pushd rbac/cluster-scope
kustomize edit set namespace "$namespace"
popd

# Clean up previous instances but do not fail if they do not exist
if [[ $delete == "true" ]]; then
  info "Deleting any previous ModelMesh Serving instances and older CRD with serving.kserve.io api group name"
  kubectl delete crd/predictors.serving.kserve.io --ignore-not-found=true
  kubectl delete crd/servingruntimes.serving.kserve.io --ignore-not-found=true
  kustomize build rbac/namespace-scope | kubectl delete -f - --ignore-not-found=true
  if [[ $namespace_scope_mode != "true" ]]; then
    kubectl delete crd/clusterservingruntimes.serving.kserve.io --ignore-not-found=true
    kustomize build rbac/cluster-scope | kubectl delete -f - --ignore-not-found=true
  fi
  kustomize build default | kubectl delete -f - --ignore-not-found=true
  kubectl delete -f dependencies/quickstart.yaml --ignore-not-found=true
  kubectl delete -f dependencies/fvt.yaml --ignore-not-found=true
fi

# Quickstart resources
if [[ $quickstart == "true" ]]; then
  info "Deploying quickstart resources for etcd and minio"
  kubectl apply -f dependencies/quickstart.yaml

  info "Waiting for dependent pods to be up..."
  wait_for_pods_ready "-l app=etcd"
  wait_for_pods_ready "-l app=minio"
fi

# FVT resources
if [[ $fvt == "true" ]]; then
  info "Deploying fvt resources for etcd and minio"
  kubectl apply -f dependencies/fvt.yaml

  info "Waiting for dependent pods to be up..."
  wait_for_pods_ready "-l app=etcd"
  wait_for_pods_ready "-l app=minio"
fi

if ! kubectl get secret model-serving-etcd >/dev/null; then
  die "Could not find etcd kube secret 'model-serving-etcd'. This is a prerequisite for running ModelMesh Serving install."
else
  echo "model-serving-etcd secret found"
fi

info "Creating storage-config secret if it does not exist"
kubectl create -f default/storage-secret.yaml 2>/dev/null || :
kubectl get secret storage-config

info "Installing ModelMesh Serving RBACs (namespace_scope_mode=$namespace_scope_mode)"
if [[ $namespace_scope_mode == "true" ]]; then
  kustomize build rbac/namespace-scope | kubectl apply -f -
  # We don't install the ClusterServingRuntime CRD when in namespace scope mode, so comment it out first in the CRD manifest file
  sed -i 's/- bases\/serving.kserve.io_clusterservingruntimes.yaml/#- bases\/serving.kserve.io_clusterservingruntimes.yaml/g' crd/kustomization.yaml
else
  kustomize build rbac/cluster-scope | kubectl apply -f -
fi

info "Installing ModelMesh Serving CRDs and controller"
kustomize build default | kubectl apply -f -

if [[ $dev_mode_logging == "true" ]]; then
  info "Enabling development mode logging"
  kubectl set env deploy/modelmesh-controller DEV_MODE_LOGGING=true
fi

if [[ $namespace_scope_mode == "true" ]]; then
  info "Enabling namespace scope mode"
  kubectl set env deploy/modelmesh-controller NAMESPACE_SCOPE=true
  # Reset crd/kustomization.yaml back to CSR crd since we used the same file for namespace scope mode installation 
  sed -i 's/#- bases\/serving.kserve.io_clusterservingruntimes.yaml/- bases\/serving.kserve.io_clusterservingruntimes.yaml/g' crd/kustomization.yaml
fi

info "Waiting for ModelMesh Serving controller pod to be up..."
wait_for_pods_ready "-l control-plane=modelmesh-controller"

# Older versions of kustomize have different load restrictor flag formats.
# Can be removed once Kubeflow installation stops requiring v3.2.
kustomize_version=$(kustomize version --short | grep -o -E "[0-9]\.[0-9]\.[0-9]")
kustomize_load_restrictor_arg="--load-restrictor LoadRestrictionsNone"
if [[ -n "$kustomize_version" && "$kustomize_version" < "3.4.0" ]]; then
    kustomize_load_restrictor_arg="--load_restrictor none"
elif [[ -n "$kustomize_version" && "$kustomize_version" < "4.0.1" ]]; then
    kustomize_load_restrictor_arg="--load_restrictor LoadRestrictionsNone"
fi

info "Installing ModelMesh Serving built-in runtimes"
if [[ $namespace_scope_mode == "true" ]]; then
    kustomize build namespace-runtimes ${kustomize_load_restrictor_arg} | kubectl apply -f -
else
    kustomize build runtimes ${kustomize_load_restrictor_arg} | kubectl apply -f -
fi

if [[ $namespace_scope_mode != "true" ]] && [[ ! -z $user_ns_array ]]; then
  cp dependencies/minio-storage-secret.yaml .
  sed -i.bak "s/controller_namespace/${namespace}/g" minio-storage-secret.yaml

  for user_ns in "${user_ns_array[@]}"; do
    if ! kubectl get namespaces $user_ns >/dev/null; then
      echo "Kube namespace does not exist: $user_ns. Will skip."
    else
      kubectl label namespace ${user_ns} modelmesh-enabled="true" --overwrite
      if ([ $quickstart == "true" ] || [ $fvt == "true" ]); then
        kubectl apply -f minio-storage-secret.yaml -n ${user_ns}
      fi
    fi
  done
  rm minio-storage-secret.yaml
  rm minio-storage-secret.yaml.bak
fi

success "Successfully installed ModelMesh Serving!"
