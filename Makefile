ENVIRONMENT ?= dev
VERSION 	?= latest

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD)
TAG := $(subst /,_,$(BRANCH))-$(HASH)

DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io
CONTAINER_REPO ?= radix$(ENVIRONMENT)

DOCKER_BUILDX_BUILD_BASE_CMD := docker buildx build -t $(DOCKER_REGISTRY)/radix-acr-cleanup:$(TAG) -t $(DOCKER_REGISTRY)/radix-acr-cleanup:$(subst /,_,$(BRANCH))-$(VERSION) --platform linux/arm64,linux/amd64 -f Dockerfile

.PHONY: build
build:
	${DOCKER_BUILDX_BUILD_BASE_CMD} .

.PHONY: build-push
build-push:
	az acr login --name $(CONTAINER_REPO)
	${DOCKER_BUILDX_BUILD_BASE_CMD} --push .

.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: lint
lint: bootstrap
	golangci-lint run

HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)

bootstrap:
ifndef HAS_GOLANGCI_LINT
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.59.1
endif
