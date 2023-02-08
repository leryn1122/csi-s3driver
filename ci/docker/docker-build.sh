#!/usr/bin/env bash

set -eux

export VERSION=0.1.0
export DEBIAN_VERSION=11.6-slim
export GO_VERSION=1.19
export MIRRORS_SOURCE="mirrors.tuna.tsinghua.edu.cn"
export GOPROXY="https://goproxy.cn,direct"

docker build \
  --tag harbor.leryn.top/infra/csi-s3driver:${VERSION} \
  --build-arg DEBIAN_VERSION=${DEBIAN_VERSION} \
  --build-arg GO_VERSION=${GO_VERSION} \
  --build-arg MIRRORS_SOURCE=${MIRRORS_SOURCE} \
  --build-arg GOPROXY=${GOPROXY} \
  -f ci/docker/Dockerfile \
  .