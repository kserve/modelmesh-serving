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

pre-commit run --all-files
RETURN_CODE=$?

function echoError() {
  LIGHT_YELLOW='\033[1;33m'
  NC='\033[0m' # No Color

  if [ "${CI}" != "true" ]; then
    echo -e "${LIGHT_YELLOW}${1}${NC}"
  else
    echo -e "[ERROR] ${1}"
  fi
}

if [ $RETURN_CODE -eq 127 ]; then
    echoError 'This failed because `pre-commit` is not installed.'
    echoError 'Did you mean to run `make run fmt` instead?'
    echoError ''
    echoError 'To run this outside of docker, see our CONTRIBUTING.md guide for'
    echoError 'how to set up your dev environment. This will automatically format'
    echoError 'your code when you make a new commit.'
elif [ "$RETURN_CODE" -ne 0 ]; then
    if [ "${CI}" != "true" ]; then
      echoError 'Pre-commit linter failed, but it may have automatically formatted your files.'
      echoError 'Check your changed files and/or manually fix the errors above then stage and commit.'
    else
      echoError "This test failed because your code isn't formatted and linted correctly."
      echoError 'To format and check the linter locally, run `make fmt`. It will appear to'
      echoError 'fail, but may automatically format some files. Stage the changed files and'
      echoError 'manually correct any other issues before committing and building again.'
    fi
fi

exit $RETURN_CODE
