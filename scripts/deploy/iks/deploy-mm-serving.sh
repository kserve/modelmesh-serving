#!/bin/bash

# Remove the x if you need no print out of each command
set -ex

# Need the following env
# - SERVING_KUBERNETES_CLUSTER_NAME:   kube cluster name
# - SERVING_NS:                        namespace for modelmesh-serving, defulat: modelmesh-serving

MAX_RETRIES="${MAX_RETRIES:-5}"
SLEEP_TIME="${SLEEP_TIME:-10}"
EXIT_CODE=0

# These env vars should come from the build.properties that `run-setup.sh` generates
echo "BUILD_NUMBER=${BUILD_NUMBER}"
echo "ARCHIVE_DIR=${ARCHIVE_DIR}"
echo "GIT_BRANCH=${GIT_BRANCH}"
echo "GIT_COMMIT=${GIT_COMMIT}"
echo "GIT_COMMIT_SHORT=${GIT_COMMIT_SHORT}"
echo "REGION=${REGION}"
echo "ORG=${ORG}"
echo "SPACE=${SPACE}"
echo "RESOURCE_GROUP=${RESOURCE_GROUP}"

# These env vars should come from the pipeline run environment properties
echo "SERVING_KUBERNETES_CLUSTER_NAME=$SERVING_KUBERNETES_CLUSTER_NAME"
echo "SERVING_NS=$SERVING_NS"

C_DIR="${BASH_SOURCE%/*}"
if [[ ! -d "$C_DIR" ]]; then C_DIR="$PWD"; fi
source "${C_DIR}/helper-functions.sh"
echo "C_DIR=${C_DIR}"

retry() {
  local max=$1; shift
  local interval=$1; shift

  until "$@"; do
    echo "trying.."
    max=$((max-1))
    if [[ "$max" -eq 0 ]]; then
      return 1
    fi
    sleep "$interval"
  done
}

retry 3 3 ibmcloud login --apikey "${IBM_CLOUD_API_KEY}" --no-region
retry 3 3 ibmcloud target -r "$REGION" -o "$ORG" -s "$SPACE" -g "$RESOURCE_GROUP"
retry 3 3 ibmcloud ks cluster config -c "$SERVING_KUBERNETES_CLUSTER_NAME"

kubectl create ns "$SERVING_NS"

wait_for_namespace "$SERVING_NS" "$MAX_RETRIES" "$SLEEP_TIME" || EXIT_CODE=$?

if [[ $EXIT_CODE -ne 0 ]]
then
  echo "Deploy unsuccessful. \"${SERVING_NS}\" not found."
  exit $EXIT_CODE
fi

export USER_NS="modelmesh-user"
kubectl delete ns "$USER_NS" || true
kubectl create ns "$USER_NS"

wait_for_namespace "$USER_NS" "$MAX_RETRIES" "$SLEEP_TIME" || EXIT_CODE=$?

if [[ $EXIT_CODE -ne 0 ]]
then
  echo "Deploy unsuccessful. \"${USER_NS}\" not found."
  exit $EXIT_CODE
fi

# Update kustomize
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
mv kustomize /usr/local/bin/kustomize

# Update target tag and namespace/organization
sed -i.bak 's/newTag:.*$/newTag: '"$GIT_COMMIT_SHORT"'/' config/manager/kustomization.yaml
sed -i.bak 's/newName:.*$/newName: '"$DOCKERSANDBOX_NAMESPACE\/modelmesh-controller"'/' config/manager/kustomization.yaml
rm config/manager/kustomization.yaml.bak

# Install and check if all pods are running - allow 60 retries (10 minutes)
./scripts/install.sh --namespace "$SERVING_NS" -u modelmesh-user --fvt
wait_for_pods "$SERVING_NS" 60 "$SLEEP_TIME" || EXIT_CODE=$?

if [[ $EXIT_CODE -ne 0 ]]
then
  echo "Deploy unsuccessful. Not all pods running."
  exit 1
fi

echo "Finished modelmesh-serving deployment."
