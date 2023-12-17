#/bin/bash
CURDIR=$(dirname $0)
PB_RELEASE="25.1"
PB_REL="https://github.com/protocolbuffers/protobuf/releases"

export PROTOC_DIR="/tmp/protoc-${PB_RELEASE}"
export GOPATH=$PROTOC_DIR
export GO111MODULE=on
export PATH=$PROTOC_DIR/bin:$PATH

mkdir -p $PROTOC_DIR

pushd $PROTOC_DIR

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

if [ "$(uname)" = "Darwin" ]; then
    curl -sLO ${PB_REL}/download/v${PB_RELEASE}/protoc-${PB_RELEASE}-osx-universal_binary.zip
    unzip protoc-${PB_RELEASE}-osx-universal_binary.zip
else
    curl -sLO ${PB_REL}/download/v${PB_RELEASE}/protoc-${PB_RELEASE}-linux-x86_64.zip
    unzip protoc-${PB_RELEASE}-linux-x86_64.zip
fi

popd

$PROTOC_DIR/bin/protoc -I . -I vendor --proto_path=grpc/ --go_out=. --go-grpc_out=. grpc.proto

pushd $CURDIR
cp ./grpccloudprovider/*.go .
rm -rf ./grpccloudprovider
popd

sudo rm -rf $PROTOC_DIR
