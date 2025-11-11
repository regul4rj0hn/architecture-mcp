---
inclusion: always
---

# Product & Protocol Conventions

## Service Purpose

MCP server exposing architectural documentation (guidelines, patterns, ADRs) as resources and interactive prompts for guided workflows. JSON-RPC 2.0 over stdio only—no HTTP, no network.

**MCP primitives**:
- **Resources**: Architectural documentation from `mcp/resources/`
- **Prompts**: Interactive templates from `mcp/prompts/` (code review, pattern suggestions, ADR creation)

## Resource URIs

Use `architecture://` scheme:
- `architecture://guidelines/{filename}` → `mcp/resources/guidelines/`
- `architecture://patterns/{filename}` → `mcp/resources/patterns/`
- `architecture://adr/{adr-id}` → `mcp/resources/adr/` (e.g., `001-microservices-architecture`)

**Always validate paths to prevent traversal attacks.**

## MCP Protocol

**Required methods**: `initialize`, `resources/list`, `resources/read`, `prompts/list`, `prompts/get`

**JSON-RPC error codes**:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error

**Behavior**:
- Resources discovered by scanning `mcp/resources/` tree
- Prompts loaded from `mcp/prompts/*.json`
- Return content as plain text in MCP format
- Communication via stdin/stdout only

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

**Prompt format** (JSON in `mcp/prompts/`):
- Template variables: `{{argumentName}}`
- Resource embedding: `{{resource:architecture://patterns/*}}`
- Hot-reload on file changes

**Validation rules**:
- Prompt names: `^[a-z0-9-]+$`
- Argument limits: 10,000 chars (code), 2,000 chars (descriptions)
- Resource limits: max 50 resources, 1MB total per prompt
- Return `-32602` for missing/invalid arguments

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
- No network listeners (stdio only)
- Read-only root filesystem in containers