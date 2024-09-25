DOCKER_COMPOSE ?= docker compose -p equiz
GO ?= go

##@ General

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Lint

lint: ## Check with 'golangci-lint'
	buf lint
	golangci-lint run

##@ Test

.PHONY: test
test: ## Run unit test
	@$(GO) test ./... -cover -race

##@ Development

up: ## Run local environment
	chmod -R +x devstack/postgres
	$(DOCKER_COMPOSE) up -d

down: ## Shutdown local environment
	@$(DOCKER_COMPOSE) down

build: ## Build the docker image
	$(DOCKER_COMPOSE) up -d --no-deps --build equiz

gen-api: ## Generate proto file
	buf generate

##@ Documentation

diagram-up: ## Start the c4 diagram server
	@docker run -it --rm -p 3030:8080 -v ./docs/:/usr/local/structurizr structurizr/lite