# Makefile for MCP Architecture Service

# Variables
BINARY_NAME=mcp-server
BINARY_PATH=./bin/$(BINARY_NAME)
MAIN_PATH=./cmd/mcp-server
DOCKER_IMAGE=mcp-architecture-service
DOCKER_TAG=latest

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
BUILD_FLAGS=-ldflags="-s -w"
CGO_ENABLED=0

.PHONY: all build clean test deps tidy run docker-build docker-run help

# Default target
all: clean deps build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PATH)
	@echo "Binary built: $(BINARY_PATH)"

# Build for Linux (useful for Docker)
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_PATH)-linux $(MAIN_PATH)
	@echo "Linux binary built: $(BINARY_PATH)-linux"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf bin/
	@echo "Clean completed"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -d ./...

# Tidy up go.mod
tidy:
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	$(BINARY_PATH)

# Run in development mode (with file watching)
dev:
	@echo "Running in development mode..."
	$(GOCMD) run $(MAIN_PATH)

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Run Docker container (MCP server communicates via stdio)
# Note: MCP servers are typically invoked by MCP clients, not run directly
docker-run:
	@echo "Running Docker container..."
	@echo "Note: MCP server will wait for JSON-RPC messages on stdin"
	docker run --rm -i \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Run Docker container with volume mount for development
docker-run-dev:
	@echo "Running Docker container with development volume..."
	@echo "Note: MCP server will wait for JSON-RPC messages on stdin"
	docker run --rm -i \
		-v $(PWD)/docs:/app/docs:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Test Docker container with a simple MCP initialization message
docker-test:
	@echo "Testing Docker container with MCP initialization..."
	@timeout 10s bash -c 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"test\",\"version\":\"1.0.0\"}}}" | docker run --rm -i $(DOCKER_IMAGE):$(DOCKER_TAG)' || true
	@echo "Test completed (container automatically stopped)"

# Test Docker container and show full interaction
docker-test-verbose:
	@echo "Testing Docker container with verbose output..."
	@echo "Sending initialization message and waiting for response..."
	@timeout 10s bash -c 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"test\",\"version\":\"1.0.0\"}}}" | docker run --rm -i $(DOCKER_IMAGE):$(DOCKER_TAG); echo "Exit code: $$?"' || echo "Container stopped after timeout"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run

# Vet code
vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

# Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint

# Show help
help:
	@echo "Available targets:"
	@echo "  all           - Clean, download deps, and build"
	@echo "  build         - Build the binary"
	@echo "  build-linux   - Build the binary for Linux"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  deps          - Download dependencies"
	@echo "  tidy          - Tidy go.mod"
	@echo "  run           - Build and run the application"
	@echo "  dev           - Run in development mode"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  docker-test   - Test Docker container with MCP initialization (auto-stops)"
	@echo "  docker-test-verbose - Test Docker container with verbose output"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  vet           - Vet code"
	@echo "  install-tools - Install development tools"
	@echo "  help          - Show this help message"