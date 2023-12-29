ENVIRONMENT ?= dev
VERSION 	?= latest

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD)
TAG := $(BRANCH)-$(HASH)

DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io
CONTAINER_REPO ?= radix$(ENVIRONMENT)

build:
	docker build -t radix-acr-cleanup .

build-push:
	az acr login --name $(CONTAINER_REPO)
	docker build -t $(DOCKER_REGISTRY)/radix-acr-cleanup:$(TAG) -t $(DOCKER_REGISTRY)/radix-acr-cleanup:$(BRANCH)-$(VERSION) .
	docker push $(DOCKER_REGISTRY)/radix-acr-cleanup:$(BRANCH)-$(HASH)
	docker push $(DOCKER_REGISTRY)/radix-acr-cleanup:$(TAG)

deploy-via-helm:
	make build-push

	# Will need to be installed in default namespace
	# as it relies on radix-sp-acr-azure secret
	helm upgrade --install radix-acr-cleanup \
	    ./charts/radix-acr-cleanup/ \
		--set image.repository=$(DOCKER_REGISTRY)/radix-acr-cleanup \
		--set image.tag=$(TAG) \
		--set period=10s \
		--set metrics.enabled=true \
		--namespace default

test:
	go test -cover `go list ./...`

lint: bootstrap
	golangci-lint run --timeout=30m --max-same-issues=0



HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)

bootstrap:
ifndef HAS_GOLANGCI_LINT
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
endif
