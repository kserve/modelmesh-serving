#!/bin/bash

# Remove the x if you need no print out of each command
set -ex

# Need the following env var
# - SERVING_KUBERNETES_CLUSTER_NAME:   kube cluster name
# - SERVING_NS:                        namespace for modelmesh-serving, defulat: modelmesh-serving

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

# Update kustomize
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
mv kustomize /usr/local/bin/kustomize

# Delete CRDs, controller, and built-in runtimes
./scripts/delete.sh --namespace "$SERVING_NS"

# Also delete kserve InferenceService CRD
kubectl delete -f https://raw.githubusercontent.com/kserve/kserve/master/test/crds/serving.kserve.io_inferenceservices.yaml || true

echo "Finished modelmesh-serving undeployment." 
