# Project Structure & Organization

## Directory Layout

```
.
├── cmd/
│   └── mcp-server/          # Main application entry point
│       ├── main.go          # Application bootstrap and signal handling
│       └── main_test.go     # Main package tests
├── internal/                # Private application code
│   ├── models/              # Data models and structures
│   │   ├── document.go      # Document representation models
│   │   ├── mcp.go          # MCP protocol message structures
│   │   └── mcp_test.go     # MCP model tests
│   └── server/              # MCP server implementation
│       ├── server.go        # Core server logic and message handling
│       └── server_test.go   # Server tests
├── pkg/                     # Public/reusable packages
│   ├── cache/               # In-memory caching system
│   ├── monitor/             # File system monitoring
│   └── scanner/             # Documentation scanning and parsing
├── docs/                    # Documentation files (monitored by service)
│   ├── guidelines/          # Architectural guidelines
│   ├── patterns/            # Design patterns
│   └── adr/                 # Architecture Decision Records
├── bin/                     # Built binaries
├── Dockerfile               # Container build configuration
├── docker-compose.yml       # Local container orchestration
├── k8s-deployment.yaml      # Kubernetes deployment manifests
├── Makefile                 # Build automation
├── go.mod                   # Go module definition
└── go.sum                   # Dependency checksums
```

## Architecture Patterns

### Package Organization
- **cmd/**: Application entry points (main packages)
- **internal/**: Private application code, not importable by external packages
- **pkg/**: Public packages that could be imported by other projects
- **docs/**: Content directory monitored by the service

### Code Organization Principles
- **Separation of Concerns**: Models, server logic, and utilities in separate packages
- **Dependency Direction**: Internal packages depend on pkg packages, not vice versa
- **Interface-Based Design**: Use interfaces for testability and modularity
- **Error Handling**: Explicit error handling following Go conventions

### MCP Protocol Structure
- **Message Handling**: Centralized in `internal/server/server.go`
- **Protocol Models**: Defined in `internal/models/mcp.go`
- **Resource Management**: Implemented across cache, monitor, and scanner packages

### Testing Strategy
- Unit tests alongside source files (`*_test.go`)
- Integration tests in `cmd/` for end-to-end scenarios
- Test coverage reporting via `make test-coverage`

### Security Considerations
- Non-root container execution (UID 1001)
- Read-only root filesystem with writable tmpfs mounts
- Input validation for MCP resource URIs to prevent path traversal
- No network exposure (stdio-only communication)