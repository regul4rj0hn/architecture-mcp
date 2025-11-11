# MCP Architecture Service

[![CI](https://github.com/regul4rj0hn/architecture-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/regul4rj0hn/architecture-mcp/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/regul4rj0hn/architecture-mcp/branch/main/graph/badge.svg)](https://codecov.io/gh/regul4rj0hn/architecture-mcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/regul4rj0hn/architecture-mcp)](https://goreportcard.com/report/github.com/regul4rj0hn/architecture-mcp)

A Model Context Protocol (MCP) server that provides structured access to architectural guidelines, patterns, and Architecture Decision Records (ADRs) through JSON-RPC communication.

## Overview

This service implements the MCP specification to enable AI agents and IDE integrations to discover and retrieve architectural documentation as contextual resources. It monitors documentation directories for real-time updates and provides fast, cached access to your architecture knowledge base.

## Available MCP Resources

The service exposes documentation through the following URI patterns:

- `architecture://guidelines/{path}` - Architectural guidelines from `mcp/resources/guidelines/`
- `architecture://patterns/{path}` - Design patterns from `mcp/resources/patterns/`
- `architecture://adr/{adr_id}` - Architecture Decision Records from `mcp/resources/adr/`

## Available MCP Prompts

The service provides interactive prompts that combine instructions with architectural documentation:

- **review-code-against-patterns** - Review code against documented architectural patterns
  - Arguments: `code` (required, max 10,000 chars), `language` (optional)
  - Embeds relevant pattern documentation for comparison
  
- **suggest-patterns** - Suggest relevant patterns for a problem description
  - Arguments: `problem` (required, max 2,000 chars)
  - Embeds complete patterns catalog for analysis
  
- **create-adr** - Assist in creating a new Architecture Decision Record
  - Arguments: `topic` (required)
  - Embeds example ADRs and template structure

## Available MCP Tools

The service provides executable tools that enable AI agents to perform actions on architectural documentation:

- **validate-against-pattern** - Validates code against documented patterns for compliance
- **search-architecture** - Searches documentation by keywords across guidelines, patterns, and ADRs
- **check-adr-alignment** - Checks if proposed decisions align with existing ADRs

See [Tools Development Guide](docs/tools-guide.md) for detailed schemas, examples, and how to create custom tools.

## MCP Protocol Support

### Resources
- `initialize` - Server initialization and capability negotiation
- `notifications/initialized` - Initialization acknowledgment
- `resources/list` - List all available documentation resources
- `resources/read` - Read specific documentation resource content

### Prompts
- `prompts/list` - List all available interactive prompts
- `prompts/get` - Invoke a prompt with arguments to get rendered content

### Tools
- `tools/list` - List all available executable tools with schemas
- `tools/call` - Execute a tool with validated arguments

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
      "autoApprove": ["resources/list", "resources/read", "prompts/list", "prompts/get"]
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
2. Monitor `mcp/resources/` and `mcp/prompts/` directories for changes
3. Create a dedicated MCP server process for each client connection
4. Provide real-time access to your architectural documentation

### Test

Verify on the client IDE that the agent is connected and appears as running (either by checking the server logs or the client itself). Write a prompt and attempt to fetch one of the available resources.

## Adding Documentation

Place your markdown files in these directories:

```
mcp/
├── resources/
│   ├── guidelines/     # Architectural guidelines
│   ├── patterns/       # Design patterns
│   └── adr/            # Architecture Decision Records
└── prompts/            # Prompt definitions (JSON)
```

The server automatically detects and indexes new files.

## Usage

AI agents can interact with the service through standard MCP methods. See the [Architecture Overview](docs/architecture.md) for detailed protocol flows and integration patterns.

### Custom Prompts and Tools

- Add custom prompts as JSON files in `mcp/prompts/` - see [Prompts Guide](docs/prompts-guide.md)
- Create custom tools by implementing the Tool interface - see [Tools Development Guide](docs/tools-guide.md)
- Prompts can reference tools using `{{tool:tool-name}}` syntax for guided workflows



## Development

```bash
make build        # Build stdio MCP server
make build-all    # Build all binaries
make test         # Run tests
make help         # Show all available commands
```

## Documentation

- [Architecture Overview](docs/architecture.md) - System architecture diagrams including tools subsystem
- [Tools Development Guide](docs/tools-guide.md) - Complete guide for creating custom tools
- [Prompts Guide](docs/prompts-guide.md) - Prompt definition format and template syntax
- [ADR Template](docs/adr-template.md) - Template for Architecture Decision Records
