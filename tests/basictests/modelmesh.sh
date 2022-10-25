#!/bin/bash

source $TEST_DIR/common

MY_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

source ${MY_DIR}/../util
RESOURCEDIR="${MY_DIR}/../resources"

MODEL_PROJECT="${ODHPROJECT}-model"
PREDICTOR_NAME="example-onnx-mnist"


os::test::junit::declare_suite_start "$MY_SCRIPT"

function check_resources() {
    header "Testing modelmesh controller installation"
    os::cmd::expect_success "oc project ${ODHPROJECT}"
    os::cmd::try_until_text "oc get deployment modelmesh-controller" "modelmesh-controller" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::try_until_text "oc get pods -l control-plane=modelmesh-controller --field-selector='status.phase=Running' -o jsonpath='{$.items[*].metadata.name}' | wc -w" "1" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::try_until_text "oc get service modelmesh-serving" "modelmesh-serving" $odhdefaulttimeout $odhdefaultinterval
}

function setup_test_serving_namespace() {
    oc new-project ${MODEL_PROJECT}
    header "Setting up test modelmesh serving in ${MODEL_PROJECT}"
    SECRETKEY=$(openssl rand -hex 32)
    sed -i "s/<secretkey>/$SECRETKEY/g" ${RESOURCEDIR}/modelmesh/sample-minio.yaml
    os::cmd::expect_success "oc apply -f ${RESOURCEDIR}/modelmesh/sample-minio.yaml -n ${MODEL_PROJECT}"
    os::cmd::try_until_text "oc get pods -n ${MODEL_PROJECT} -l app=minio --field-selector='status.phase=Running' -o jsonpath='{$.items[*].metadata.name}' | wc -w" "1" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::expect_success "oc apply -f ${RESOURCEDIR}/modelmesh/triton-inference-service.yaml -n ${MODEL_PROJECT}"
    os::cmd::expect_success "oc apply -f ${RESOURCEDIR}/modelmesh/serving-runtime.yaml -n ${MODEL_PROJECT}"
    os::cmd::try_until_text "oc get pods -n ${ODHPROJECT} -l app=odh-model-controller --field-selector='status.phase=Running' -o jsonpath='{$.items[*].metadata.name}' | wc -w" "3" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::try_until_text "oc get pods -n ${MODEL_PROJECT} -l name=modelmesh-serving-triton-2.x --field-selector='status.phase=Running' -o jsonpath='{$.items[*].metadata.name}' | wc -w" "2" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::try_until_text "oc get inferenceservice -n ${MODEL_PROJECT} ${PREDICTOR_NAME} -o jsonpath='{$.status.modelStatus.states.activeModelState}'" "Loaded" $odhdefaulttimeout $odhdefaultinterval
    oc project ${ODHPROJECT}
}

function teardown_test_serving() {
    header "Tearing down test modelmesh serving"
    oc project ${MODEL_PROJECT}
    os::cmd::expect_success "oc delete -f ${RESOURCEDIR}/modelmesh/sample-minio.yaml"
    os::cmd::try_until_text "oc get pods -l app=minio --field-selector='status.phase=Running' -o jsonpath='{$.items[*].metadata.name}' | wc -w" "0" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::expect_success "oc delete -f ${RESOURCEDIR}/modelmesh/triton-inference-service.yaml "
    os::cmd::try_until_text "oc get pods -l name=modelmesh-serving-triton-2.x --field-selector='status.phase=Running' -o jsonpath='{$.items[*].metadata.name}' | wc -w" "0" $odhdefaulttimeout $odhdefaultinterval
    os::cmd::expect_success "oc delete project ${MODEL_PROJECT}"
    oc project ${ODHPROJECT}
}

function test_inferences() {
    header "Testing inference from example mnist model"
    oc project ${MODEL_PROJECT}
    route=$(oc get route ${PREDICTOR_NAME} --template={{.spec.host}})
    # The result of the inference call should give us back an "8" for the data part of the output
    os::cmd::try_until_text "curl -X POST -k ${route}/infer -d '{\"inputs\": [{ \"name\": \"predict\", \"shape\": [1, 64], \"datatype\": \"FP32\", \"data\": [0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0]}]}' | jq '.outputs[0].data[0]'" "8" $odhdefaulttimeout $odhdefaultinterval
    oc project ${ODHPROJECT}
}

function setup_monitoring() {
    header "Enabling User Workload Monitoring on the cluster"
    oc apply -f ${RESOURCEDIR}/modelmesh/enable-uwm.yaml
}

function test_metrics() {
    header "Checking metrics for total models loaded, should be 1 since we have 1 model being served"
    monitoring_token=$(oc sa get-token prometheus-k8s -n openshift-monitoring)
    os::cmd::try_until_text "oc -n openshift-monitoring exec -c prometheus prometheus-k8s-0 -- curl -k -H \"Authorization: Bearer $monitoring_token\" 'https://thanos-querier.openshift-monitoring.svc:9091/api/v1/query?query=modelmesh_models_loaded_total' | jq '.data.result[0].value[1]'" "1" $odhdefaulttimeout $odhdefaultinterval
}

setup_monitoring
check_resources
setup_test_serving_namespace
test_inferences
test_metrics
teardown_test_serving


os::test::junit::declare_suite_end
