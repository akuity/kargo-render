SHELL ?= /bin/bash

ifndef BOOKKEEPER_SERVER_PORT
	BOOKKEEPER_SERVER_PORT := 8080
endif

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

.PHONY: lint-chart
lint-chart:
	$(DOCKER_CMD) sh -c ' \
		cd charts/bookkeeper && \
		helm dep up && \
		helm lint . \
	'

################################################################################
# Build CLI                                                                    #
################################################################################

.PHONY: build-cli
build-cli:
	$(DOCKER_CMD) sh -c ' \
		VERSION=$(VERSION) \
		OSES="linux darwin windows" \
		ARCHS=amd64 \
		./scripts/build-cli.sh && \
		VERSION=$(VERSION) \
		OSES="linux darwin" \
		ARCHS=arm64 \
		./scripts/build-cli.sh \
	'

################################################################################
# Hack: Targets to help you hack                                               #
################################################################################

.PHONY: hack-build-image
hack-build-image:
	docker build . -t bookkeeper:dev

.PHONY: hack-build-cli
hack-build-cli:
	$(DOCKER_CMD) sh -c ' \
		OSES=$(shell go env GOOS) \
		ARCHS=$(shell go env GOARCH) \
		./scripts/build-cli.sh \
	'

.PHONY: hack-run-server
hack-run-server: hack-build-image
	docker run \
		-it \
		-e BOOKKEEPER_LOG_LEVEL=DEBUG \
		-p $(BOOKKEEPER_SERVER_PORT):8080 \
		bookkeeper:dev
