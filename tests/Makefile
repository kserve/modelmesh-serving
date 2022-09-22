IMAGE=modelmesh-serving-tests
GIT_ORG=opendatahub-io
GIT_BRANCH=master
ODHPROJECT=opendatahub
# Specify the repo and git ref/branch to use for cloning ods-ci
ODS_CI_REPO=https://github.com/red-hat-data-services/ods-ci
ODS_CI_GITREF=master
OC_CLI_URL=https://mirror.openshift.com/pub/openshift-v4/amd64/clients/ocp/latest/openshift-client-linux.tar.gz
OPENSHIFT_USER=
OPENSHIFT_PASS=
OPENSHIFT_LOGIN_PROVIDER=
# Setting SKIP_INSTALL will let you run the tests against an ODH instance that is already setup
SKIP_INSTALL=
# Setting TESTS_REGEX will allow you to change which tests are going to be run
TESTS_REGEX=
# Location inside the container where CI system will retrieve files after a test run
ARTIFACT_DIR=/tmp/artifacts
LOCAL_ARTIFACT_DIR="${PWD}/artifacts"

all: test
test: build run clean

build:
	podman build -t $(IMAGE) --build-arg ORG=$(GIT_ORG) --build-arg BRANCH=$(GIT_BRANCH) --build-arg ODS_CI_REPO=$(ODS_CI_REPO) --build-arg ODS_CI_GITREF=$(ODS_CI_GITREF) --build-arg OC_CLI_URL=$(OC_CLI_URL) .

run:
	# Confirm that we have a directory for storing any screenshots from selenium tests
	mkdir -p ${LOCAL_ARTIFACT_DIR}/screenshots
	oc config view --flatten --minify > /tmp/tests-kubeconfig
	podman run -e SKIP_INSTALL=$(SKIP_INSTALL) -e TESTS_REGEX=$(TESTS_REGEX) -e SKIP_OPERATOR_INSTALL=$(SKIP_OPERATOR_INSTALL) \
	    -e SKIP_KFDEF_INSTALL=$(SKIP_KFDEF_INSTALL) -e ODHPROJECT=$(ODHPROJECT) \
		-e OPENSHIFT_USER="$(OPENSHIFT_USER)" -e OPENSHIFT_PASS="$(OPENSHIFT_PASS)" -e OPENSHIFT_LOGIN_PROVIDER=$(OPENSHIFT_LOGIN_PROVIDER) -e ARTIFACT_DIR=$(ARTIFACT_DIR) \
		-it -v ${LOCAL_ARTIFACT_DIR}/:$(ARTIFACT_DIR):z -v /tmp/tests-kubeconfig:/tmp/kubeconfig:z $(IMAGE)

clean:
	oc delete -n $(ODHPROJECT) kfdef opendatahub || true
	oc delete project $(ODHPROJECT) || echo -e "\n\n==> If the project deletion failed, you can try to use this script to force it: https://raw.githubusercontent.com/jefferyb/useful-scripts/master/openshift/force-delete-openshift-project\n\n"
	#Clean up openshift-operators namespace
	oc get csv -n openshift-operators -o name | grep strimzi-cluster-operator | xargs oc delete -n openshift-operators || true
	oc get csv -n openshift-operators -o name | grep opendatahub-operator | xargs oc delete -n openshift-operators || true
	oc delete subscription -n openshift-operators -l peak.test.subscription=opendatahub-operator
	oc get mutatingwebhookconfiguration -o name | grep katib | grep $(ODHPROJECT) | xargs oc delete || true
	oc get validatingwebhookconfiguration -o name | grep katib | grep $(ODHPROJECT) | xargs oc delete || true
