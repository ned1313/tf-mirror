.PHONY: help build test clean run dev docker docker-build docker-run frontend backend lint

# Default target
help:
	@echo "Available targets:"
	@echo "  make build          - Build backend and frontend"
	@echo "  make backend        - Build backend only"
	@echo "  make frontend       - Build frontend only"
	@echo "  make test           - Run all tests"
	@echo "  make test-unit      - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make run            - Run the application"
	@echo "  make dev            - Run in development mode"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run Docker container"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make lint           - Run linters"

# Build everything
build: backend frontend
	@echo "Build complete"

# Build backend
backend:
	@echo "Building backend..."
	go build -o bin/terraform-mirror ./cmd/terraform-mirror

# Build frontend
frontend:
	@echo "Building frontend..."
	cd web && npm install && npm run build

# Run all tests
test: test-unit test-integration
	@echo "All tests passed"

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./test/integration/...

# Run the application
run: build
	@echo "Starting Terraform Mirror..."
	./bin/terraform-mirror

# Development mode
dev:
	@echo "Starting in development mode..."
	@echo "Backend will run on :8080"
	@echo "Frontend will run on :5173"
	@trap 'kill 0' EXIT; \
		(cd web && npm run dev) & \
		go run ./cmd/terraform-mirror

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -f deployments/docker/Dockerfile -t terraform-mirror:latest .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker-compose -f deployments/docker-compose/docker-compose.yml up

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf web/dist/
	rm -rf coverage.txt
	rm -rf *.db
	rm -rf data/
	rm -rf cache/

# Run linters
lint:
	@echo "Running linters..."
	go vet ./...
	gofmt -s -w .
	cd web && npm run lint

# Initialize database
init-db:
	@echo "Initializing database..."
	sqlite3 data/terraform-mirror.db < internal/database/migrations/001_initial.sql

# Generate mocks (if using mockery)
mocks:
	@echo "Generating mocks..."
	go generate ./...

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	cd web && npm install

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	cd web && npm run format
