# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR   ?= $(CURDIR)/bin
HACK_DIR ?= hack
PATH     := $(BINDIR):$(PATH)

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: help build docker_build test depend verify all clean generate

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	SHASUM := sha256sum -c
	DEP_URL := https://github.com/golang/dep/releases/download/v0.5.1/dep-linux-amd64
	DEP_HASH := 7479cca72da0596bb3c23094d363ea32b7336daa5473fa785a2099be28ecd0e3
endif
ifeq ($(UNAME_S),Darwin)
	SHASUM := shasum -a 256 -c
	DEP_URL := https://github.com/golang/dep/releases/download/v0.5.1/dep-darwin-amd64
	DEP_HASH := 7479cca72da0596bb3c23094d363ea32b7336daa5473fa785a2099be28ecd0e3
endif

$(BINDIR)/mockgen:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/mockgen ./vendor/github.com/golang/mock/mockgen

$(BINDIR)/dep:
	mkdir -p $(BINDIR)
	curl -sL -o $@ $(DEP_URL)
	echo "$(DEP_HASH)  $@" | $(SHASUM)
	chmod +x $@

depend: $(BINDIR)/mockgen $(BINDIR)/dep

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
	go vet $$(go list ./pkg/... ./cmd/...| grep -v pkg/wing/client/clientset/internalversion/fake | grep -v pkg/wing/client/clientset/versioned/fake)

clean: ## clean up created files
	rm -rf \
		$(BINDIR) \
		kube-oidc-proxy \
		pkg/mocks/authenticator.go

verify: verify_boilerplate verify_vendor go_fmt go_vet ## verify code and vendor

generate: depend ## generates mocks and assets files
	go generate $$(go list ./pkg/... ./cmd/...)

test: generate verify ## run all go tests
	go test $$(go list ./pkg/... ./cmd/...)

build: generate ## build kube-oidc-proxy
	CGO_ENABLED=0 go build

docker_build: generate test build ## build docker image
	docker build -t kube-oidc-proxy .

all: test build docker_build # runs tests, build and docker build
