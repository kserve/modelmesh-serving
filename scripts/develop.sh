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
Run a dockerized development environment. If you specify a command to run inside the
environment, otherwise it will put you into an interactive shell.

usage: $0 [optional command]
  [-h | --help]    Display this help
EOF
)"
usage() {
  echo "$USAGE" >&2
  exit 1
}

REGISTRY="kserve"
PARAMS=""
CONTROLLER_IMG="modelmesh-controller"

while (("$#")); do
  arg="$1"
  case $arg in
  -h | --help)
    usage
    ;;
  -* | --*=) # unsupported flags
    echo "Error: Unsupported flag $1" >&2
    usage
    exit 1
    ;;
  *) # preserve positional arguments
    PARAMS="$PARAMS $1"
    shift
    ;;
  esac
done

eval set -- "$PARAMS"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "${DIR}/.."

# Make sure .bash_history exists and is a file
touch .bash_history

declare -a docker_run_args=(
  -v "${PWD}:/workspace"
  -v "${PWD}/.bash_history:/workspace/.bash_history"
  -v /var/run/docker.sock:/var/run/docker.sock
)

if [ "${CI}" != "true" ]; then
  docker_run_args+=(
    "-it"
  )
else
  docker_run_args+=(
    "-e CI=true"
  )
fi

# Run the develop container with local source mounted in
docker run --rm \
  "${docker_run_args[@]}" \
  --env NAMESPACE \
  "${REGISTRY}/${CONTROLLER_IMG}-develop:latest" "$@"
