#!/bin/bash

# Remove the x if you need no print out of each command
set -xe

# Environment variables needed by this script:
# - REGION: cloud region (us-south as default)
# - ORG:    target organization (dev-advo as default)
# - SPACE:  target space (dev as default)

REGION=${REGION:-"us-south"}
ORG=${ORG:-"dev-advo"}
SPACE=${SPACE:-"dev"}
RESOURCE_GROUP=${RESOURCE_GROUP:-"default"}
GIT_COMMIT_SHORT=$(git log -n1 --format=format:"%h")

# Git repo cloned at $WORKING_DIR, copy into $ARCHIVE_DIR and
# could be used by next stage
echo "Checking archive dir presence"
if [[ -z "$ARCHIVE_DIR" || "$ARCHIVE_DIR" == "." ]]; then
  echo -e "Build archive directory contains entire working directory."
else
  echo -e "Copying working dir into build archive directory: ${ARCHIVE_DIR} "
  mkdir -p "$ARCHIVE_DIR"
  find . -mindepth 1 -maxdepth 1 -not -path "./${ARCHIVE_DIR}" -exec cp -R '{}' "${ARCHIVE_DIR}/" ';'
fi

# Record git info
{
  echo "GIT_URL=${GIT_URL}"
  echo "GIT_BRANCH=${GIT_BRANCH}"
  echo "GIT_COMMIT=${GIT_COMMIT}"
  echo "GIT_COMMIT_SHORT=${GIT_COMMIT_SHORT}"
  echo "BUILD_NUMBER=${BUILD_NUMBER}"
  echo "REGION=${REGION}"
  echo "ORG=${ORG}"
  echo "SPACE=${SPACE}"
  echo "RESOURCE_GROUP=${RESOURCE_GROUP}"
} >> "${ARCHIVE_DIR}/build.properties"
grep -v -i password "${ARCHIVE_DIR}/build.properties"