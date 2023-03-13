# Project
SHELL := /usr/bin/env bash -o pipefail
NAME := s3driver
VERSION := 0.1.0
BUILD_DATE := $(shell date +%Y%m%d)
GIT_VERSION := $(shell git describe --long --all)
SHA := $(shell git rev-parse --short=8 HEAD)

# Toolchain
GO := GO111MODULE=on GOPROXY="https://goproxy.cn,direct" go
GO_VERSION := $(shell $(GO) version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')

# Main
BINARY := s3driver
MAIN := ./cmd/csi-s3driver/main.go

# Docker
DOCKER := docker
DOCKER_CONTEXT := .
DOCKERFILE := ci/docker/Dockerfile
REGISTRY := harbor.leryn.top/infra
IMAGE_NAME := csi-s3driver
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(VERSION)
TEST_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME)-test:$(VERSION)

##@ General

.PHONY: help
help: ## Print help info.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Developement

.PHONY: install
install: ## Install dependencies.
	$(GO) get -d -v ./...

.PHONY: check
check: ## Check
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format against code.
	$(GO) fmt ./...

.PHONY: clean
clean: ## Clean target artifact.
	$(GO) clean -r -x

.PHONY: unittest
unittest: ## Run all unit tests.
	$(GO) test ./...

.PHONY: test
test: ## Run all integrate tests.
	./test/docker-build.sh
	$(DOCKER) run --rm \
      --device /dev/fuse \
      --privileged \
      --volume=$(PWD):/opt \
      $(TEST_IMAGE_NAME)

##@ Build

.PHONY: build
build: ## Build target artifact.
	@echo -e "\033[1;35mLead to the same behavior as \`make build\`\033[0m"
	./ci/docker/docker-build.sh

.PHONY: docker-build
docker-build: ## Build docker image.
	./ci/docker/docker-build.sh