ENVIRONMENT ?= dev
VERSION 	?= latest

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io

build:
	docker build -t radix-acr-cleanup .

build-push:
	az acr login --name $(CONTAINER_REPO)
	docker build -t $(DOCKER_REGISTRY)/radix-acr-cleanup:$(BRANCH)-$(VERSION) .
	docker push $(DOCKER_REGISTRY)/radix-acr-cleanup:$(BRANCH)-$(VERSION)

deploy-via-helm:
	make build-push

	helm upgrade --install radix-acr-cleanup \
	    ./charts/radix-acr-cleanup/ \
		--namespace radix-acr-cleanup