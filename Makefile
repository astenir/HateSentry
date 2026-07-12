.PHONY: all build run clean test test-integration deps docker-build docker-up docker-down docker-logs verify-compose smoke-moderation smoke-mvp-local install-deps

# Variables
BINARY_NAME=hatesentry
BUILD_DIR=bin
GO_FILES=$(shell find . -name '*.go' -type f ! -path '*/vendor/*')

# Default target
all: deps build

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run integration tests that need external services
test-integration:
	@echo "Running integration tests..."
	@go test -p 1 -v -tags=integration ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.txt ./...
	@go tool cover -html=coverage.txt -o coverage.html

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run ./...

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker-compose build

# Docker up
docker-up:
	@echo "Starting Docker containers..."
	@docker-compose up -d

# Docker down
docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down

# Docker logs
docker-logs:
	@docker-compose logs -f

# Verify Docker Compose runtime health
verify-compose:
	@echo "Building and starting Docker Compose stack..."
	@docker compose up -d --build
	@echo "Waiting for API health..."
	@for i in $$(seq 1 30); do \
		if response=$$(curl -fsS http://localhost:8080/api/v1/health 2>/dev/null); then \
			echo "$$response"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "API health check did not become healthy in time"; \
	docker compose ps; \
	docker compose logs --no-color --tail=80 hatesentry; \
	exit 1

# Docker restart
docker-restart: docker-down docker-up

# Database migration
migrate-up:
	@echo "Running database migrations..."
	@$(BUILD_DIR)/$(BINARY_NAME) migrate up

# Create sample user
create-user:
	@[ -n "$$ADMIN_BOOTSTRAP_TOKEN" ] || (echo "ADMIN_BOOTSTRAP_TOKEN is required for initial admin bootstrap" && exit 1)
	@echo "Creating initial admin user..."
	@curl -X POST http://localhost:8080/api/v1/auth/register \
		-H "Content-Type: application/json" \
		-d "{\"username\":\"admin\",\"email\":\"admin@example.com\",\"password\":\"password123\",\"admin_bootstrap_token\":\"$$ADMIN_BOOTSTRAP_TOKEN\"}"

# Health check
health:
	@echo "Checking health..."
	@curl http://localhost:8080/api/v1/health

# Test metrics endpoint
test-metrics:
	@echo "Testing metrics endpoint..."
	./scripts/test_metrics.sh

# Smoke-test the external client moderation workflow against a running API
smoke-moderation:
	@echo "Running external client moderation smoke workflow..."
	@python3 scripts/smoke_moderation_workflow.py

# Run a self-contained local MVP smoke workflow with a temporary database and OpenAI-compatible stub
smoke-mvp-local:
	@echo "Running local text moderation MVP smoke workflow..."
	@python3 scripts/smoke_mvp_local.py

# Install development tools
install-deps:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/air-verse/air@latest

# Generate API documentation
docs:
	@echo "Generating API documentation..."
	@swag init -g main.go

# Development run (with hot reload using air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	@air

# Clean all volumes
clean-all: docker-down
	@echo "Removing all volumes..."
	@docker volume rm hatesentry_mysql-data hatesentry_redis-data hatesentry_rabbitmq-data 2>/dev/null || true
