.PHONY: all build buildbin clean run check cover lint docker help
BIN_DIR=bin
BASE_PATH=$(shell pwd)
all: check build
build:
	docker run --rm -v $(BASE_PATH):/go/src/github.com/basenana/nanafs \
	-v $(BASE_PATH)/bin:/bin/nanafs \
	-w /go/src/github.com/basenana/nanafs \
	golang:1.18 sh ./hack/multibuild.sh ./cmd /bin/nanafs
buildbin:
	CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -ldflags="-s -w" -o /usr/bin/nanafs ./cmd
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