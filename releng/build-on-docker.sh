#!/bin/bash

set -e

source $(cd $(dirname $BASH_SOURCE); pwd -P)/config.sh

pushd $WORKDIR

# for proj in github.com/goamz/goamz/aws github.com/goamz/goamz/s3 github.com/disintegration/imaging github.com/bradfitz/gomemcache/memcache github.com/lestrrat/go-server-starter github.com/lestrrat/go-apache-logformat github.com/lestrrat/go-file-rotatelogs; do
#    echo " + go get -u $proj"
#    go get -u $proj
#done

goxc \
    -tasks "xc archive" \
    -bc "linux windows darwin" \
    -wd $WORKDIR \
    -resources-include "README*,Changes" \
    -d $RESULTSDIR
popd
