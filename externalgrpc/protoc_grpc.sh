#/bin/bash
CURDIR=$(dirname $0)

pushd $CURDIR

rm -rf *.go

RELEASE=1.28.2
AUTOSCALER_PATH="/tmp/autoscaler-cluster-autoscaler-${RELEASE}/cluster-autoscaler"

rm -rf /tmp/autoscaler-cluster-autoscaler-${RELEASE}

pushd /tmp
    curl -Ls https://github.com/kubernetes/autoscaler/archive/refs/tags/cluster-autoscaler-${RELEASE}.tar.gz -O cluster-autoscaler-${RELEASE}.tar.gz
popd

tar zxvf /tmp/cluster-autoscaler-${RELEASE}.tar.gz autoscaler-cluster-autoscaler-${RELEASE}/cluster-autoscaler/cloudprovider/externalgrpc/protos

for FILE in autoscaler-cluster-autoscaler-${RELEASE}/cluster-autoscaler/cloudprovider/externalgrpc/protos/*.go
do
    cat ${FILE} | sed 's/package protos/package externalgrpc/' > $(basename $FILE)
done

rm -rf /tmp/cluster-autoscaler-${RELEASE}.tar.gz autoscaler-cluster-autoscaler-${RELEASE}

popd