# SAI Auth Microservice Makefile
define load_env
	@if [ -f ./.env ]; then set -a && . ./.env && set +a; fi &&
endef

# Build configuration
BINARY_NAME?=$(SERVICE_NAME)
VERSION?=$(SERVICE_VERSION)

# Go configuration
GO_VERSION=1.24
GOOS=linux
GOARCH=amd64
CGO_ENABLED=1

# Docker configuration
DOCKER_IMAGE?=$(SERVICE_NAME)
DOCKER_TAG=latest

# Environment configuration
ENV_FILE=.env

# Default target
.DEFAULT_GOAL := help

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

## Help
.PHONY: help
help: ## Show this help message
	@echo "$(GREEN)SAI Auth Microservice$(NC)"
	@echo "$(YELLOW)Available commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Environment Setup
.PHONY: env
env: ## Create .env file from .env.example
	@echo "$(YELLOW)Setting up environment file...$(NC)"
	@if [ -f ".env" ]; then \
		echo "$(GREEN).env file already exists!$(NC)"; \
	elif [ -f ".env.example" ]; then \
		cp .env.example .env; \
		echo "$(GREEN).env file created from .env.example$(NC)"; \
		echo "$(YELLOW)Please edit .env file with your configuration$(NC)"; \
	else \
		echo "$(RED)Error: .env.example file not found!$(NC)"; \
		echo "$(YELLOW)Please create .env.example file first or create .env manually$(NC)"; \
		exit 1; \
	fi

## Development
.PHONY: deps
deps: ## Download Go dependencies
	@echo "$(YELLOW)Downloading Go dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Dependencies downloaded!$(NC)"

.PHONY: config
config: env ## Generate config.yml from template using environment variables
	@echo "$(YELLOW)Generating configuration from template...$(NC)"
	@if [ ! -f "config.template.yml" ]; then \
		echo "$(RED)Error: config.template.yml not found!$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Loading environment variables from $(ENV_FILE)...$(NC)"
	@set -a; . ./$(ENV_FILE); set +a; envsubst < ./config.template.yml > ./config.yml
	@echo "$(GREEN)Configuration generated at ./config.yml$(NC)"

.PHONY: reconfig
reconfig: ## Force regenerate config with current .env values
	@if command -v envsubst >/dev/null 2>&1; then \
		set -a && . ./.env && set +a && envsubst < config.template.yml > config.yml; \
		echo "$(GREEN)Config file regenerated with current environment variables.$(NC)"; \
	else \
		echo "$(RED)envsubst not found. Please install gettext package.$(NC)"; \
		echo "On Ubuntu/Debian: sudo apt-get install gettext-base"; \
		echo "On macOS: brew install gettext"; \
		exit 1; \
	fi

## Build
.PHONY: build
build: config ## Build the application binary
	@echo "$(YELLOW)Building $(BINARY_NAME)...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags="-w -s -X main.version=$(VERSION) -extldflags '-static'" \
		-a -installsuffix cgo \
		-o $(BINARY_NAME) \
		./cmd
	@echo "$(GREEN)Build complete: $(BINARY_NAME)$(NC)"

## Run
.PHONY: run
run: config ## Run the application locally
	@echo "$(YELLOW)Starting $(SERVICE_NAME) locally...$(NC)"
	@if [ ! -f "./config.yml" ]; then \
		echo "$(RED)Configuration not found. Generating...$(NC)"; \
		$(MAKE) config; \
	fi
	@go run ./cmd

## Docker
.PHONY: docker
docker: config ## Build and start all services with docker-compose
	@echo "$(YELLOW)Building and starting all services...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)All services started!$(NC)"

## Docker Compose
.PHONY: up
up: ## Start all services with docker-compose
	@echo "$(YELLOW)Starting all services...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)Services started!$(NC)"

.PHONY: down
down: ## Stop all services
	@echo "$(YELLOW)Stopping all services...$(NC)"
	@docker-compose down
	@echo "$(GREEN)Services stopped!$(NC)"

.PHONY: restart
restart: clean-all ## Clean, rebuild and restart all services
	@echo "$(YELLOW)Restarting services with full rebuild...$(NC)"
	@docker-compose down -v
	@docker-compose build --no-cache
	@docker-compose up -d
	@echo "$(GREEN)Services restarted with full rebuild!$(NC)"

.PHONY: logs
logs: ## Show logs from all services
	@docker-compose logs -f

.PHONY: logs-app
logs-app: ## Show logs from application only
	@docker-compose logs -f sai-auth

.PHONY: logs-redis
logs-mongo: ## Show logs from MongoDB only
	@docker-compose logs -f redis

## Redis Management
.PHONY: redis-cli
redis-cli: ## Connect to Redis CLI
	@echo "$(YELLOW)Connecting to Redis CLI...$(NC)"
	@docker-compose exec redis redis-cli

.PHONY: redis-reset
redis-reset: ## Reset Redis data (WARNING: This will delete all data!)
	@echo "$(RED)WARNING: This will delete all Redis data!$(NC)"
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ]
	@echo "$(YELLOW)Stopping services and removing Redis data...$(NC)"
	@docker-compose down
	@docker volume rm sai-auth_redis_data || true
	@echo "$(GREEN)Redis data reset complete!$(NC)"

## Code Quality
.PHONY: lint
lint: ## Run linter
	@echo "$(YELLOW)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(RED)golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(YELLOW)Formatting Go code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)Code formatted!$(NC)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet ./...

## Cleanup
.PHONY: clean
clean: ## Clean build artifacts and generated files
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -f config.yml
	@echo "$(GREEN)Cleanup complete!$(NC)"

.PHONY: clean-docker
clean-docker: ## Clean Docker images and volumes
	@echo "$(YELLOW)Cleaning Docker resources...$(NC)"
	@docker-compose down -v --remove-orphans
	@echo "$(GREEN)Docker cleanup complete!$(NC)"

.PHONY: clean-all
clean-all: clean clean-docker ## Clean everything

## Health & Status
.PHONY: health
health: ## Check health of the application
	@echo "$(YELLOW)Checking application health...$(NC)"
	@curl -s http://localhost:8081/health | jq . || echo "$(RED)Service not responding$(NC)"

.PHONY: status
status: ## Show status of all services
	@echo "$(YELLOW)Service status:$(NC)"
	@docker-compose ps

.PHONY: version
version: ## Show version information
	@echo "$(GREEN)SAI Auth Microservice$(NC)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(GO_VERSION)"

