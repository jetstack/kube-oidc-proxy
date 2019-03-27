# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR   ?= $(CURDIR)/bin
HACK_DIR ?= hack
PATH     := $(BINDIR):$(PATH)

help:  ## display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: help build docker_build test depend verify all clean generate

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	SHASUM := sha256sum -c
	DEP_URL := https://github.com/golang/dep/releases/download/v0.5.1/dep-linux-amd64
	DEP_HASH := 7479cca72da0596bb3c23094d363ea32b7336daa5473fa785a2099be28ecd0e3
	KIND_URL := https://github.com/kubernetes-sigs/kind/releases/download/0.1.0/kind-linux-amd64
	KIND_HASH := 7566c0117d824731be5caee10fef0a88fb65e3508ee22a305dc17507ee87d874
	KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.13.3/bin/linux/amd64/kubectl
	KUBECTL_HASH := f3be209a48394e0e649b30ea376ce5093205fd6769c12e62c7ab39a0827c26fb
	GOLANGCILINT_URL := https://github.com/golangci/golangci-lint/releases/download/v1.15.0/golangci-lint-1.15.0-linux-amd64.tar.gz
	GOLANGCILINT_HASH := f37f4a15eb309578b0546703da5ea96bc5bd472f45f204338051aaca6fbbfc5b
endif
ifeq ($(UNAME_S),Darwin)
	SHASUM := shasum -a 256 -c
	DEP_URL := https://github.com/golang/dep/releases/download/v0.5.1/dep-darwin-amd64
	DEP_HASH := 7479cca72da0596bb3c23094d363ea32b7336daa5473fa785a2099be28ecd0e3
	KIND_URL := https://github.com/kubernetes-sigs/kind/releases/download/0.1.0/kind-darwin-amd64
	KIND_HASH := ce85d3ed3d03702af0e9c617098249aff2e0811e1202036b260b23df4551f3ad
	KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.13.3/bin/darwin/amd64/kubectl
	KUBECTL_HASH := 2ff06345a02636f1e6934f19dbc55452b587e06b2828c775dcdb29229c8da40f
	GOLANGCILINT_URL := https://github.com/golangci/golangci-lint/releases/download/v1.15.0/golangci-lint-1.15.0-darwin-amd64.tar.gz
	GOLANGCILINT_HASH := 083941efa692bfe3c29ba709964e9fe5896889316d51813e523157c96c3153e0
endif

$(BINDIR)/mockgen:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/mockgen ./vendor/github.com/golang/mock/mockgen

$(BINDIR)/dep:
	mkdir -p $(BINDIR)
	curl -sL -o $@ $(DEP_URL)
	echo "$(DEP_HASH)  $@" | $(SHASUM)
	chmod +x $@

$(BINDIR)/kind:
	mkdir -p $(BINDIR)
	curl -sL -o $@ $(KIND_URL)
	echo "$(KIND_HASH)  $@" | $(SHASUM)
	chmod +x $@

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

depend: $(BINDIR)/mockgen $(BINDIR)/dep $(BINDIR)/kind $(BINDIR)/kubectl $(BINDIR)/golangci-lint

verify_boilerplate:
	$(HACK_DIR)/verify-boilerplate.sh

verify_vendor: $(BINDIR)/dep
	$(BINDIR)/dep ensure -no-vendor -dry-run -v

go_fmt:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

go_vet:
	go vet $$(go list ./pkg/... ./cmd/...)

go_lint: $(BINDIR)/golangci-lint ## lint golang code for problems
	$(BINDIR)/golangci-lint run

clean: ## clean up created files
	rm -rf \
		$(BINDIR) \
		kube-oidc-proxy \
		pkg/mocks/authenticator.go

verify: verify_boilerplate verify_vendor go_fmt go_vet go_lint ## verify code and vendor

generate: depend ## generates mocks and assets files
	go generate $$(go list ./pkg/... ./cmd/...)

test: generate verify ## run all go tests
	go test $$(go list ./pkg/... ./cmd/... | grep -v cmd/e2e)

e2e: e2e-1.13 ## run end to end tests

e2e-1.13: build ## run end to end tests for kubernetes version 1.13
	KUBE_OIDC_PROXY_NODE_IMAGE=v1.13.3 go test ./cmd/e2e/. -v

e2e-1.12: build ## run end to end tests for kubernetes version 1.12
	KUBE_OIDC_PROXY_NODE_IMAGE=v1.12.5 go test ./cmd/e2e/. -v

e2e-1.11: build ## run end to end tests for kubernetes version 1.11
	KUBE_OIDC_PROXY_NODE_IMAGE=v1.11.3 go test ./cmd/e2e/. -v

build: generate ## build kube-oidc-proxy
	CGO_ENABLED=0 go build -ldflags '-w $(shell hack/version-ldflags.sh)'

docker_build: generate test build ## build docker image
	docker build -t kube-oidc-proxy .

all: test build ## runs tests, build

image: all docker_build ## runs tests, build and docker build
