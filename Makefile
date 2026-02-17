.PHONY: help build run stop restart logs clean test db-migrate db-reset docker-build docker-up docker-down docker-logs docker-clean tools

# Variables
DOCKER_COMPOSE := docker compose
GO := go
API_BINARY := bin/voronka-api
WORKERS_BINARY := bin/voronka-workers

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Vortex (Voronka) - Development Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# ==========================================
# Local Development (without Docker)
# ==========================================

build: ## Build API and Workers binaries
	@echo "Building binaries..."
	@mkdir -p bin
	@$(GO) build -o $(API_BINARY) ./cmd/api
	@$(GO) build -o $(WORKERS_BINARY) ./cmd/workers
	@echo "✓ Binaries built successfully!"

run-api: ## Run API server locally
	@echo "Starting API server..."
	@$(GO) run cmd/api/main.go

run-workers: ## Run workers locally
	@echo "Starting workers..."
	@$(GO) run cmd/workers/main.go

test: ## Run tests
	@echo "Running tests..."
	@$(GO) test -v -race ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

lint: ## Run linters
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: brew install golangci-lint"; exit 1)
	@golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "✓ Code formatted!"

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.txt coverage.html
	@echo "✓ Clean complete!"

# ==========================================
# Docker Development
# ==========================================

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@$(DOCKER_COMPOSE) build
	@echo "✓ Docker images built successfully!"

docker-up: ## Start all services with Docker Compose
	@echo "Starting services..."
	@$(DOCKER_COMPOSE) up -d
	@echo "✓ Services started!"
	@echo ""
	@echo "Services available at:"
	@echo "  - API:          http://localhost:8080"
	@echo "  - PostgreSQL:   localhost:5432"
	@echo "  - Redis:        localhost:6379"
	@echo ""
	@echo "Run 'make docker-logs' to view logs"

docker-down: ## Stop all services
	@echo "Stopping services..."
	@$(DOCKER_COMPOSE) down
	@echo "✓ Services stopped!"

docker-restart: docker-down docker-up ## Restart all services

docker-logs: ## View logs from all services
	@$(DOCKER_COMPOSE) logs -f

docker-logs-api: ## View API logs only
	@$(DOCKER_COMPOSE) logs -f api

docker-logs-workers: ## View Workers logs only
	@$(DOCKER_COMPOSE) logs -f workers

docker-ps: ## List running containers
	@$(DOCKER_COMPOSE) ps

docker-clean: ## Stop and remove all containers, networks, and volumes
	@echo "Cleaning up Docker resources..."
	@$(DOCKER_COMPOSE) down -v --remove-orphans
	@docker system prune -f
	@echo "✓ Docker cleanup complete!"

docker-rebuild: docker-clean docker-build docker-up ## Rebuild and restart everything

# ==========================================
# Docker Tools (pgAdmin, Redis Commander)
# ==========================================

tools: ## Start development tools (pgAdmin, Redis Commander)
	@echo "Starting development tools..."
	@$(DOCKER_COMPOSE) --profile tools up -d
	@echo "✓ Tools started!"
	@echo ""
	@echo "Tools available at:"
	@echo "  - pgAdmin:          http://localhost:5050 (admin@voronka.local / admin)"
	@echo "  - Redis Commander:  http://localhost:8081"

tools-down: ## Stop development tools
	@echo "Stopping development tools..."
	@$(DOCKER_COMPOSE) --profile tools down
	@echo "✓ Tools stopped!"

# ==========================================
# Database Migrations (Versioned)
# ==========================================

migrate-up: ## Run all pending migrations
	@echo "Running migrations..."
	@$(GO) run cmd/migrate/main.go up

migrate-down: ## Rollback last migration
	@echo "Rolling back migration..."
	@$(GO) run cmd/migrate/main.go down

migrate-steps: ## Run N migration steps (usage: make migrate-steps N=2)
	@echo "Running $(N) migration steps..."
	@$(GO) run cmd/migrate/main.go steps $(N)

migrate-version: ## Show current migration version
	@$(GO) run cmd/migrate/main.go version

migrate-force: ## Force migration version (usage: make migrate-force VERSION=5)
	@echo "Forcing migration version to $(VERSION)..."
	@$(GO) run cmd/migrate/main.go force $(VERSION)

migrate-create: ## Create new migration (usage: make migrate-create NAME=add_user_avatar)
	@$(GO) run cmd/migrate/main.go create $(NAME)

# ==========================================
# Database Management
# ==========================================

db-migrate: migrate-up ## Alias for migrate-up

db-reset: ## Reset database and run migrations
	@echo "Resetting database..."
	@$(DOCKER_COMPOSE) exec postgres psql -U postgres -c "DROP DATABASE IF EXISTS voronka;"
	@$(DOCKER_COMPOSE) exec postgres psql -U postgres -c "CREATE DATABASE voronka;"
	@echo "Running migrations..."
	@$(GO) run cmd/migrate/main.go up
	@echo "✓ Database reset complete!"

db-shell: ## Connect to PostgreSQL shell
	@$(DOCKER_COMPOSE) exec postgres psql -U postgres -d voronka

redis-cli: ## Connect to Redis CLI
	@$(DOCKER_COMPOSE) exec redis redis-cli

# ==========================================
# Development Workflow
# ==========================================

dev: docker-up tools ## Start full development environment
	@echo ""
	@echo "🚀 Development environment is ready!"
	@echo ""
	@echo "Services:"
	@echo "  - API:              http://localhost:8080"
	@echo "  - API Health:       http://localhost:8080/health"
	@echo "  - PostgreSQL:       localhost:5432"
	@echo "  - Redis:            localhost:6379"
	@echo ""
	@echo "Tools:"
	@echo "  - pgAdmin:          http://localhost:5050"
	@echo "  - Redis Commander:  http://localhost:8081"
	@echo ""
	@echo "Run 'make docker-logs' to view logs"

stop: docker-down tools-down ## Stop all services and tools

status: ## Show status of all services
	@echo "Service Status:"
	@$(DOCKER_COMPOSE) ps
	@echo ""
	@echo "Docker Images:"
	@docker images | grep voronka || echo "No Vortex images found"

# ==========================================
# Utilities
# ==========================================

check-health: ## Check health of all services
	@echo "Checking API health..."
	@curl -s http://localhost:8080/health | jq '.' || echo "API not responding"
	@echo ""
	@echo "Checking PostgreSQL..."
	@$(DOCKER_COMPOSE) exec postgres pg_isready -U postgres || echo "PostgreSQL not ready"
	@echo ""
	@echo "Checking Redis..."
	@$(DOCKER_COMPOSE) exec redis redis-cli ping || echo "Redis not responding"

watch-logs: ## Watch logs with pretty printing
	@$(DOCKER_COMPOSE) logs -f --tail=100 | grep --color=auto -E 'ERROR|WARN|INFO|$$'

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@echo "✓ Dependencies downloaded!"

deps-tidy: ## Tidy Go dependencies
	@echo "Tidying dependencies..."
	@$(GO) mod tidy
	@echo "✓ Dependencies tidied!"

deps-vendor: ## Vendor dependencies
	@echo "Vendoring dependencies..."
	@$(GO) mod vendor
	@echo "✓ Dependencies vendored!"