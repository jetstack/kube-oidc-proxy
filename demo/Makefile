# Copyright Jetstack Ltd. See LICENSE for details.
BINDIR ?= $(CURDIR)/bin
CLOUD  ?= google

KUBECONFIG := $(CURDIR)/.kubeconfig-$(CLOUD)

GOOGLE_PROJECT := $(shell gcloud config get-value core/project)
CA_CRT_FILE ?= /dev/null
CA_KEY_FILE ?= /dev/null

EXT_VARS := --tla-str cloud=$(CLOUD)

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	SHASUM := sha256sum -c
	KUBECFG_URL := https://github.com/ksonnet/kubecfg/releases/download/v0.14.0/kubecfg-linux-amd64
	KUBECFG_HASH := bb1455ec70f93d6f0fd344becec2f1617837a879e8363272d3216bf44c04cb2c
	JSONNET_URL := https://github.com/google/jsonnet/releases/download/v0.14.0/jsonnet-bin-v0.14.0-linux.tar.gz
	JSONNET_HASH := 30ba7b243ddb24231882ae3aa1930e76b6050bb76b5ccd0d97410b21fb784039
endif
ifeq ($(UNAME_S),Darwin)
	SHASUM := shasum -a 256 -c
	KUBECFG_URL := https://github.com/ksonnet/kubecfg/releases/download/v0.14.0/kubecfg-darwin-amd64
	KUBECFG_HASH := d448ae2888a28692a20d5ddb29ddae0574f74327611bd5b933092312f8c18fcf
	JSONNET_URL := https://github.com/google/jsonnet/releases/download/v0.14.0/jsonnet-bin-v0.14.0-osx.tar.gz
	JSONNET_HASH := e056c7305e8f86c0e2441085d5cdfd29b219ef09d8daae54e9f6d2647dd18d3e
endif

export KUBECONFIG

# from https://suva.sh/posts/well-documented-makefiles/
.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: terraform_apply
terraform_apply: ## Applies terraform infrastructure
	echo '' > infrastructure/$(CLOUD)/terraform.tfvars
	touch manifests/google-config.json && [ -s manifests/google-config.json ] || echo {} > manifests/google-config.json
	echo 'google_project = "$(GOOGLE_PROJECT)"' > infrastructure/$(CLOUD)/terraform.tfvars
	echo 'ca_crt_file = "$(CA_CRT_FILE)"' >> infrastructure/$(CLOUD)/terraform.tfvars
	echo 'ca_key_file = "$(CA_KEY_FILE)"' >> infrastructure/$(CLOUD)/terraform.tfvars
	echo 'cloud = "$(CLOUD)"' >> infrastructure/$(CLOUD)/terraform.tfvars
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
	$(BINDIR)/kubecfg $(EXT_VARS) show config.jsonnet | kubectl apply -f - --validate=false

.PHONY: manifests_validate
manifests_validate: depend manifests/$(CLOUD)-config.json ## Use kubecfg to validate manifests
	$(BINDIR)/kubecfg $(EXT_VARS) validate config.jsonnet

.PHONY: manifests_validate
jsonnet_fmt: depend ## validate formatting of jsonnet files
	$(BINDIR)/jsonnetfmt --test $(shell find manifests/components/. manifests/main.jsonnet -name "*.jsonnet")

.PHONY: manifests_destroy
manifests_destroy: depend manifests/$(CLOUD)-config.json ## Use kubecfg to delete manifests
	$(BINDIR)/kubecfg $(EXT_VARS) show config.jsonnet | kubectl delete -f - --ignore-not-found

.PHONY: manifests_backup_certificates
manifests_backup_certificates: ## backup tls secrets
	kubectl get -n auth -o yaml $(shell kubectl get secrets -n auth -oname) > .backup-certificates-$(CLOUD).yaml.tmp
	mv .backup-certificates-$(CLOUD).yaml.tmp .backup-certificates-$(CLOUD).yaml

.PHONY: manifests_restore_certificates
manifests_restore_certificates: ## restore tls secrets
	kubectl create ns auth --dry-run -o yaml | kubectl apply -f -
	kubectl apply -f .backup-certificates-$(CLOUD).yaml

.PHONY: verify
verify: jsonnet_fmt ## verify code and vendor

.PHONY: depend
depend: $(BINDIR)/kubecfg $(BINDIR)/jsonnet $(BINDIR)/jsonnetfmt

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
	cd $(BINDIR) && tar xvf jsonnet.tar.gz && rm jsonnet.tar.gz
