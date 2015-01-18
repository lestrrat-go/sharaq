#!/bin/bash

set -e

thisdir=$(cd $(dirname $BASH_SOURCE); pwd -P)
source $thisdir/config.sh

MOUNT_DIR=$(cd $(dirname $0)/..; pwd -P)
id=$(echo $(date) $$| shasum | awk '{print $1}')
docker run --rm \
    --name sharaq-build-$id \
    -v $MOUNT_DIR:$WORKDIR \
    -v $thisdir/artifacts:$RESULTSDIR \
    -e RESULTSDIR=$RESULTSDIR \
    sharaq-docker \
    ./build-on-docker.sh