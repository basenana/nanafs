.PHONY: all build buildbin clean run check cover lint docker help
BIN_DIR=bin
BASE_PATH=$(shell pwd)
$(shell git fetch --tags)
GIT_COMMIT := $(shell git rev-parse HEAD)
LATEST_TAG := $(shell git describe --tags --abbrev=0)

all: check build
build:
	docker run --rm -v $(BASE_PATH):/go/src/github.com/basenana/nanafs \
	-e GIT_COMMIT=$(GIT_COMMIT) \
	-e GIT_TAG=$(LATEST_TAG) \
	-v $(BASE_PATH)/bin:/bin/nanafs \
	-w /go/src/github.com/basenana/nanafs \
	golang:1.20 sh ./hack/multibuild.sh ./cmd /bin/nanafs
buildbin:
	CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build \
		-ldflags="-s -w -X github.com/basenana/nanafs/config.gitCommit=${GIT_COMMIT} -X github.com/basenana/nanafs/config.gitTag=${GIT_TAG}" \
		-o /usr/bin/nanafs ./cmd
clean:
	@go clean
test:
	@go test ./...
check:
	@go fmt ./...
	@go vet ./...
lint:
	golangci-lint run --enable-all
fsapi:
	API_V1_DIR="cmd/apps/apis/fsapi/v1"; \
	protoc --go_out=. --go_opt=paths=source_relative \
		  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		  --go-grpc_opt=require_unimplemented_servers=false \
		  $${API_V1_DIR}/fsapi-v1.proto
help:
	@echo "make build - build multi arch binary"
	@echo "make clean - clean workspace"
	@echo "make test  - run all testcase"
	@echo "make check - go format and vet"
	@echo "make lint  - golint"