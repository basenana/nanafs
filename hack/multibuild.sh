#!/bin/sh

#
# Copyright 2023 NanaFS Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

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