#!/bin/bash

# Need the following env var
# - SERVING_KUBERNETES_CLUSTER_NAME:   kube cluster name
# - SERVING_NS:                        namespace for modelmesh-serving, defulat: modelmesh-serving

# These env vars should come from the pipeline run environment properties
echo "SERVING_KUBERNETES_CLUSTER_NAME=$SERVING_KUBERNETES_CLUSTER_NAME"
echo "SERVING_NS=$SERVING_NS"

retry() {
  local max=$1; shift
  local interval=$1; shift

  until eval "$@"; do
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

kubectl config set-context --current --namespace=${SERVING_NS}

# Use the perofrmance test yaml in modelmesh-performance repo to ensure no performance degradation
kubectl apply -f https://github.com/kserve/modelmesh-performance/raw/main/perf_test/k8s/example-mnist-predictor.yaml
retry 10 6 "kubectl get predictor | grep example-mnist-predictor | grep Loaded"
kubectl apply -f https://github.com/kserve/modelmesh-performance/raw/main/perf_test/k8s/howitzer_k6_test-k8s.yaml
succeeded=0
failed=0
check_interval=3

while [[ $succeeded -eq 0 && $failed -eq 0 ]]
do 
  succeeded=$(kubectl describe job.batch/perf-test-job | grep "Pods Statuses" | tr -s ' '  | cut -d ' ' -f 6)
  failed=$(kubectl describe job.batch/perf-test-job | grep "Pods Statuses" | tr -s ' '  | cut -d ' ' -f 9)
  if [[ $succeeded -gt 0 ]];
  then
    echo "Performance verification passed!"
    podname=$(kubectl get po | grep perf-test-job | tr -s ' '  | cut -d ' ' -f 1)
    kubectl logs $podname
    exit 0
  else
    if [[ $failed -gt 0 ]];
    then
      echo "Performance verification failed..."
      podname=$(kubectl get po | grep perf-test-job | tr -s ' '  | cut -d ' ' -f 1)
      kubectl logs $podname
      exit 1
    else
      echo "Still running, check in $check_interval seconds..."
      sleep $check_interval
    fi
  fi
done
