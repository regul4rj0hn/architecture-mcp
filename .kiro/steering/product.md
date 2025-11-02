# Product Overview

## MCP Architecture Service

A Model Context Protocol (MCP) server that provides structured access to architectural guidelines, patterns, and Architecture Decision Records (ADRs) stored in a Git repository.

### Core Purpose

Enable AI agents and IDE integrations to discover and retrieve architectural documentation as contextual resources through JSON-RPC communication over stdio, following the MCP specification.

### Key Features

- **MCP Protocol Compliance**: Full implementation of MCP specification for resource discovery and retrieval
- **Documentation Categories**: Support for guidelines, design patterns, and ADRs
- **Real-time Updates**: Automatic detection and refresh of documentation changes via file system monitoring
- **In-memory Caching**: Fast resource retrieval with automatic cache invalidation
- **Container Ready**: Docker containerization with security-focused configuration

### Resource URI Patterns

- Guidelines: `architecture://guidelines/{path}`
- Patterns: `architecture://patterns/{path}`
- ADRs: `architecture://adr/{adr_id}`

### Target Users

- Software engineers using AI-assisted development tools
- IDE integrations requiring architectural context
- AI agents performing code review and architectural guidance