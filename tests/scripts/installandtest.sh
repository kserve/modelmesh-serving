#!/bin/bash

set -x
env | sort >  ${ARTIFACT_DIR}/env.txt
mkdir -p ~/.kube
cp /tmp/kubeconfig ~/.kube/config 2> /dev/null || cp /var/run/secrets/ci.openshift.io/multi-stage/kubeconfig ~/.kube/config
chmod 644 ~/.kube/config
export KUBECONFIG=~/.kube/config
export ARTIFACT_SCREENSHOT_DIR="${ARTIFACT_DIR}/screenshots"

if [ ! -d "${ARTIFACT_SCREENSHOTS_DIR}" ]; then
  echo "Creating the screenshot artifact directory: ${ARTIFACT_SCREENSHOT_DIR}"
  mkdir -p ${ARTIFACT_SCREENSHOT_DIR}
fi

TESTS_REGEX=${TESTS_REGEX:-"basictests"}
ODHPROJECT=${ODHPROJECT:-"opendatahub"}
export ODHPROJECT

echo "OCP version info"
echo `oc version`

if [ -z "${SKIP_INSTALL}" ]; then
    # This is needed to avoid `oc status` failing inside openshift-ci
    oc new-project ${ODHPROJECT}
    $HOME/peak/install.sh
    echo "Sleeping for 5 min to let the KfDef install settle"
    sleep 5m
    # Save the list of events and pods that are running prior to the test run
    oc get events --sort-by='{.lastTimestamp}' > ${ARTIFACT_DIR}/pretest-${ODHPROJECT}.events.txt
    oc get pods -o yaml -n ${ODHPROJECT} > ${ARTIFACT_DIR}/pretest-${ODHPROJECT}.pods.yaml
    oc get pods -o yaml -n openshift-operators > ${ARTIFACT_DIR}/pretest-openshift-operators.pods.yaml
fi

success=1
$HOME/peak/run.sh ${TESTS_REGEX}

if  [ "$?" -ne 0 ]; then
    echo "The tests failed"
    success=0
fi

echo "Saving the dump of the pods logs in the artifacts directory"
oc get pods -o yaml -n ${ODHPROJECT} > ${ARTIFACT_DIR}/${ODHPROJECT}.pods.yaml
oc get pods -o yaml -n openshift-operators > ${ARTIFACT_DIR}/openshift-operators.pods.yaml
echo "Saving the events in the artifacts directory"
oc get events --sort-by='{.lastTimestamp}' > ${ARTIFACT_DIR}/${ODHPROJECT}.events.txt
echo "Saving the logs from the opendatahub-operator pod in the artifacts directory"
oc logs -n openshift-operators $(oc get pods -n openshift-operators -l name=opendatahub-operator -o jsonpath="{$.items[*].metadata.name}") > ${ARTIFACT_DIR}/opendatahub-operator.log 2> /dev/null || echo "No logs for openshift-operators/opendatahub-operator"

if [ "$success" -ne 1 ]; then
    exit 1
fi



## Debugging pause...uncomment below to be able to poke around the test pod post-test
# echo "Debugging pause for 3 hours"
#sleep 180m
