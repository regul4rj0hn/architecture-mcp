# MCP Architecture Service

A Model Context Protocol (MCP) server that provides structured access to architectural guidelines, patterns, and Architecture Decision Records (ADRs) through JSON-RPC communication.

## Overview

This service implements the MCP specification to enable AI agents and IDE integrations to discover and retrieve architectural documentation as contextual resources. It monitors documentation directories for real-time updates and provides fast, cached access to your architecture knowledge base.

## Available MCP Resources

The service exposes documentation through the following URI patterns:

- `architecture://guidelines/{path}` - Architectural guidelines
- `architecture://patterns/{path}` - Design patterns  
- `architecture://adr/{adr_id}` - Architecture Decision Records

## MCP Protocol Support

- `initialize` - Server initialization and capability negotiation
- `notifications/initialized` - Initialization acknowledgment
- `resources/list` - List all available documentation resources
- `resources/read` - Read specific documentation resource content

Communication via JSON-RPC 2.0 over stdio (local) or TCP (bridge mode).

## Quick Start

### Prerequisites

- Go 1.21 or later
- VSCode or a fork (or any IDE that support AI Agents with MCP configuration)

### Build

```bash
make build-bridge
```

### Configuration

Add to `.vscode/settings/mcp.json` in your workspace:

```json
{
  "servers": {
    "architecture-service": {
      "command": "nc",
      "args": ["localhost", "8080"],
      "disabled": false,
      "autoApprove": ["resources/list", "resources/read"]
    }
  }
}
```

### Run

Start the MCP bridge server:
```bash
make run-bridge
```

The server will:
1. Listen on TCP port 8080
2. Monitor `docs/` directories for changes
3. Create a dedicated MCP server process for each client connection
4. Provide real-time access to your architectural documentation

### Test

Verify on the client IDE that the agent is connected and appears as running (either by checking the server logs or the client itself). Write a prompt and attempt to fetch one of the available resources.

## Adding Documentation

Place your markdown files in these directories:

```
docs/
├── guidelines/     # Architectural guidelines
├── patterns/       # Design patterns
└── adr/            # Architecture Decision Records
```

The server automatically detects and indexes new files.

## Development

```bash
make build        # Build stdio MCP server
make build-all    # Build all binaries
make test         # Run tests
make help         # Show all available commands
```
