.PHONY: help proto build test docker-build docker-up docker-down migrate clean lint

# Variables
APP_NAME := uploadstream
DOCKER_IMAGE := $(APP_NAME):latest
DOCKER_REGISTRY := paulbabatuyi.io  
DB_URL := postgres://uploader:uploader@localhost:5432/uploadstream?sslmode=disable

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
RED := \033[0;31m
NC := \033[0m # No Color

## help: Display this help message
help:
	@echo "$(BLUE)UploadStream gRPC - Available Commands$(NC)"
	@echo ""
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/##//' | column -t -s ':'

## proto: Generate Go code from .proto files
proto:
	@echo "$(BLUE)[1/2] Generating protobuf code...$(NC)"
	buf generate
	@echo "$(GREEN)✓ Proto files generated$(NC)"

## build: Build the server binary
build:
	@echo "$(BLUE)[1/3] Building server...$(NC)"
	go build -o bin/$(APP_NAME) cmd/server/main.go
	@echo "$(GREEN)✓ Binary created: bin/$(APP_NAME)$(NC)"

## build-client: Build the client binary
build-client:
	@echo "$(BLUE)[1/2] Building client...$(NC)"
	go build -o bin/$(APP_NAME)-client cmd/client/main.go
	@echo "$(GREEN)✓ Binary created: bin/$(APP_NAME)-client$(NC)"

## run: Run the server locally
run: build
	@echo "$(BLUE)[1/2] Starting server...$(NC)"
	UPLOADSTREAM=$(DB_URL) ENV=development ./bin/$(APP_NAME)

## test: Run all tests
test:
	@echo "$(BLUE)[1/3] Running tests...$(NC)"
	go test -v -race -cover ./...
	@echo "$(GREEN)✓ All tests passed$(NC)"

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "$(BLUE)[1/4] Running tests with coverage...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"

## lint: Run linters
lint:
	@echo "$(BLUE)[1/3] Running linters...$(NC)"
	buf lint
	golangci-lint run
	@echo "$(GREEN)✓ No linting issues$(NC)"

## migrate-up: Run database migrations
migrate-up:
	@echo "$(BLUE)[1/2] Running migrations...$(NC)"
	@for file in migrations/*.up.sql; do \
		echo "  Applying $$file..."; \
		psql $(DB_URL) -f $$file; \
	done
	@echo "$(GREEN)✓ Migrations applied$(NC)"

## migrate-down: Rollback database migrations
migrate-down:
	@echo "$(BLUE)[1/2] Rolling back migrations...$(NC)"
	@for file in $$(ls -r migrations/*.down.sql); do \
		echo "  Reverting $$file..."; \
		psql $(DB_URL) -f $$file; \
	done
	@echo "$(GREEN)✓ Migrations rolled back$(NC)"

## docker-build: Build Docker image
docker-build:
	@echo "$(BLUE)[1/3] Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE) .
	@echo "$(GREEN)✓ Docker image built: $(DOCKER_IMAGE)$(NC)"

## docker-build-no-cache: Build Docker image without cache
docker-build-no-cache:
	@echo "$(BLUE)[1/3] Building Docker image (no cache)...$(NC)"
	docker build --no-cache -t $(DOCKER_IMAGE) .
	@echo "$(GREEN)✓ Docker image built$(NC)"

## docker-up: Start all services with Docker Compose
docker-up:
	@echo "$(BLUE)[1/3] Starting services...$(NC)"
	docker-compose up -d
	@echo "$(GREEN)✓ Services started$(NC)"
	@echo ""
	@echo "Services available at:"
	@echo "  - gRPC Server:    localhost:50051"
	@echo "  - Metrics:        http://localhost:9090/metrics"
	@echo "  - Prometheus:     http://localhost:9091"
	@echo "  - Grafana:        http://localhost:3000 (admin/admin)"
	@echo "  - Jaeger:         http://localhost:16686"

## docker-down: Stop all services
docker-down:
	@echo "$(BLUE)[1/2] Stopping services...$(NC)"
	docker-compose down
	@echo "$(GREEN)✓ Services stopped$(NC)"

## docker-logs: View logs from all services
docker-logs:
	docker-compose logs -f

## docker-clean: Remove all containers, volumes, and images
docker-clean:
	@echo "$(BLUE)[1/4] Cleaning up Docker resources...$(NC)"
	docker-compose down -v --rmi all
	@echo "$(GREEN)✓ Docker resources cleaned$(NC)"

## k8s-deploy: Deploy to Kubernetes
k8s-deploy:
	@echo "$(BLUE)[1/3] Deploying to Kubernetes...$(NC)"
	kubectl apply -f k8s/
	@echo "$(GREEN)✓ Deployed to Kubernetes$(NC)"

## k8s-delete: Delete Kubernetes deployment
k8s-delete:
	@echo "$(BLUE)[1/2] Deleting Kubernetes resources...$(NC)"
	kubectl delete -f k8s/
	@echo "$(GREEN)✓ Resources deleted$(NC)"

## k8s-logs: View Kubernetes logs
k8s-logs:
	kubectl logs -f deployment/uploadstream -n uploadstream

## bench: Run benchmarks
bench:
	@echo "$(BLUE)[1/2] Running benchmarks...$(NC)"
	go test -bench=. -benchmem ./...
	@echo "$(GREEN)✓ Benchmarks complete$(NC)"

## install-tools: Install development tools
install-tools:
	@echo "$(BLUE)[1/5] Installing tools...$(NC)"
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "$(GREEN)✓ Tools installed$(NC)"

## clean: Remove build artifacts
clean:
	@echo "$(BLUE)[1/3] Cleaning build artifacts...$(NC)"
	rm -rf bin/
	rm -rf coverage.out coverage.html
	rm -rf data/files/*
	@echo "$(GREEN)✓ Build artifacts removed$(NC)"

## deps: Download Go dependencies
deps:
	@echo "$(BLUE)[1/2] Downloading dependencies...$(NC)"
	go mod download
	go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

## format: Format code
format:
	@echo "$(BLUE)[1/2] Formatting code...$(NC)"
	gofmt -s -w .
	@echo "$(GREEN)✓ Code formatted$(NC)"

## dev: Start development environment
dev: docker-up
	@echo "$(BLUE)[1/2] Starting development server...$(NC)"
	@sleep 5  # Wait for database
	$(MAKE) migrate-up
	$(MAKE) run

## ci: Run CI checks (used in CI/CD)
ci: proto lint test

## release: Build and tag a release
release:
	@echo "$(BLUE)[1/4] Creating release...$(NC)"
	@read -p "Enter version (e.g., v1.0.0): " version; \
	git tag -a $$version -m "Release $$version"; \
	git push origin $$version; \
	docker build -t $(DOCKER_REGISTRY)/$(APP_NAME):$$version .; \
	docker push $(DOCKER_REGISTRY)/$(APP_NAME):$$version
	@echo "$(GREEN)✓ Release created$(NC)"

# Default target
.DEFAULT_GOAL := help