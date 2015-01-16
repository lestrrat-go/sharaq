#!/bin/bash

set -e

if [ -z "$GITHUB_TOKEN" ]; then
    echo "GITHUB_TOKEN environment variable must be set"
    exit 1
fi

if [ -z "$GITHUB_USERNAME" ]; then
    echo "GITHUB_USERNAME environment variable must be set"
    exit 1
fi

if [ -z "$RELEASE_VERSION" ]; then
    echo "RELEASE_VERSION environment variable must be set"
    exit 1
fi

source $(cd $(dirname $BASH_SOURCE); pwd -P)/config.sh

# Change directory to the project because that makes
# things much easier
pushd $WORKDIR

/build-on-docker.sh
ghr --debug -p 1 --replace -u "$GITHUB_USERNAME" $RELEASE_VERSION $RESULTSDIR/snapshot

popd
