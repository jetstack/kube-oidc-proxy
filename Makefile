# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR   ?= $(CURDIR)/bin
HACK_DIR ?= hack
PATH     := $(BINDIR):$(PATH)

export GO111MODULE=on

help:  ## display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: help build docker_build test depend verify all clean generate

UNAME_S := $(shell uname -s)
GOLANGCILINT_VERSION := 1.21.0
ifeq ($(UNAME_S),Linux)
	SHASUM := sha256sum -c
	KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.15.0/bin/linux/amd64/kubectl
	KUBECTL_HASH := ecec7fe4ffa03018ff00f14e228442af5c2284e57771e4916b977c20ba4e5b39
	GOLANGCILINT_URL := https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCILINT_VERSION)/golangci-lint-$(GOLANGCILINT_VERSION)-linux-amd64.tar.gz
	GOLANGCILINT_HASH := 2c861f8dc56b560474aa27cab0c075991628cc01af3451e27ac82f5d10d5106b
endif
ifeq ($(UNAME_S),Darwin)
	SHASUM := shasum -a 256 -c
	KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.15.0/bin/darwin/amd64/kubectl
	KUBECTL_HASH := 63f1ace419edffa1f5ebb64a6c63597afd48f8d94a61d4fb44e820139adbbe54
	GOLANGCILINT_URL := https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCILINT_VERSION)/golangci-lint-$(GOLANGCILINT_VERSION)-darwin-amd64.tar.gz
	GOLANGCILINT_HASH := 2b2713ec5007e67883aa501eebb81f22abfab0cf0909134ba90f60a066db3760
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

.PHONY: $(BINDIR)/golangci-lint
$(BINDIR)/golangci-lint: $(BINDIR)/golangci-lint-$(GOLANGCILINT_VERSION)
	@ln -fs golangci-lint-$(GOLANGCILINT_VERSION) $(BINDIR)/golangci-lint

$(BINDIR)/golangci-lint-$(GOLANGCILINT_VERSION):
	mkdir -p $(BINDIR) $(BINDIR)/.golangci-lint
	curl --fail -sL -o $(BINDIR)/.golangci-lint.tar.gz $(GOLANGCILINT_URL)
	echo "$(GOLANGCILINT_HASH)  $(BINDIR)/.golangci-lint.tar.gz" | $(SHASUM)
	tar xvf $(BINDIR)/.golangci-lint.tar.gz -C $(BINDIR)/.golangci-lint
	mv $(BINDIR)/.golangci-lint/*/golangci-lint $(BINDIR)/golangci-lint-$(GOLANGCILINT_VERSION)
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
	go vet ./cmd

go_lint: $(BINDIR)/golangci-lint ## lint golang code for problems
	$(BINDIR)/golangci-lint run

clean: ## clean up created files
	rm -rf \
		$(BINDIR) \
		pkg/mocks/authenticator.go \
		test/e2e/framework/issuer/bin

verify: depend verify_boilerplate go_fmt go_vet go_lint ## verify code and mod

generate: depend ## generates mocks and assets files
	go generate $$(go list ./pkg/... ./cmd/...)

test: generate verify ## run all go tests
	go test $$(go list ./pkg/... ./cmd/... | grep -v pkg/e2e)

e2e: ## run end to end tests
	KUBE_OIDC_PROXY_ROOT_PATH="$$(pwd)" go test -timeout 30m -v ./test/e2e/suite/.

#e2e-1.15: build ## run end to end tests for kubernetes version 1.15
#	KUBE_OIDC_PROXY_NODE_IMAGE=1.15.0 go test ./pkg/e2e/. -v --count=1
#
#e2e-1.14: build ## run end to end tests for kubernetes version 1.14
#	KUBE_OIDC_PROXY_NODE_IMAGE=1.14.3 go test ./pkg/e2e/. -v --count=1
#
#e2e-1.13: build ## run end to end tests for kubernetes version 1.13
#	KUBE_OIDC_PROXY_NODE_IMAGE=1.13.7 go test ./pkg/e2e/. -v --count=1
#
#e2e-1.12: build ## run end to end tests for kubernetes version 1.12
#	KUBE_OIDC_PROXY_NODE_IMAGE=1.12.8 go test ./pkg/e2e/. -v --count=1
#
#e2e-1.11: build ## run end to end tests for kubernetes version 1.11
#	KUBE_OIDC_PROXY_NODE_IMAGE=1.11.10 go test ./pkg/e2e/. -v --count=1

build: generate ## build kube-oidc-proxy
	CGO_ENABLED=0 go build -ldflags '-w $(shell hack/version-ldflags.sh)' -o ./bin/kube-oidc-proxy ./cmd/.

docker_build: generate test build ## build docker image
	docker build -t kube-oidc-proxy .

all: test build ## runs tests, build

image: all docker_build ## runs tests, build and docker build

dev_cluster_create: ## create dev cluster for development testing
	KUBE_OIDC_PROXY_ROOT_PATH="$$(pwd)" go run -v ./test/environment/dev create

dev_cluster_destroy: ## destroy dev cluster
	KUBE_OIDC_PROXY_ROOT_PATH="$$(pwd)" go run -v ./test/environment/dev destroy
