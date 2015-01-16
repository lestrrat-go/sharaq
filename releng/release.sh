#!/bin/bash

set -e

source $(cd $(dirname $BASH_SOURCE); pwd -P)/config.sh

if [ -z "$RELEASE_VERSION" ]; then
    echo "RELEASE_VERSION must be specified"
    exit 1
fi

if [ -z "$GITHUB_TOKEN_FILE" ]; then
    GITHUB_TOKEN_FILE=github_token
fi

if [ ! -e "$GITHUB_TOKEN_FILE" ]; then
    echo "GITHUB_TOKEN_FILE does not exist"
    exit 1
fi

docker run --rm \
    -v $PROJECTDIR:$WORKDIR\
    -e RELEASE_VERSION=$RELEASE_VERSION \
    -e GITHUB_USERNAME=Peatix \
    -e GITHUB_TOKEN=`cat $GITHUB_TOKEN_FILE` \
    sharaq-docker \
    /release-on-docker.sh