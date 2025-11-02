# Technology Stack & Build System

## Core Technologies

- **Language**: Go 1.21+
- **Protocol**: Model Context Protocol (MCP) - JSON-RPC 2.0 over stdio
- **Runtime**: Alpine Linux containers
- **File Monitoring**: fsnotify library for real-time documentation updates

## Dependencies

```go
// Core dependencies from go.mod
github.com/fsnotify/fsnotify v1.7.0  // File system monitoring
golang.org/x/sys v0.4.0              // System calls (indirect)
```

## Build System

Uses Make for build automation. Key commands:

### Development
```bash
make dev          # Run in development mode
make run          # Build and run
make test         # Run tests
make test-coverage # Run tests with coverage report
```

### Building
```bash
make build        # Build binary to ./bin/mcp-server
make build-linux  # Build for Linux (Docker compatible)
make deps         # Download dependencies
make tidy         # Clean up go.mod
```

### Code Quality
```bash
make fmt          # Format code
make vet          # Vet code
make lint         # Lint code (requires golangci-lint)
```

### Docker
```bash
make docker-build        # Build Docker image
make docker-build-secure # Build with security scanning
make docker-run          # Run container
make docker-run-secure   # Run with enhanced security
make docker-test         # Test MCP initialization
```

### Kubernetes
```bash
make k8s-deploy          # Deploy to Kubernetes
make k8s-undeploy        # Remove deployment
make k8s-test-security   # Test security configuration
```

## Build Configuration

- **CGO**: Disabled (`CGO_ENABLED=0`) for static binaries
- **Build Flags**: `-ldflags="-s -w"` for smaller binaries
- **Target**: Linux AMD64 for container deployment
- **Binary Location**: `./bin/mcp-server`

## Container Security

- Multi-stage Docker builds
- Non-root user (UID 1001)
- Read-only root filesystem
- Security options: `no-new-privileges:true`
- Resource limits: 256M memory, 0.2 CPU
- Health checks via process monitoring