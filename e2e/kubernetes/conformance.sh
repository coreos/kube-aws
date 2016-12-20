#!/bin/bash

set -eu

if [ ! -d /kube ]; then
  echo /kube does not exist. run this docker container like '`docker run -v path-to-kube-assets:/kube image-name`' 1>&2
  exit 1
fi

DIR=/kube-temp

mkdir ${DIR}

cp -R /kube/* ${DIR}/

export KUBECONFIG=${DIR}/kubeconfig

sed -i -e "s#credentials/#${DIR}/credentials/#g" ${KUBECONFIG}

set -vx

if [ "$FOCUS" != "" ]; then
  FOCUS=$(echo $FOCUS | sed -e 's/\[/\\[/' -e 's/\]/\\]/')
fi

FOCUS=${FOCUS:-\[Conformance\]}

go run hack/e2e.go -v --test -check_version_skew=false --test_args="--ginkgo.focus=$FOCUS"
