# Makefile for MCP Architecture Service

# Variables
BINARY_NAME=mcp-server
BINARY_PATH=./bin/$(BINARY_NAME)
MAIN_PATH=./cmd/mcp-server

# Bridge server variables
BRIDGE_BINARY_NAME=mcp-bridge
BRIDGE_BINARY_PATH=./bin/$(BRIDGE_BINARY_NAME)
BRIDGE_MAIN_PATH=./cmd/mcp-bridge

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

# Build the bridge server binary
build-bridge:
	@echo "Building $(BRIDGE_BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(BUILD_FLAGS) -o $(BRIDGE_BINARY_PATH) $(BRIDGE_MAIN_PATH)
	@echo "Bridge server binary built: $(BRIDGE_BINARY_PATH)"

# Build all binaries
build-all: build build-bridge

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

# Run tests (excludes load tests and benchmarks)
test:
	@echo "Running tests..."
	$(GOTEST) -v -short ./...

# Run tests with coverage (excludes load tests and benchmarks)
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -short -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run performance tests (load tests and benchmarks)
test-performance:
	@echo "Running performance tests..."
	@echo "Running load tests..."
	@$(GOTEST) -run="TestLoad" ./cmd/mcp-server/... > /tmp/load_test.log 2>&1 && (echo "Load tests passed" && tail -15 /tmp/load_test.log) || (echo "Load tests failed" && tail -20 /tmp/load_test.log && exit 1)
	@echo "Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem -benchtime=100ms -run=^$$ -skip="BenchmarkConcurrent|BenchmarkMemory" ./internal/server/... > /tmp/bench_test.log 2>&1 && (echo "Benchmarks passed" && tail -25 /tmp/bench_test.log) || (echo "Benchmarks failed" && tail -30 /tmp/bench_test.log && exit 1)
	@echo "Performance tests completed"

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

# Run the bridge server
run-bridge: build build-bridge
	@echo "Running $(BRIDGE_BINARY_NAME)..."
	$(BRIDGE_BINARY_PATH)

# Run the bridge server with custom port
run-bridge-port: build build-bridge
	@echo "Running $(BRIDGE_BINARY_NAME) on port 8081..."
	$(BRIDGE_BINARY_PATH) -port 8081

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

# Security-focused Docker build with enhanced security scanning
docker-build-secure:
	@echo "Building secure Docker image with security scanning..."
	docker build --no-cache -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Running security scan on Docker image..."
	@if command -v docker >/dev/null 2>&1; then \
		docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
			-v $(PWD):/app aquasec/trivy:latest image $(DOCKER_IMAGE):$(DOCKER_TAG) || true; \
	fi
	@echo "Secure Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Test Docker container security configuration
docker-test-security:
	@echo "Testing Docker container security configuration..."
	@echo "Checking non-root user execution..."
	@docker run --rm $(DOCKER_IMAGE):$(DOCKER_TAG) id || true
	@echo "Checking read-only filesystem..."
	@docker run --rm $(DOCKER_IMAGE):$(DOCKER_TAG) sh -c 'touch /test 2>&1 || echo "Read-only filesystem working correctly"' || true
	@echo "Checking process capabilities..."
	@docker run --rm $(DOCKER_IMAGE):$(DOCKER_TAG) sh -c 'cat /proc/self/status | grep Cap' || true
	@echo "Security test completed"

# Run Docker container with security options for testing
docker-run-secure:
	@echo "Running Docker container with enhanced security options..."
	docker run --rm -i \
		--security-opt=no-new-privileges:true \
		--read-only \
		--tmpfs /app/tmp:size=100M,noexec,nosuid,nodev \
		--tmpfs /app/logs:size=100M,noexec,nosuid,nodev \
		--user 1001:1001 \
		--memory=256m \
		--cpus=0.2 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Test health check functionality
docker-test-health:
	@echo "Testing Docker health check functionality..."
	@echo "Starting container with health check..."
	@docker run -d --name mcp-health-test $(DOCKER_IMAGE):$(DOCKER_TAG) || true
	@sleep 10
	@echo "Checking health status..."
	@docker inspect --format='{{.State.Health.Status}}' mcp-health-test || true
	@echo "Health check logs:"
	@docker inspect --format='{{range .State.Health.Log}}{{.Output}}{{end}}' mcp-health-test || true
	@echo "Cleaning up test container..."
	@docker rm -f mcp-health-test || true

# Deploy to Kubernetes with security configurations
k8s-deploy:
	@echo "Deploying to Kubernetes with security configurations..."
	kubectl apply -f k8s-deployment.yaml
	@echo "Waiting for deployment to be ready..."
	kubectl rollout status deployment/mcp-architecture-service
	@echo "Checking pod security context..."
	kubectl get pods -l app=mcp-architecture-service -o jsonpath='{.items[0].spec.securityContext}' | jq .
	@echo "Deployment completed"

# Remove Kubernetes deployment
k8s-undeploy:
	@echo "Removing Kubernetes deployment..."
	kubectl delete -f k8s-deployment.yaml --ignore-not-found=true
	@echo "Deployment removed"

# Test Kubernetes deployment security
k8s-test-security:
	@echo "Testing Kubernetes deployment security..."
	@echo "Checking pod security context..."
	kubectl get pods -l app=mcp-architecture-service -o jsonpath='{.items[0].spec.securityContext}' | jq .
	@echo "Checking container security context..."
	kubectl get pods -l app=mcp-architecture-service -o jsonpath='{.items[0].spec.containers[0].securityContext}' | jq .
	@echo "Checking resource limits..."
	kubectl get pods -l app=mcp-architecture-service -o jsonpath='{.items[0].spec.containers[0].resources}' | jq .
	@echo "Security test completed"

# Run Docker Compose with security configurations
compose-up:
	@echo "Starting services with Docker Compose (secure configuration)..."
	docker-compose up -d
	@echo "Services started"

# Stop Docker Compose services
compose-down:
	@echo "Stopping Docker Compose services..."
	docker-compose down
	@echo "Services stopped"

# Test Docker Compose security configuration
compose-test-security:
	@echo "Testing Docker Compose security configuration..."
	@echo "Checking container security options..."
	docker inspect mcp-architecture-service | jq '.[0].HostConfig.SecurityOpt'
	@echo "Checking read-only filesystem..."
	docker inspect mcp-architecture-service | jq '.[0].HostConfig.ReadonlyRootfs'
	@echo "Checking resource limits..."
	docker inspect mcp-architecture-service | jq '.[0].HostConfig.Memory, .[0].HostConfig.CpuQuota'
	@echo "Security test completed"

# Show help
help:
	@echo "Available targets:"
	@echo "  all                    - Clean, download deps, and build"
	@echo "  build                  - Build the binary"
	@echo "  build-bridge           - Build the MCP bridge server binary"
	@echo "  build-all              - Build all binaries"
	@echo "  build-linux            - Build the binary for Linux"
	@echo "  clean                  - Clean build artifacts"
	@echo "  test                   - Run tests (excludes load tests and benchmarks)"
	@echo "  test-coverage          - Run tests with coverage report (excludes load tests and benchmarks)"
	@echo "  test-performance       - Run performance tests (load tests and benchmarks)"
	@echo "  deps                   - Download dependencies"
	@echo "  tidy                   - Tidy go.mod"
	@echo "  run                    - Build and run the application"
	@echo "  run-bridge             - Build and run the MCP bridge server"
	@echo "  run-bridge-port        - Run the MCP bridge server on port 8081"
	@echo "  dev                    - Run in development mode"
	@echo "  docker-build           - Build Docker image"
	@echo "  docker-build-secure    - Build Docker image with security scanning"
	@echo "  docker-run             - Run Docker container"
	@echo "  docker-run-secure      - Run Docker container with enhanced security"
	@echo "  docker-test            - Test Docker container with MCP initialization"
	@echo "  docker-test-security   - Test Docker container security configuration"
	@echo "  docker-test-health     - Test Docker health check functionality"
	@echo "  k8s-deploy             - Deploy to Kubernetes with security configs"
	@echo "  k8s-undeploy           - Remove Kubernetes deployment"
	@echo "  k8s-test-security      - Test Kubernetes deployment security"
	@echo "  compose-up             - Start services with Docker Compose"
	@echo "  compose-down           - Stop Docker Compose services"
	@echo "  compose-test-security  - Test Docker Compose security configuration"
	@echo "  fmt                    - Format code"
	@echo "  lint                   - Lint code"
	@echo "  vet                    - Vet code"
	@echo "  install-tools          - Install development tools"
	@echo "  help                   - Show this help message"