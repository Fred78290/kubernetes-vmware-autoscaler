ALL_ARCH = amd64 arm64

.EXPORT_ALL_VARIABLES:

all: $(addprefix build-arch-,$(ALL_ARCH))

VERSION_MAJOR ?= 1
VERSION_MINOR ?= 21
VERSION_BUILD ?= 0
TAG?=v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_BUILD)
FLAGS=
ENVVAR=
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
REGISTRY?=fred78290
BUILD_DATE?=`date +%Y-%m-%dT%H:%M:%SZ`
VERSION_LDFLAGS=-X main.phVersion=$(TAG)

ifdef BUILD_TAGS
  TAGS_FLAG=--tags ${BUILD_TAGS}
  PROVIDER=-${BUILD_TAGS}
  FOR_PROVIDER=" for ${BUILD_TAGS}"
else
  TAGS_FLAG=
  PROVIDER=
  FOR_PROVIDER=
endif

IMAGE=$(REGISTRY)/vsphere-autoscaler$(PROVIDER)

deps:
	go mod vendor
#	wget "https://raw.githubusercontent.com/Fred78290/autoscaler/master/cluster-autoscaler/cloudprovider/grpc/grpc.proto" -O grpc/grpc.proto
#	protoc -I . -I vendor grpc/grpc.proto --go_out=plugins=grpc:.

build: build-arch-$(GOARCH)

build-arch-%: deps clean-arch-%
	$(ENVVAR) GOOS=$(GOOS) GOARCH=$* go build -ldflags="-X main.phVersion=$(TAG) -X main.phBuildDate=$(BUILD_DATE)" -a -o out/$(GOOS)/$*/vsphere-autoscaler ${TAGS_FLAG}

test-unit: clean build
	go get -u github.com/vmware/govmomi/vcsim
	bash ./scripts/run-tests.sh

dev-release: $(addprefix dev-release-arch-,$(ALL_ARCH))

dev-release-arch-%: build-arch-% make-image-arch-% push-image-arch-%
	@echo "Release ${TAG}${FOR_PROVIDER}-$* completed"

make-image: make-image-arch-$(GOARCH)

make-image-arch-%:
ifdef BASEIMAGE
	docker build --pull --build-arg BASEIMAGE=${BASEIMAGE} \
		-t ${IMAGE}-$*:${TAG} \
		-f Dockerfile.$* .
else
	docker build --pull \
		-t ${IMAGE}-$*:${TAG} \
		-f Dockerfile.$* .
endif
	@echo "Image ${TAG}${FOR_PROVIDER}-$* completed"

push-image: push-image-arch-$(GOARCH)

push-image-arch-%:
	./push_image.sh ${IMAGE}-$*:${TAG}

docker-push: docker-push-arch-$(GOARCH)

docker-push-arch-%:
	docker push ${IMAGE}-$*:${TAG}

push-manifest:
ifdef BASEIMAGE
	docker buildx build --pull --platform linux/amd64,linux/arm64 --push \
		--build-arg BASEIMAGE=${BASEIMAGE} \
		-t ${IMAGE}:${TAG} .
else
	docker buildx build --pull --platform linux/amd64,linux/arm64 --push \
		-t ${IMAGE}:${TAG} .
endif
	@echo "Image ${TAG}${FOR_PROVIDER}-$* completed"

container-push-manifest: container push-manifest

execute-release: $(addprefix make-image-arch-,$(ALL_ARCH)) $(addprefix push-image-arch-,$(ALL_ARCH))
	@echo "Release ${TAG}${FOR_PROVIDER} completed"

clean: clean-arch-$(GOARCH)

clean-arch-%:
	rm -f ./out/$(GOOS)/$*/vsphere-autoscaler$(PROVIDER)

format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -s -d {} + | tee /dev/stderr)" || \
    test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -s -w {} + | tee /dev/stderr)"

docker-builder:
	docker build -t kubernetes-vmware-autoscaler-builder ./builder

build-in-docker: build-in-docker-arch-$(GOARCH)

build-in-docker-arch-%: clean-arch-% docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/vsphere-autoscaler/ kubernetes-vmware-autoscaler-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/vsphere-autoscaler \
		&& BUILD_TAGS=${BUILD_TAGS} make -e REGISTRY=${REGISTRY} -e TAG=${TAG} -e BUILD_DATE=`date +%Y-%m-%dT%H:%M:%SZ` build-arch-$*'

release: $(addprefix build-in-docker-arch-,$(ALL_ARCH)) execute-release
	@echo "Full in-docker release ${TAG}${FOR_PROVIDER} completed"

container: $(addprefix container-arch-,$(ALL_ARCH))

container-arch-%: build-in-docker-arch-%
	@echo "Full in-docker image ${TAG}${FOR_PROVIDER}-$* completed"

test-in-docker: clean docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/kubernetes-vmware-autoscaler/ kubernetes-vmware-autoscaler-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/kubernetes-vmware-autoscaler && bash ./scripts/run-tests.sh'

.PHONY: all build test-unit clean format execute-release dev-release docker-builder build-in-docker release generate push-image push-manifest
