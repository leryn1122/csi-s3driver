# Project
SHELL := /bin/sh
NAME := s3driver
#VERSION = $(shell git branch | grep '*' | awk '{print $2}')
VERSION := 0.1.0

# Toolchain
GO := GO111MODULE=on go
GO_VERSION := $(shell $(GO) version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')

# Main
BINARY := s3driver
MAIN := ./cmd/s3driver/main.go

# Container
DOCKER := docker
DOCKERFILE := ci/docker/Dockerfile
REGISTRY := harbor.leryn.top/infra
IMAGE_NAME := csi-s3driver
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(VERSION)
TEST_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):test-$(VERSION)

.PHONY: build test-build test clean
test-build:
	CGO_ENABLED=0 GOOS=linux $(GO) build -a -ldflags '-extldflags "-static"' -o target/$(BINARY) $(MAIN)

build:
	./ci/docker/docker-build.sh

test:
	$(DOCKER) build -t $(FULL_IMAGE_NAME) -f $(DOCKERFILE) .
	$(DOCKER) build -t $(TEST_IMAGE_NAME) -f $(DOCKERFILE).test .
	$(DOCKER) run --rm \
                  --device /dev/fuse \
                  --privileged \
                  --volume=$(PWD):/opt \
                  $(TEST_IMAGE_TAG)

clean:
	$(GO) clean -r -x