---
inclusion: always
---

# Product & Protocol Conventions

## Service Purpose

This project consists of two components:

1. **MCP Server** (`cmd/mcp-server`): Core MCP service exposing architectural documentation (guidelines, patterns, ADRs) as resources and interactive prompts for guided workflows. Communicates via JSON-RPC 2.0 over stdio only—no HTTP, no network.

2. **MCP Bridge** (`cmd/mcp-bridge`): TCP-to-stdio bridge service that allows network clients to connect to the MCP Server. Listens on `localhost:8080` and spawns an MCP Server process for each client connection.

**MCP primitives**:
- **Resources**: Architectural documentation from `mcp/resources/`
- **Prompts**: Interactive templates from `mcp/prompts/` (code review, pattern suggestions, ADR creation)
- **Tools**: Executable functions for architecture validation and search
- **Completions**: Autocomplete suggestions for prompt arguments

## Resource URIs

Use `architecture://` scheme:
- `architecture://guidelines/{filename}` → `mcp/resources/guidelines/`
- `architecture://patterns/{filename}` → `mcp/resources/patterns/`
- `architecture://adr/{adr-id}` → `mcp/resources/adr/` (e.g., `001-microservices-architecture`)

**Always validate paths to prevent traversal attacks.**

## MCP Protocol

**Implemented methods**:
- `initialize` - Server initialization and capability negotiation
- `notifications/initialized` - Client initialization complete notification
- `resources/list` - List available architectural documentation
- `resources/read` - Read specific resource content
- `prompts/list` - List available interactive prompts
- `prompts/get` - Get rendered prompt with arguments
- `tools/list` - List available executable tools
- `tools/call` - Execute a tool with arguments
- `completion/complete` - Get autocomplete suggestions for prompt arguments

**JSON-RPC error codes**:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error

**Behavior**:
- Resources discovered by scanning `mcp/resources/` tree
- Prompts loaded from `mcp/prompts/*.json`
- Tools registered from `pkg/tools/` implementations
- Return content as plain text in MCP format
- MCP Server: Communication via stdin/stdout only
- MCP Bridge: TCP connections on localhost:8080

## Documentation Structure

**mcp/resources/guidelines/** - High-level architectural principles
**mcp/resources/patterns/** - Reusable design patterns with examples
**mcp/resources/adr/** - Decision records (title, status, context, decision, consequences)

**Rules**:
- All files are markdown (`.md`)
- ADR filenames: `NNN-kebab-case-title.md` (e.g., `001-microservices-architecture.md`)

## Prompts

**Built-in prompts**:
- `review-code-against-patterns` - Code review (args: code, language)
- `suggest-patterns` - Pattern suggestions (args: problem description)
- `create-adr` - ADR creation (args: decision topic)
- `guided-pattern-validation` - Interactive pattern validation workflow

**Prompt format** (JSON in `mcp/prompts/`):
- Template variables: `{{argumentName}}`
- Resource embedding: `{{resource:architecture://patterns/*}}`
- Hot-reload on file changes

**Validation rules**:
- Prompt names: `^[a-z0-9-]+$`
- Argument limits: 10,000 chars (code), 2,000 chars (descriptions)
- Resource limits: max 50 resources, 1MB total per prompt
- Return `-32602` for missing/invalid arguments

**Completions**:
- Reference type: `ref/prompt` with prompt name
- Supports argument-specific completions (pattern_name, guideline_name, adr_id)
- Prefix-based filtering with case-insensitive matching
- Returns completion items with value, optional label and description

## Caching

- Documents cached in-memory on startup
- Prompts loaded into registry on startup
- `fsnotify` watches `mcp/resources/` and `mcp/prompts/` for changes
- Auto-invalidate on create/modify/delete (within 2 seconds)
- Thread-safe with `sync.RWMutex`

## Error Handling

- Use `pkg/errors` for structured errors with context
- Log errors internally, return generic messages to clients
- Invalid URIs → "resource not found" (not "invalid path")
- Never expose internal paths or stack traces

## Security

**Critical rules**:
- Never expose paths outside `mcp/resources/` directory
- Validate all URIs before filesystem access
- Run as non-root (UID 1001)
- MCP Server: No network listeners (stdio only)
- MCP Bridge: Network listener on localhost:8080 only
- Read-only root filesystem in containers

## Logging

**Log levels**: DEBUG, INFO, WARN, ERROR
- Use `--log-level` flag to set verbosity (default: INFO)
- DEBUG: Full request/response logging with details
- INFO: Standard operational messages
- WARN: Degraded functionality warnings
- ERROR: Error conditions and failures only
