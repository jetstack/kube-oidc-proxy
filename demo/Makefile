# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR ?= $(CURDIR)/bin
CLOUD  ?= google

KUBECONFIG := $(CURDIR)/.kubeconfig-$(CLOUD)

GOOGLE_PROJECT := $(shell gcloud config get-value core/project)

EXT_VARS := --tla-str cloud=$(CLOUD)

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
JSONNET_URL := https://github.com/google/jsonnet/archive/v0.12.1.tar.gz
JSONNET_HASH := 257c6de988f746cc90486d9d0fbd49826832b7a2f0dbdb60a515cc8a2596c950

export KUBECONFIG

# from https://suva.sh/posts/well-documented-makefiles/
.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: terraform_apply
terraform_apply: ## Applies terraform infrastructure
	echo '' > infrastructure/$(CLOUD)/terraform.tfvars
	echo 'google_project = "$(GOOGLE_PROJECT)"' > infrastructure/$(CLOUD)/terraform.tfvars
	cd infrastructure/$(CLOUD) && terraform init && terraform apply
	cd infrastructure/$(CLOUD) && terraform output config > ../../manifests/$(CLOUD)-config.json
	$(shell cd infrastructure/$(CLOUD) && terraform output kubeconfig_command)

.PHONY: terraform_destroy
terraform_destroy: ## Destroy terraform infrastructure
	cd infrastructure/$(CLOUD) && terraform init && terraform destroy

manifests/$(CLOUD)-config.json:
	$(MAKE) terraform_apply

.PHONY: manifests_apply
manifests_apply: depend manifests/$(CLOUD)-config.json ## Use kubecfg to apply manifests into cluster
	# apply all CRDs
	$(BINDIR)/kubecfg $(EXT_VARS) show config.jsonnet --format json | sed 's#^---$$##' | jq 'select(.kind == "CustomResourceDefinition")' | kubectl apply -f -
	# apply everything
	$(BINDIR)/kubecfg $(EXT_VARS) show config.jsonnet | kubectl apply -f -

.PHONY: manifests_validate
manifests_validate: depend manifests/$(CLOUD)-config.json ## Use kubecfg to validate manifests
	$(BINDIR)/kubecfg $(EXT_VARS) validate config.jsonnet

.PHONY: manifests_validate
jsonnet_fmt: depend ## validate formatting of jsonnet files
	$(BINDIR)/jsonnet fmt --test $(shell find manifests/components/. manifests/main.jsonnet -name "*.jsonnet")

.PHONY: manifests_destroy
manifests_destroy: depend manifests/$(CLOUD)-config.json ## Use kubecfg to delete manifests
	$(BINDIR)/kubecfg $(EXT_VARS) show config.jsonnet | kubectl delete -f - --ignore-not-found

.PHONY: manifests_backup_certificates
manifests_backup_certificates:
	kubectl get secrets --all-namespaces -l certmanager.k8s.io/certificate-name -o yaml > .backup-certificates-$(CLOUD).yaml.tmp
	mv .backup-certificates-$(CLOUD).yaml.tmp .backup-certificates-$(CLOUD).yaml

.PHONY: manifests_restore_certificates
manifests_restore_certificates:
	kubectl create ns auth --dry-run -o yaml | kubectl apply -f -
	kubectl apply -f .backup-certificates-$(CLOUD).yaml

.PHONY: verify
verify: jsonnet_fmt ## verify code and vendor

.PHONY: depend
depend: $(BINDIR)/kubecfg $(BINDIR)/jsonnet

.PHONY: clean
clean: ## clean up created files
	rm -rf $(BINDIR)

$(BINDIR)/kubecfg:
	mkdir -p $(BINDIR)
	curl -sL -o $@ $(KUBECFG_URL)
	echo "$(KUBECFG_HASH)  $@" | $(SHASUM)
	chmod +x $@

$(BINDIR)/jsonnet:
	mkdir -p $(BINDIR)
	curl -sL -o $@.tar.gz $(JSONNET_URL)
	echo "$(JSONNET_HASH)  $@.tar.gz" | $(SHASUM)
	cd $(BINDIR) && tar xf jsonnet.tar.gz && rm jsonnet.tar.gz
	make -C $(BINDIR)/jsonnet-0.12.1
	mv $(BINDIR)/jsonnet-0.12.1/jsonnet $(BINDIR)/jsonnet
	rm -rf $(BINDIR)/jsonnet-0.12.1
