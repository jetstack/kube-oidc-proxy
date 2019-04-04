# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR ?= $(CURDIR)/bin
CLOUD  := google

KUBECONFIG := $(CURDIR)/.kubeconfig-$(CLOUD)

ifeq ($(CLOUD),google)
GOOGLE_PROJECT := $(shell gcloud config get-value core/project)
endif

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	SHASUM := sha256sum -c
	KUBECFG_URL := https://github.com/ksonnet/kubecfg/releases/download/v0.9.1/kubecfg-linux-amd64
	KUBECFG_HASH := c972b52893794ccd49adfc9d7b69e2d388c0c91c4837e194de7cdf0d67b2baf8
endif
ifeq ($(UNAME_S),Darwin)
	SHASUM := shasum -a 256 -c
	KUBECFG_URL := https://github.com/ksonnet/kubecfg/releases/download/v0.9.1/kubecfg-darwin-amd64
	KUBECFG_HASH := 9b6772abc3e2da5add51aa57a2944f77f4c05be49df56087056837aa640fb9c9
endif

export KUBECONFIG

# from https://suva.sh/posts/well-documented-makefiles/
.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: terraform_apply
terraform_apply: ## Applies terraform infrastructure
	echo '' > infrastructure/$(CLOUD)/terraform.tfvars
ifeq ($(CLOUD),google)
	echo 'google_project = "$(GOOGLE_PROJECT)"' > infrastructure/$(CLOUD)/terraform.tfvars
endif
	cd infrastructure/$(CLOUD) && terraform init && terraform apply
	cd infrastructure/$(CLOUD) && terraform output config > ../../manifests/config.json
	$(shell cd infrastructure/$(CLOUD) && terraform output kubeconfig_command)

.PHONY: terraform_destroy
terraform_destroy: ## Destroy terraform infrastructure
	cd infrastructure/$(CLOUD) && terraform init && terraform destroy

manifests/config.json:
	$(MAKE) terraform_apply

.PHONY: manifests_apply
manifests_apply: depend manifests/config.json ## Use kubecfg to apply manifests into cluster
	# apply all CRDs
	$(BINDIR)/kubecfg show config.jsonnet --format json | sed 's#^---$$##' | jq 'select(.kind == "CustomResourceDefinition")' | kubectl apply -f -
	# apply everything
	$(BINDIR)/kubecfg show config.jsonnet | kubectl apply -f -

.PHONY: manifests_validate
manifests_validate: depend manifests/config.json ## Use kubecfg to validate manifests
	$(BINDIR)/kubecfg validate config.jsonnet

.PHONY: manifests_destroy
manifests_destroy: depend manifests/config.json ## Use kubecfg to delete manifests
	$(BINDIR)/kubecfg show config.jsonnet | kubectl delete -f - --ignore-not-found

.PHONY: verify
verify: manifests_validate

.PHONY: depend
depend: $(BINDIR)/kubecfg

$(BINDIR)/kubecfg:
	mkdir -p $(BINDIR)
	curl -sL -o $@ $(KUBECFG_URL)
	echo "$(KUBECFG_HASH)  $@" | $(SHASUM)
	chmod +x $@
