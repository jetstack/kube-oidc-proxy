# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR   ?= $(CURDIR)/bin
HACK_DIR ?= hack
PATH     := $(BINDIR):$(PATH)

export GO111MODULE=on

help:  ## display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: help build docker_build test depend verify all clean generate

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	SHASUM := sha256sum -c
	KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.15.0/bin/linux/amd64/kubectl
	KUBECTL_HASH := ecec7fe4ffa03018ff00f14e228442af5c2284e57771e4916b977c20ba4e5b39
	GOLANGCILINT_URL := https://github.com/golangci/golangci-lint/releases/download/v1.18.0/golangci-lint-1.18.0-linux-amd64.tar.gz
	GOLANGCILINT_HASH := 0ef2c502035d5f12d6d3a30a7c4469cfcae4dd3828d15fbbfb799c8331cd51c4
endif
ifeq ($(UNAME_S),Darwin)
	SHASUM := shasum -a 256 -c
	KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.15.0/bin/darwin/amd64/kubectl
	KUBECTL_HASH := 63f1ace419edffa1f5ebb64a6c63597afd48f8d94a61d4fb44e820139adbbe54
	GOLANGCILINT_URL := https://github.com/golangci/golangci-lint/releases/download/v1.18.0/golangci-lint-1.18.0-darwin-amd64.tar.gz
	GOLANGCILINT_HASH := 37402c4606de3bc0116c9fc4d1ce18f2e90f55069ece1fa20f1b1622e0b54da3
endif

$(BINDIR)/mockgen:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/mockgen github.com/golang/mock/mockgen

$(BINDIR)/kubectl:
	mkdir -p $(BINDIR)
	curl --fail -sL -o $(BINDIR)/.kubectl $(KUBECTL_URL)
	echo "$(KUBECTL_HASH)  $(BINDIR)/.kubectl" | $(SHASUM)
	chmod +x $(BINDIR)/.kubectl
	mv $(BINDIR)/.kubectl $(BINDIR)/kubectl

$(BINDIR)/golangci-lint:
	mkdir -p $(BINDIR) $(BINDIR)/.golangci-lint
	curl --fail -sL -o $(BINDIR)/.golangci-lint.tar.gz $(GOLANGCILINT_URL)
	echo "$(GOLANGCILINT_HASH)  $(BINDIR)/.golangci-lint.tar.gz" | $(SHASUM)
	tar xvf $(BINDIR)/.golangci-lint.tar.gz -C $(BINDIR)/.golangci-lint
	mv $(BINDIR)/.golangci-lint/*/golangci-lint $(BINDIR)/golangci-lint
	rm -rf $(BINDIR)/.golangci-lint $(BINDIR)/.golangci-lint.tar.gz

depend: $(BINDIR)/mockgen $(BINDIR)/kubectl $(BINDIR)/golangci-lint

verify_boilerplate:
	$(HACK_DIR)/verify-boilerplate.sh

go_fmt:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

go_vet:
	go vet

go_lint: $(BINDIR)/golangci-lint ## lint golang code for problems
	$(BINDIR)/golangci-lint run

clean: ## clean up created files
	rm -rf \
		$(BINDIR) \
		kube-oidc-proxy \
		pkg/mocks/authenticator.go

verify: depend verify_boilerplate go_fmt go_vet go_lint ## verify code and mod

generate: depend ## generates mocks and assets files
	go generate $$(go list ./pkg/... ./cmd/...)

test: generate verify ## run all go tests
	go test $$(go list ./pkg/... ./cmd/... | grep -v pkg/e2e)

e2e: e2e-1.15 ## run end to end tests

e2e-1.15: build ## run end to end tests for kubernetes version 1.15
	KUBE_OIDC_PROXY_NODE_IMAGE=1.15.0 go test ./pkg/e2e/. -v --count=1

e2e-1.14: build ## run end to end tests for kubernetes version 1.14
	KUBE_OIDC_PROXY_NODE_IMAGE=1.14.3 go test ./pkg/e2e/. -v --count=1

e2e-1.13: build ## run end to end tests for kubernetes version 1.13
	KUBE_OIDC_PROXY_NODE_IMAGE=1.13.7 go test ./pkg/e2e/. -v --count=1

e2e-1.12: build ## run end to end tests for kubernetes version 1.12
	KUBE_OIDC_PROXY_NODE_IMAGE=1.12.8 go test ./pkg/e2e/. -v --count=1

e2e-1.11: build ## run end to end tests for kubernetes version 1.11
	KUBE_OIDC_PROXY_NODE_IMAGE=1.11.10 go test ./pkg/e2e/. -v --count=1

build: generate ## build kube-oidc-proxy
	CGO_ENABLED=0 go build -ldflags '-w $(shell hack/version-ldflags.sh)'

docker_build: generate test build ## build docker image
	docker build -t kube-oidc-proxy .

all: test build ## runs tests, build

image: all docker_build ## runs tests, build and docker build
