ARTIFACT_NAME = external-dns-huaweicloud-webhook

# logging
LOG_LEVEL = debug
LOG_ENVIRONMENT = production
LOG_FORMAT = auto

REGISTRY ?= localhost:5001
IMAGE_NAME ?= external-dns-huaweicloud-webhook
IMAGE_TAG ?= latest
IMAGE = $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

show: ## Show variables
	@echo "ARTIFACT_NAME: $(ARTIFACT_NAME)"
	@echo "REGISTRY: $(REGISTRY)"
	@echo "IMAGE: $(IMAGE)"

##@ Development

.PHONY: fmt
fmt: ## Format Go source files
	go fmt ./...
	goimports -w -local github.com/setoru/external-dns-huaweicloud-webhook .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	$(shell go env GOPATH)/bin/golangci-lint run ./...

.PHONY: tidy
tidy: ## Tidy and verify go modules
	go mod tidy
	go mod verify

##@ Build

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf ./dist ./build ./vendor

.PHONY: build
build: ## Build the binary
	CGO_ENABLED=0 go build -ldflags="-s -w" -o build/bin/$(ARTIFACT_NAME) ./cmd/webhook

.PHONY: run
run: build ## Build and run locally
	LOG_LEVEL=$(LOG_LEVEL) LOG_ENVIRONMENT=$(LOG_ENVIRONMENT) LOG_FORMAT=$(LOG_FORMAT) build/bin/$(ARTIFACT_NAME)

##@ Docker

.PHONY: docker-build
docker-build: ## Build the docker image
	docker build ./ -f Dockerfile -t $(IMAGE)

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(IMAGE)
