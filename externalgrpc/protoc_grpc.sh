#/bin/bash
CURDIR=$(dirname $0)
PB_RELEASE="21.12"
PB_REL="https://github.com/protocolbuffers/protobuf/releases"

pushd $CURDIR

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1

export GOPATH="/tmp/protoc-${RANDOM}"
export GO111MODULE=on
export PATH=$GOPATH/bin:$PATH

mkdir -p $GOPATH

pushd $GOPATH

if [ "$(uname)" = "Darwin" ]; then
    curl -sLO ${PB_REL}/download/v${PB_RELEASE}/protoc-${PB_RELEASE}-osx-universal_binary.zip
    unzip protoc-${PB_RELEASE}-osx-universal_binary.zip
else
    curl -sLO ${PB_REL}/download/v${PB_RELEASE}/protoc-${PB_RELEASE}-osx-linux-x86_64.zip
    unzip protoc-${PB_RELEASE}-osx-linux-x86_64.zip
fi

mkdir -p src/k8s.io

pushd src/k8s.io

git clone https://github.com/kubernetes/autoscaler.git

pushd autoscaler/cluster-autoscaler
go mod tidy
go mod vendor
popd

popd
popd

#sed -i'' 's/option go_package = "cluster-autoscaler\/cloudprovider\/externalgrpc\/protos"/option go_package = "externalgrpc"/' \
#    ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos/externalgrpc.proto

#sed -e 's/package clusterautoscaler\.cloudprovider\.v1\.externalgrpc/package externalgrpc/' \
#    -e '/option go_package = "cluster-autoscaler\/cloudprovider\/externalgrpc\/protos"*/d' \
#    ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos/externalgrpc.proto > ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler/externalgrpc.proto

#sed -i'' -e 's/package clusterautoscaler\.cloudprovider\.v1\.externalgrpc/package externalgrpc/' \
#    ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos/externalgrpc.proto

$GOPATH/bin/protoc \
  -I ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler \
  -I ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler/vendor \
  --go_out=. \
  --go-grpc_out=. \
  ${GOPATH}/src/k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos/externalgrpc.proto

rm -rf *.go
mv cluster-autoscaler/cloudprovider/externalgrpc/protos/*.go .
rm -rf cluster-autoscaler

for FILE in *.go
do
    sed -i'' -e 's/package protos/package externalgrpc/' $FILE
done

popd

sudo rm -rf $GOPATH
