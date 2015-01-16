#!/bin/bash

set -e

source $(cd $(dirname $BASH_SOURCE); pwd -P)/config.sh

MOUNT_DIR=$(cd $(dirname $0)/..; pwd -P)
id=$(echo $(date) $$| shasum | awk '{print $1}')
docker run --rm \
    --name sharaq-build-$id \
    -v $MOUNT_DIR:$WORKDIR \
    -e RESULTSDIR=$RESULTSDIR \
    sharaq-docker \
    ./build-on-docker.sh