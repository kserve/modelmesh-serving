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

USAGE="$(
  cat <<EOF
Build the Dockerfile to a specific target

usage: $0 [flags]
  [-h | --help]       Display this help
  [-t | --target]     Specify a target to build, default: "runtime"
  [--tag]             Docker Image Tag to use
  [--devimage]        Full name of dev docker image
  [--engine]          Container Engine to be used, default: "docker"
EOF
)"

usage() {
  echo "$USAGE" >&2
  exit 1
}

DOCKER_TARGET="runtime"
DOCKER_TAG="$(git rev-parse --abbrev-ref HEAD)-$(date +"%Y%m%dT%H%M%S%Z")"
CONTROLLER_IMG="kserve/modelmesh-controller"
DEV_IMAGE="$(cat .develop_image_name)"
ENGINE="docker"

while (("$#")); do
  arg="$1"
  case $arg in
  -h | --help)
    usage
    ;;
  -t | --target)
    if [ -n "$2" ] && [ "${2:0:1}" != "-" ]; then
      DOCKER_TARGET=$2
      shift 2
    else
      echo "Error: Argument for $1 is missing" >&2
      usage
    fi
    ;;
  --tag)
    if [ -n "$2" ] && [ "${2:0:1}" != "-" ]; then
      DOCKER_TAG=$2
      shift 2
    else
      echo "Error: Argument for $1 is missing" >&2
      usage
    fi
    ;;
  --devimage)
    if [ -n "$2" ] && [ "${2:0:1}" != "-" ]; then
      DEV_IMAGE=$2
      shift 2
    else
      echo "Error: Argument for $1 is missing" >&2
      usage
    fi
    ;;
  --engine)
    if [ -n "$2" ] && [ "${2:0:1}" != "-" ]; then
      ENGINE=$2
      shift 2
    else
      echo "Error: Argument for $1 is missing" >&2
      usage
    fi
    ;;
  -* | --*=) # unsupported flags
    echo "Error: Unsupported flag $1" >&2
    usage
    ;;
  esac
done

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

cd "$DIR/.."

IMAGE_SUFFIX=""

if [ "${DOCKER_TARGET}" != "runtime" ]; then
  IMAGE_SUFFIX="-${DOCKER_TARGET}"
fi

declare -a docker_args=(
  --target "${DOCKER_TARGET}"
  -t "${CONTROLLER_IMG}${IMAGE_SUFFIX}:${DOCKER_TAG}"
  -t "${CONTROLLER_IMG}${IMAGE_SUFFIX}:latest"
  --build-arg "DEV_IMAGE=${DEV_IMAGE}"
)

if [[ $DOCKER_TARGET == 'runtime' ]]; then
  git_commit_sha=${GIT_COMMIT:-}
  if [[ -z $git_commit_sha ]]; then
    git_commit_sha="$(git rev-parse HEAD)"
  fi

  docker_args+=("--build-arg=COMMIT_SHA=${git_commit_sha}")
  docker_args+=("--build-arg=IMAGE_VERSION=${DOCKER_TAG}")
fi

$ENGINE build . \
  "${docker_args[@]}"
