SHELL ?= /bin/bash

ifneq ($(SKIP_DOCKER),true)
	DOCKER_CMD := docker run \
		-it \
		--rm \
		-e SKIP_DOCKER=true \
		-v gomodcache:/go/pkg/mod \
		-v $(dir $(realpath $(firstword $(MAKEFILE_LIST)))):/workspaces/bookkeeper \
		-w /workspaces/bookkeeper \
		ghcr.io/akuityio/k8sta-tools:v0.3.0
endif

################################################################################
# Tests                                                                        #
################################################################################

.PHONY: lint
lint:
	$(DOCKER_CMD) golangci-lint run --config golangci.yaml

.PHONY: test-unit
test-unit:
	$(DOCKER_CMD) go test \
		-v \
		-timeout=120s \
		-race \
		-coverprofile=coverage.txt \
		-covermode=atomic \
		./...

################################################################################
# Hack: Targets to help you hack                                               #
################################################################################

.PHONY: hack-build-image
hack-build-image:
	docker build . -t bookkeeper:dev
