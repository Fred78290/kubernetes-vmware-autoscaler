#/bin/bash

PB_RELEASE="3.11.1"
PB_REL="https://github.com/protocolbuffers/protobuf/releases"

export PROTOC_DIR="/tmp/protoc-${PB_RELEASE}"
export GOPATH=$PROTOC_DIR
export GO111MODULE=on
export PATH=$PROTOC_DIR/bin:$PATH

mkdir -p $PROTOC_DIR

pushd $PROTOC_DIR

#go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
#go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1

go get -v google.golang.org/grpc@v1.29.1
#go get -v github.com/golang/protobuf@v1.3.2
#go get -v github.com/golang/protobuf/protoc-gen-go@v1.3.2

curl -LO ${PB_REL}/download/v${PB_RELEASE}/protoc-${PB_RELEASE}-linux-x86_64.zip
unzip protoc-${PB_RELEASE}-linux-x86_64.zip

popd

$PROTOC_DIR/bin/protoc -I . -I vendor grpc/grpc.proto --go_out=plugins=grpc:.

sudo rm -rf $PROTOC_DIR
