#!/bin/bash

# Remove the x if you need no print out of each command
set -xe

# Environment variables needed by this script:
# - RUN_TASK:             execution task:
#                           - `build`: build the image
#                           - `build_push`: build and push the image
#                           - `cleanup`: cleanup the image in sandbox
#

# The following envs could be loaded from `build.properties` that
# `run-setup.sh` generates.
# - REGION:               cloud region (us-south as default)
# - ORG:                  target organization (dev-advo as default)
# - SPACE:                target space (dev as default)
# - GIT_BRANCH:           git branch
# - GIT_COMMIT:           git commit hash
# - GIT_COMMIT_SHORT:     git commit hash short

REGION=${REGION:-"us-south"}
ORG=${ORG:-"dev-advo"}
SPACE=${SPACE:-"dev"}
RUN_TASK=${RUN_TASK:-"build"}

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

######################################################################################
# Build image                                                                        #
######################################################################################
build_image() {
  echo "=======================Build modelmesh controller image======================="
  # Will build develop and then runtime images.

  echo "==============================Build dev image ================================"
  make build.develop
  docker images
  docker inspect "kserve/modelmesh-controller-develop:latest"
  echo "==========================Build runtime image ================================"
  make build
  docker images
  docker inspect "kserve/modelmesh-controller:latest"

  echo "==========================Push image to sandbox================================"
  echo "BUILD_NUMBER=${BUILD_NUMBER}"
  echo "ARCHIVE_DIR=${ARCHIVE_DIR}"
  echo "GIT_BRANCH=${GIT_BRANCH}"
  echo "GIT_COMMIT=${GIT_COMMIT}"
  echo "GIT_COMMIT_SHORT=${GIT_COMMIT_SHORT}"
  echo $PUBLISH_TAG
  set +x
  docker login -u "$DOCKERSANDBOX_USERNAME" -p "$DOCKERSANDBOX_TOKEN"
  set -x
  docker tag "kserve/modelmesh-controller:latest" "${DOCKERSANDBOX_NAMESPACE}/modelmesh-controller:${GIT_COMMIT_SHORT}"
  docker push "${DOCKERSANDBOX_NAMESPACE}/modelmesh-controller:${GIT_COMMIT_SHORT}"
}

######################################################################################
# Push image to Docker Hub                                                           #
######################################################################################
push_image() {
  echo "=======================Push image to Docker Hub==============================="
  if [[ "$PUBLISH_TAG" == "latest" ]]; then
    apt update
    apt install jq -y
    apt install curl -y
    export LAST_PUSHED=$(curl -X GET https://hub.docker.com/v2/repositories/${DOCKERHUB_NAMESPACE}/modelmesh-controller/tags/latest | jq -r '.tag_last_pushed')
    let DIFF=($(git log -1 --format=%ct)-$(date -d $LAST_PUSHED +%s))
    if [[ "$DIFF" -gt 0 ]]; then
      # Will proceed to push the latest image since it is not published yet
      echo "Will push the latest image since the latest commit is more recent";
    else
      # The latest commit should be already published since the last pushed time is more recent
      echo "Will not push the image since the last pushed time is more recent";
      return 0;
    fi
  fi

  # login dockerhub
  echo $DOCKERHUB_USERNAME
  echo $DOCKERHUB_NAMESPACE
  echo $PUBLISH_TAG
  set +x
  docker login -u "$DOCKERHUB_USERNAME" -p "$DOCKERHUB_TOKEN"
  set -x
  docker tag "kserve/modelmesh-controller:latest" "${DOCKERHUB_NAMESPACE}/modelmesh-controller:${PUBLISH_TAG}"
  docker push "${DOCKERHUB_NAMESPACE}/modelmesh-controller:${PUBLISH_TAG}"
}

######################################################################################
# Cleanup image                                                                      #
######################################################################################
cleanup_image() {
  echo "======================Cleanup modelmesh controller image======================"
  # Will delete the image tag from Docker Hub sandbox.
  echo "GIT_COMMIT_SHORT=${GIT_COMMIT_SHORT}"
  set +x
  HUB_TOKEN=$(curl -s -H "Content-Type: application/json" -X POST -d '{"username": "'${DOCKERSANDBOX_USERNAME}'", "password": "'${DOCKERSANDBOX_TOKEN}'"}' https://hub.docker.com/v2/users/login/ | jq -r .token)
  curl -H "Authorization: JWT ${HUB_TOKEN}" -X DELETE https://hub.docker.com/v2/repositories/${DOCKERSANDBOX_NAMESPACE}/modelmesh-controller/tags/${GIT_COMMIT_SHORT}/
}

case "$RUN_TASK" in
  "build")
    build_image
    ;;

  "build_push")
    build_image
    push_image
    ;;

  "cleanup")
    cleanup_image
    ;;

  *)
    echo "please specify RUN_TASK=build|build_push|cleanup"
    ;;
esac
