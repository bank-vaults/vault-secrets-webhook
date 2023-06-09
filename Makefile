# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

export PATH := $(abspath bin/):${PATH}

# Dependency versions
GOLANGCI_VERSION = 1.53.1
LICENSEI_VERSION = 0.8.0
KIND_VERSION = 0.18.0
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
	helm upgrade --install vault-secrets-webhook charts/vault-secrets-webhook --namespace vault-infra --set replicaCount=0 --set podsFailurePolicy=Fail --set secretsFailurePolicy=Fail --set configMapMutation=true --set configMapFailurePolicy=Fail
	kurun port-forward localhost:8443 --namespace vault-infra --servicename vault-secrets-webhook --tlssecret vault-secrets-webhook-webhook-tls

.PHONY: clean
clean: ## Clean operator resources from a Kubernetes cluster
	kubectl delete -f deploy/crd.yaml
	kubectl delete -f deploy/rbac.yaml

.PHONY: artifacts
artifacts: container-image
artifacts: ## Build artifacts

.PHONY: container-image
container-image: ## Build container image
	docker build .

.PHONY: check
check: test lint ## Run checks (tests and linters)

.PHONY: test
test: ## Run tests
	go test -race -v ./...

.PHONY: test-acceptance
test-acceptance: ## Run acceptance tests
	go test -race -v -timeout 900s -tags kubeall ./test

.PHONY: lint
lint: ## Run linter
	golangci-lint run ${LINT_ARGS}

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
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'
