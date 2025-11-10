---
inclusion: always
---

# Code Organization & Style

## Directory Structure

```
cmd/mcp-server/     # Entry point: main.go with signal handling
internal/models/    # document.go (docs), mcp.go (protocol messages)
internal/server/    # MCP message handling logic
pkg/cache/          # Thread-safe in-memory cache
pkg/monitor/        # File system watcher (fsnotify)
pkg/scanner/        # Documentation parser
pkg/errors/         # Error handling, circuit breaker, graceful degradation
pkg/logging/        # Structured logging
pkg/validation/     # Input validation
docs/               # Content: guidelines/, patterns/, adr/
```

## Package Rules

**Dependency flow**: `cmd/` → `internal/` → `pkg/`
- `internal/` is private to this module
- `pkg/` contains reusable utilities
- Never import `internal/` from `pkg/`

**Where to add new code**:
- MCP handlers → `internal/server/server.go`
- Protocol types → `internal/models/mcp.go`
- Document types → `internal/models/document.go`
- Utilities → `pkg/` subdirectories
- Bootstrap → `cmd/mcp-server/main.go`

## Go Style

**Errors**:
- Return errors explicitly, never panic in production
- Use `pkg/errors` for structured errors
- Wrap with context: `fmt.Errorf("scan failed: %w", err)`
- Log internally, return generic messages to clients

**Concurrency**:
- Use `sync.RWMutex` for read-heavy caches
- File watcher runs in separate goroutine
- Use channels for shutdown signals
- Always protect shared state

**Naming**:
- Interfaces: `Scanner`, `Cacheable` (noun/adjective)
- Constructors: `NewServer`, `NewCache` (New prefix)
- Getters: `Resource()` not `GetResource()` (no Get prefix)
- Private: lowercase first letter

**Testing**:
- Place `*_test.go` alongside source
- Use table-driven tests with `t.Run()`
- Mock external dependencies (filesystem, time)

## Architecture

**Separation of concerns**:
- Models = data structures only (no logic)
- Server = protocol orchestration
- Pkg = single-purpose utilities

**Interfaces**:
- Define in consumer packages, not implementers
- Keep small (1-3 methods)
- Use for testability

**Resources**:
- Initialize in `main.go`
- Pass dependencies explicitly (no globals)
- Graceful shutdown with signal handling
- Clean up with defer

## Security

**Path validation** (critical):
- Always validate URIs before filesystem access
- Use `filepath.Clean()` to normalize
- Verify paths stay within `docs/` directory
- Reject `..` traversal attempts

**Container constraints**:
- Non-root user (UID 1001)
- Read-only root filesystem
- No network listeners (stdio only)