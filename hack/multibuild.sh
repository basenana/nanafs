#!/bin/sh

set -x
set -e

PKG=$1
BINDIR=$2

build() {
  echo "build OS=$1 Arch=$2"
  mkdir -p $BINDIR/$1/$2
  CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -ldflags="-s -w" -o $BINDIR/$1/$2/nanafs $PKG
}

main(){
  build "linux" "amd64"
  build "linux" "arm64"
  build "darwin" "amd64"
  build "darwin" "arm64"
}

main