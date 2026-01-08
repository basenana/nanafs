.PHONY: all build buildbin clean run check cover lint docker help
BIN_DIR=bin
BASE_PATH=$(shell pwd)
$(shell git fetch --tags)
GIT_COMMIT := $(shell git rev-parse HEAD)
LATEST_TAG := $(shell git describe --tags --abbrev=0)
CONTAINER_TOOL ?= docker
IMG ?= registry.cn-hangzhou.aliyuncs.com/ihypo/nanafs:latest

all: check build
build:
	docker run --rm -v $(BASE_PATH):/go/src/github.com/basenana/nanafs \
	-e GIT_COMMIT=$(GIT_COMMIT) \
	-e GIT_TAG=$(LATEST_TAG) \
	-v $(BASE_PATH)/bin:/bin/nanafs \
	-w /go/src/github.com/basenana/nanafs \
	registry.cn-hangzhou.aliyuncs.com/hdls/golang:1.25 sh ./hack/multibuild.sh ./cmd /bin/nanafs
buildbin:
	CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build \
		-ldflags="-s -w -X github.com/basenana/nanafs/config.gitCommit=${GIT_COMMIT} -X github.com/basenana/nanafs/config.gitTag=${GIT_TAG}" \
		-o /usr/bin/nanafs ./cmd

PLATFORMS ?= linux/arm64,linux/amd64
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build --platform=linux/amd64 -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

clean:
	@go clean
test:
	@go test ./...
check:
	@go fmt ./...
	@go vet ./...
lint:
	golangci-lint run --enable-all
help:
	@echo "make build - build multi arch binary"
	@echo "make clean - clean workspace"
	@echo "make test  - run all testcase"
	@echo "make check - go format and vet"
	@echo "make lint  - golint"