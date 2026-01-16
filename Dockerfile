# Build the manager binary
FROM registry.cn-hangzhou.aliyuncs.com/hdls/golang:1.25 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
ADD . /workspace

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o nanafs cmd/main.go

FROM registry.cn-hangzhou.aliyuncs.com/hdls/busybox:1.37-musl
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /workspace/nanafs /usr/bin/
RUN mkdir -p /var/lib/nanafs
ENV TZ=Asia/Shanghai
