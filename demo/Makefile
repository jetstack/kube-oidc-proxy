CLOUD := google

# from https://suva.sh/posts/well-documented-makefiles/
.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: terraform_apply
terraform_apply: ## Applies terraform infrastructure
	cd infrastructure/$(CLOUD) && terraform init && terraform apply && terraform output config > ../../manifests/config.json

.PHONY: terraform_destroy
terraform_destroy: ## Destroy terraform infrastructure
	cd infrastructure/$(CLOUD) && terraform init && terraform destroy

manifests/config.json:
	$(MAKE) terraform_apply

.PHONY: manifests_apply
manifests_apply: manifests/config.json ## Use kubecfg to apply manifests into cluster
	kubecfg show manifests/main.jsonnet | kubectl apply -f -

.PHONY: manifests_validate
manifests_validate: manifests/config.json ## Use kubecfg to validate manifests
	kubecfg validate manifests/main.jsonnet

.PHONY: manifests_destroy
manifests_destroy: manifests/config.json ## Use kubecfg to delete manifests
	kubecfg show manifests/main.jsonnet | kubectl delete -f - --ignore-not-found
