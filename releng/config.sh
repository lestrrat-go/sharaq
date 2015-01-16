export WORKDIR=/work/src/github.com/Peatix/sharaq
export RESULTSDIR=/work/artifacts
export PROJECTDIR=$(cd $(dirname $BASH_SOURCE)/..; pwd -P)
if [ -z "$RELEASE_VERSION" ]; then
    export RELEASE_VERSION=$(grep version $PROJECTDIR/cmd/sharaq/sharaq.go | perl -ne 'print $1 if /version\s+=\s+"([^"]+)"/')
fi
