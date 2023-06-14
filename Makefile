# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

export PATH := $(abspath bin/):${PATH}

CONTAINER_IMAGE_REF = ghcr.io/bank-vaults/vault-secrets-webhook:dev

# Dependency versions
GOLANGCI_VERSION = 1.53.1
LICENSEI_VERSION = 0.8.0
KIND_VERSION = 0.19.0
KURUN_VERSION = 0.7.0
HELM_DOCS_VERSION = 1.11.0

.PHONY: up
up: ## Start development environment
	kind create cluster
	docker compose up -d

.PHONY: stop
stop: ## Stop development environment
	# TODO: consider using k3d instead
	kind delete cluster
	docker compose stop

.PHONY: down
down: ## Destroy development environment
	kind delete cluster
	docker compose down -v

.PHONY: build
build: ## Build binary
	@mkdir -p build
	go build -race -o build/webhook .

.PHONY: run
run: ## Run the operator locally talking to a Kubernetes cluster
	KUBERNETES_NAMESPACE=vault-infra go run .

.PHONY: forward
forward: ## Install the webhook chart and kurun to port-forward the local webhook into Kubernetes
	kubectl create namespace vault-infra --dry-run -o yaml | kubectl apply -f -
	kubectl label namespaces vault-infra name=vault-infra --overwrite
	helm upgrade --install vault-secrets-webhook deploy/charts/vault-secrets-webhook --namespace vault-infra --set replicaCount=0 --set podsFailurePolicy=Fail --set secretsFailurePolicy=Fail --set configMapMutation=true --set configMapFailurePolicy=Fail
	kurun port-forward localhost:8443 --namespace vault-infra --servicename vault-secrets-webhook --tlssecret vault-secrets-webhook-webhook-tls

.PHONY: artifacts
artifacts: container-image helm-chart
artifacts: ## Build artifacts

.PHONY: container-image
container-image: ## Build container image
	docker build -t ${CONTAINER_IMAGE_REF} .

.PHONY: helm-chart
helm-chart: ## Build Helm chart
	@mkdir -p build
	helm package -d build/ deploy/charts/vault-secrets-webhook

.PHONY: check
check: test lint ## Run checks (tests and linters)

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
lint: lint-go lint-helm lint-yaml
lint: ## Run linter

.PHONY: lint-go
lint-go:
	golangci-lint run $(if ${CI},--out-format github-actions,)

.PHONY: lint-helm
lint-helm:
	helm lint deploy/charts/vault-secrets-webhook

.PHONY: lint-yaml
lint-yaml:
	yamllint $(if ${CI},-f github,) --no-warnings .

.PHONY: fmt
fmt: ## Format code
	golangci-lint run --fix

.PHONY: license-check
license-check: ## Run license check
	licensei check
	licensei header

.PHONY: generate
generate: generate-helm-docs
generate: ## Run generation jobs

.PHONY: generate-helm-docs
generate-helm-docs:
	helm-docs -s file -c charts/ -t README.md.gotmpl

deps: bin/golangci-lint bin/licensei bin/kind bin/kurun bin/helm-docs
deps: ## Install dependencies

bin/golangci-lint:
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -s -- v${GOLANGCI_VERSION}

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

bin/helm-docs:
	@mkdir -p bin
	curl -L https://github.com/norwoodj/helm-docs/releases/download/v${HELM_DOCS_VERSION}/helm-docs_${HELM_DOCS_VERSION}_$(shell uname)_x86_64.tar.gz | tar -zOxf - helm-docs > ./bin/helm-docs
	@chmod +x bin/helm-docs

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
