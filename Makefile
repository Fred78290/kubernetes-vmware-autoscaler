all: build
VERSION_MAJOR ?= 0
VERSION_MINOR ?= 1
VERSION_BUILD ?= 1
VERSION ?= v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_BUILD)
DEB_VERSION ?= $(VERSION_MAJOR).$(VERSION_MINOR)-$(VERSION_BUILD)
TAG?=v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_BUILD)
FLAGS=
ENVVAR=
GOOS?=linux
GOARCH?=amd64
REGISTRY?=fred78290
BASEIMAGE?=k8s.gcr.io/debian-base-amd64:0.4.0
BUILD_DATE?=`date +%Y-%m-%dT%H:%M:%SZ`
VERSION_LDFLAGS=-X main.phVersion=$(VERSION)

ifdef BUILD_TAGS
  TAGS_FLAG=--tags ${BUILD_TAGS}
  PROVIDER=-${BUILD_TAGS}
  FOR_PROVIDER=" for ${BUILD_TAGS}"
else
  TAGS_FLAG=
  PROVIDER=
  FOR_PROVIDER=
endif

deps:
	govendor fetch +missing +external
#	wget "https://raw.githubusercontent.com/Fred78290/autoscaler/master/cluster-autoscaler/cloudprovider/grpc/grpc.proto" -O grpc/grpc.proto
#	protoc -I . -I vendor grpc/grpc.proto --go_out=plugins=grpc:.

build:
	$(ENVVAR) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-X main.phVersion=$(VERSION) -X main.phBuildDate=$(BUILD_DATE)" -a -o out/vsphere-autoscaler-$(GOOS)-$(GOARCH) ${TAGS_FLAG}

make-image:
	docker build --pull --build-arg BASEIMAGE=${BASEIMAGE} \
	    -t ${REGISTRY}/vsphere-autoscaler${PROVIDER}:${TAG} .

build-binary: clean deps
	make -e GOOS=linux -e GOARCH=amd64 build
	make -e GOOS=darwin -e GOARCH=amd64 build

test-unit: clean deps
	./scripts/run-tests.sh'

dev-release: build-binary execute-release
	@echo "Release ${TAG}${FOR_PROVIDER} completed"

clean:
#	sudo rm -rf out

format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -s -d {} + | tee /dev/stderr)" || \
    test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -s -w {} + | tee /dev/stderr)"

docker-builder:
	docker build -t kubernetes-vmware-autoscaler-builder ./builder

build-in-docker: docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/kubernetes-vmware-autoscaler/ kubernetes-vmware-autoscaler-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/kubernetes-vmware-autoscaler \
		&& BUILD_TAGS=${BUILD_TAGS} make -e BUILD_DATE=`date +%Y-%m-%dT%H:%M:%SZ` build-binary'

release: build-in-docker execute-release
	@echo "Full in-docker release ${TAG}${FOR_PROVIDER} completed"

container: clean build-in-docker make-image
	@echo "Created in-docker image ${TAG}${FOR_PROVIDER}"

test-in-docker: clean docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/kubernetes-vmware-autoscaler/ kubernetes-vmware-autoscaler-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/kubernetes-vmware-autoscaler && bash ./scripts/run-tests.sh'

.PHONY: all deps build test-unit clean format execute-release dev-release docker-builder build-in-docker release generate

