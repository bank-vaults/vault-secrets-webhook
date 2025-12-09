# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

export PATH := $(abspath bin/):${PATH}

CONTAINER_IMAGE_REF = ghcr.io/bank-vaults/vault-secrets-webhook:dev

##@ General

# Targets commented with ## will be visible in "make help" info.
# Comments marked with ##@ will be used as categories for a group of targets.

.PHONY: help
default: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: up
up: ## Start development environment
	$(KIND_BIN) create cluster
	docker compose up -d

.PHONY: down
down: ## Destroy development environment
	$(KIND_BIN) delete cluster
	docker compose down -v

.PHONY: run
run: ## Run the operator locally talking to a Kubernetes cluster
	KUBERNETES_NAMESPACE=vault-infra go run .

.PHONY: forward
forward: ## Install the webhook chart and kurun to port-forward the local webhook into Kubernetes
	kubectl create namespace vault-infra --dry-run -o yaml | kubectl apply -f -
	kubectl label namespaces vault-infra name=vault-infra --overwrite
	$(HELM_BIN) upgrade --install vault-secrets-webhook deploy/charts/vault-secrets-webhook --namespace vault-infra --set replicaCount=0 --set podsFailurePolicy=Fail --set secretsFailurePolicy=Fail --set configMapMutation=true --set configMapFailurePolicy=Fail
	$(KURUN_BIN) port-forward localhost:8443 --namespace vault-infra --servicename vault-secrets-webhook --tlssecret vault-secrets-webhook-webhook-tls

##@ Build

.PHONY: build
build: ## Build binary
	@mkdir -p build
	go build -race -o build/webhook .

.PHONY: artifacts
artifacts: container-image helm-chart
artifacts: ## Build docker image and helm chart

.PHONY: container-image
container-image: ## Build container image
	docker build -t ${CONTAINER_IMAGE_REF} .

.PHONY: helm-chart
helm-chart: ## Build Helm chart
	@mkdir -p build
	$(HELM_BIN) package -d build/ deploy/charts/vault-secrets-webhook

##@ Checks

.PHONY: check
check: test lint ## Run lint checks and tests

.PHONY: test
test: ## Run tests
	go test -race -v ./...

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	go test -race -v -timeout 900s -tags e2e ./e2e/

.PHONY: test-e2e-local
test-e2e-local: container-image ## Run e2e tests locally
	LOAD_IMAGE=${CONTAINER_IMAGE_REF} WEBHOOK_VERSION=dev ${MAKE} test-e2e

.PHONY: lint
lint: lint-go lint-helm lint-docker lint-yaml
lint: ## Run linters

.PHONY: lint-go
lint-go:
	$(GOLANGCI_LINT_BIN) run

.PHONY: lint-helm
lint-helm:
	$(HELM_BIN) lint deploy/charts/vault-secrets-webhook

.PHONY: lint-docker
lint-docker:
	$(HADOLINT_BIN) Dockerfile

.PHONY: lint-yaml
lint-yaml:
	$(YAMLLINT_BIN) $(if ${CI},-f github,) --no-warnings .

.PHONY: fmt
fmt: ## Format code
	$(GOLANGCI_LINT_BIN) run --fix

.PHONY: license-cache
license-cache: ## Populate license cache
	$(LICENSEI_BIN) cache

.PHONY: license-check
license-check: ## Run license check
	$(LICENSEI_BIN) check
	$(LICENSEI_BIN) header

##@ Autogeneration

.PHONY: generate
generate: generate-helm-docs
generate: ## Run generation jobs

.PHONY: generate-helm-docs
generate-helm-docs:
	$(HELM_DOCS_BIN) -s file -c deploy/charts/ -t README.md.gotmpl

##@ Dependencies

deps: bin/golangci-lint bin/licensei bin/kind bin/kurun bin/helm bin/helm-docs
deps: ## Install dependencies

# Dependency versions
GOLANGCI_LINT_VERSION = 2.7.2
LICENSEI_VERSION = 0.9.0
KIND_VERSION = 0.30.0
KURUN_VERSION = 0.7.0
HELM_VERSION = 4.0.1
HELM_DOCS_VERSION = 1.14.2

# Dependency binaries
GOLANGCI_LINT_BIN := golangci-lint
LICENSEI_BIN := licensei
KIND_BIN := kind
KURUN_BIN := kurun
HELM_BIN := helm
HELM_DOCS_BIN := helm-docs

# TODO: add support for hadolint and yamllint dependencies
HADOLINT_BIN := hadolint
YAMLLINT_BIN := yamllint

# If we have "bin" dir, use those binaries instead
ifneq ($(wildcard ./bin/.),)
	GOLANGCI_LINT_BIN := bin/$(GOLANGCI_LINT_BIN)
	LICENSEI_BIN := bin/$(LICENSEI_BIN)
	KIND_BIN := bin/$(KIND_BIN)
	KURUN_BIN := bin/$(KURUN_BIN)
	HELM_BIN := bin/$(HELM_BIN)
	HELM_DOCS_BIN := bin/$(HELM_DOCS_BIN)
endif

bin/golangci-lint:
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -s -- v${GOLANGCI_LINT_VERSION}

bin/licensei:
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/goph/licensei/master/install.sh | bash -s -- v${LICENSEI_VERSION}

bin/kind:
	@mkdir -p bin
	curl -Lo bin/kind https://kind.sigs.k8s.io/dl/v${KIND_VERSION}/kind-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m | sed -e "s/aarch64/arm64/; s/x86_64/amd64/")
	@chmod +x bin/kind

bin/kurun:
	@mkdir -p bin
	curl -Lo bin/kurun https://github.com/banzaicloud/kurun/releases/download/${KURUN_VERSION}/kurun-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m | sed -e "s/aarch64/arm64/; s/x86_64/amd64/")
	@chmod +x bin/kurun

bin/helm:
	@mkdir -p bin
	curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | USE_SUDO=false HELM_INSTALL_DIR=bin DESIRED_VERSION=v$(HELM_VERSION) bash
	@chmod +x bin/helm

bin/helm-docs:
	@mkdir -p bin
	curl -L https://github.com/norwoodj/helm-docs/releases/download/v${HELM_DOCS_VERSION}/helm-docs_${HELM_DOCS_VERSION}_$(shell uname)_x86_64.tar.gz | tar -zOxf - helm-docs > ./bin/helm-docs
	@chmod +x bin/helm-docs
