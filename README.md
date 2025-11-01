# MCP Architecture Service

A Model Context Protocol (MCP) server that provides structured access to architectural guidelines, patterns, and Architecture Decision Records (ADRs) stored in a Git repository.

## Overview

The MCP Architecture Service implements the MCP specification to enable AI agents and IDE integrations to discover and retrieve architectural documentation as contextual resources through JSON-RPC communication over stdio.

## Features

- **MCP Protocol Compliance**: Full implementation of MCP specification for resource discovery and retrieval
- **Documentation Categories**: Support for guidelines, design patterns, and ADRs
- **Real-time Updates**: Automatic detection and refresh of documentation changes
- **In-memory Caching**: Fast resource retrieval with automatic cache invalidation
- **Container Ready**: Docker containerization for consistent deployment

## Project Structure

```
.
├── cmd/
│   └── mcp-server/          # Main application entry point
├── internal/
│   ├── models/              # Data models and structures
│   └── server/              # MCP server implementation
├── pkg/
│   ├── cache/               # In-memory caching system
│   ├── monitor/             # File system monitoring
│   └── scanner/             # Documentation scanning and parsing
├── docs/                    # Documentation files (monitored)
│   ├── guidelines/          # Architectural guidelines
│   ├── patterns/            # Design patterns
│   └── adr/                 # Architecture Decision Records
├── Makefile                 # Build and development tasks
└── go.mod                   # Go module definition
```

## Quick Start

### Prerequisites

- Go 1.21 or later
- Make (optional, for using Makefile targets)

### Building

```bash
# Download dependencies
make deps

# Build the binary
make build

# Or build and run directly
make run
```

### Development

```bash
# Run in development mode
make dev

# Run tests
make test

# Format code
make fmt
```

### Docker

```bash
# Build Docker image
make docker-build

# Run in container
make docker-run
```

## MCP Resource URIs

The service exposes documentation through the following URI patterns:

- `architecture://guidelines/{path}` - Architectural guidelines
- `architecture://patterns/{path}` - Design patterns  
- `architecture://adr/{adr_id}` - Architecture Decision Records

## Configuration

The service monitors the following directories for documentation:

- `docs/guidelines/` - Architectural guidelines (Markdown files)
- `docs/patterns/` - Design patterns (Markdown files)
- `docs/adr/` - Architecture Decision Records (Markdown files)

## MCP Protocol Support

### Supported Methods

- `initialize` - Server initialization and capability negotiation
- `notifications/initialized` - Initialization acknowledgment
- `resources/list` - List all available documentation resources
- `resources/read` - Read specific documentation resource content

### Communication

The service communicates via JSON-RPC 2.0 over stdio, making it compatible with MCP clients and IDE integrations.

## Development

### Available Make Targets

- `make build` - Build the binary
- `make test` - Run tests
- `make clean` - Clean build artifacts
- `make deps` - Download dependencies
- `make run` - Build and run the application
- `make docker-build` - Build Docker image
- `make help` - Show all available targets

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage
```

## License

[Add your license information here]