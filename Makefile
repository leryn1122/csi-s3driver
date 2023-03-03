# Project
SHELL := /usr/bin/env bash -o pipefail
NAME := s3driver
#VERSION = $(shell git branch | grep '*' | awk '{print $2}')
VERSION := 0.1.0

# Toolchain
GO := GO111MODULE=on go
GO_VERSION := $(shell $(GO) version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')

# Main
BINARY := s3driver
MAIN := ./cmd/s3driver/main.go

# Docker
DOCKER := docker
DOCKERFILE := ci/docker/Dockerfile
REGISTRY := harbor.leryn.top/infra
IMAGE_NAME := csi-s3driver
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(VERSION)
TEST_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):test-$(VERSION)

##@ General

.PHONY: help
help: ## Print help info.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Developement

.PHONY: install
install: ## Install dependencies.
	$(GO) get -d -v ./...

.PHONY: fmt
fmt: ## Format against code.
	$(GO) fmt ./...

.PHONY: clean
clean: ## Clean target artifact.
	$(GO) clean -r -x

.PHONY: test
test: ## Run test.
	$(DOCKER) build -t $(FULL_IMAGE_NAME) -f $(DOCKERFILE) .
	$(DOCKER) build -t $(TEST_IMAGE_NAME) -f $(DOCKERFILE).test .
	$(DOCKER) run --rm \
                  --device /dev/fuse \
                  --privileged \
                  --volume=$(PWD):/opt \
                  $(TEST_IMAGE_TAG)

##@ Build

.PHONY: check
check: ## Check
	CGO_ENABLED=0 GOOS=linux $(GO) build -a -ldflags '-extldflags "-static"' -o target/$(BINARY) $(MAIN)

.PHONY: build
build: ## Build target artifact.
	./ci/docker/docker-build.sh

.PHONY: docker-build
docker-build: ## Build docker image.
	./ci/docker/docker-build.sh