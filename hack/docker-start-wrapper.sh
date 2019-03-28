#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

export KUBE_ROOT=$(dirname "${BASH_SOURCE}")/..

# start docker
/etc/init.d/docker start

# wait for it to be started
n=0
until [ $n -ge 15 ]
do
    docker info > /dev/null && break
    n=$[$n+1]
    sleep 1
done

exec $@
