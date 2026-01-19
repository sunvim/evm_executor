.PHONY: help build run test clean docker-build docker-run fmt lint

# Default target
help:
	@echo "Available targets:"
	@echo "  build         - Build the executor binary"
	@echo "  run           - Run the executor"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"

# Build the binary
build:
	@echo "Building executor..."
	go build -o bin/executor ./cmd/executor

# Run the executor
run: build
	@echo "Running executor..."
	./bin/executor -config config/config.yaml

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/ coverage.txt coverage.html

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t evm-executor:latest -f deployments/docker/Dockerfile .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker-compose -f deployments/docker/docker-compose.yaml up

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate code coverage report
coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"
