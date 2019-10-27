ENVIRONMENT ?= dev
VERSION 	?= latest

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD)
TAG := $(BRANCH)-$(HASH)

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io

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