---
inclusion: always
---

# Technology Stack & Build System

## Stack

- **Go 1.21+** - Use standard library where possible
- **fsnotify v1.7.0** - File system monitoring (only external dependency)
- **JSON-RPC 2.0 over stdio** - No HTTP, no network ports
- **Alpine Linux containers** - Production runtime

## Build Commands

Use Makefile for all operations:

```bash
# Development
make dev           # Run with hot reload
make test          # Run all tests
make test-coverage # Generate coverage report

# Building
make build         # Build to ./bin/mcp-server
make fmt           # Format code (run before commits)
make vet           # Static analysis

# Docker
make docker-build  # Build container image
make docker-run    # Run containerized server

# Kubernetes
make k8s-deploy    # Deploy to cluster
```

## Build Requirements

- **CGO disabled** (`CGO_ENABLED=0`) - Static binaries only
- **Build flags**: `-ldflags="-s -w"` - Strip debug info
- **Target**: Linux AMD64 for containers
- **Output**: `./bin/mcp-server`

## Security Constraints

When writing code, remember:
- Run as non-root (UID 1001) in containers
- Read-only root filesystem - no file writes outside /tmp
- No network listeners - stdio communication only
- Resource limits: 256M memory, 0.2 CPU