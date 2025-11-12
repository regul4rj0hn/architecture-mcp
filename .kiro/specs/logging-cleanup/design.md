# Design Document

## Overview

This design document outlines the approach for cleaning up logging practices across the MCP Architecture Service codebase. The cleanup will replace all standard library `log` package usage with the structured logging system from `pkg/logging`, ensure appropriate log levels are used, and add proper context to all log messages.

## Architecture

### Current State Analysis

Based on code review, the following files contain standard library `log` package usage:

1. **cmd/mcp-server/main.go**: Uses `log.Printf` and `log.Println` for startup, shutdown, and error messages
2. **cmd/mcp-bridge/main.go**: Uses `log.Printf` and `log.Println` extensively for session management, connection handling, and error logging
3. **internal/server/server.go**: Uses `log.Printf` in the `processMessages` method for JSON-RPC message decoding errors

The structured logging system (`pkg/logging`) is already well-designed and in use throughout most of the codebase. The cleanup task is to extend its usage to the remaining files.

### Design Principles

1. **Consistency**: All logging must go through `pkg/logging.LoggingManager`
2. **Context**: Every log message should include relevant context fields
3. **Appropriate Levels**: Use DEBUG for diagnostics, INFO for normal operations, WARN for degraded states, ERROR for failures
4. **Minimal Changes**: Preserve existing functionality while improving logging quality
5. **No Breaking Changes**: Maintain the same command-line interface and behavior

## Components and Interfaces

### LoggingManager Integration

Each entry point (MCP Server and MCP Bridge) will:

1. Create a `LoggingManager` instance early in `main()`
2. Set global context (service name, version)
3. Configure log level from command-line flags
4. Obtain component-specific loggers as needed
5. Pass the logging manager to components that need it

### Component-Specific Loggers

The following component loggers will be used:

- **MCP Server (`cmd/mcp-server/main.go`)**:
  - `main`: For startup, shutdown, and signal handling
  
- **MCP Bridge (`cmd/mcp-bridge/main.go`)**:
  - `bridge`: For bridge server lifecycle
  - `session`: For individual session management
  
- **Server Core (`internal/server/server.go`)**:
  - Already has `server` logger, just needs to use it consistently

## Data Models

### Log Context Fields

Standard context fields to include:

- **Startup/Shutdown**: `phase`, `duration_ms`, `success`
- **Errors**: `error`, `operation`, relevant identifiers
- **Bridge Sessions**: `session_id`, `remote_addr`, `server_path`
- **JSON-RPC Messages**: `method`, `request_id` (already handled by existing code)
- **File Operations**: `file_path`, `operation`

## Error Handling

### Error Logging Strategy

1. **Startup Errors**: Log at ERROR level with context, then exit with non-zero status
2. **Runtime Errors**: Log at ERROR level with context, attempt recovery if possible
3. **Shutdown Errors**: Log at ERROR level with context, continue shutdown sequence
4. **Degraded Operations**: Log at WARN level with context, continue with reduced functionality

### Error Context

All error logs must include:
- The error object via `.WithError(err)`
- Operation being performed
- Relevant identifiers (session ID, file path, etc.)
- Any additional context that aids debugging

## Testing Strategy

### Verification Approach

1. **Code Review**: Manually verify all `log` package imports are removed
2. **Search Verification**: Use `grep` or similar to confirm no `log.Print*` calls remain
3. **Manual Testing**: Run both MCP Server and MCP Bridge to verify:
   - Logs are output as JSON
   - Log levels are respected
   - Context fields are present
   - No panics or missing logger errors occur
4. **Integration Testing**: Verify existing integration tests still pass

### Test Scenarios

1. **Normal Startup**: Verify INFO-level logs for initialization
2. **Graceful Shutdown**: Verify INFO-level logs for shutdown sequence
3. **Error Conditions**: Verify ERROR-level logs with proper context
4. **Bridge Sessions**: Verify session lifecycle logs include session_id
5. **Log Level Filtering**: Verify DEBUG logs are hidden when log level is INFO

## Implementation Notes

### MCP Server Changes

The `cmd/mcp-server/main.go` file needs:
- Remove `log` import
- Add `logging` import
- Create LoggingManager early in main()
- Replace all `log.Printf`/`log.Println` with structured logger calls
- Add appropriate context to each log message
- Use correct log levels (INFO for normal, ERROR for failures)

### MCP Bridge Changes

The `cmd/mcp-bridge/main.go` file needs:
- Remove `log` import
- Add `logging` import
- Create LoggingManager early in main()
- Pass logger to MCPBridge struct
- Pass logger to MCPSession struct
- Replace all `log.Printf`/`log.Println` with structured logger calls
- Add session_id and remote_addr context to session logs
- Use correct log levels throughout

### Server Core Changes

The `internal/server/server.go` file needs:
- Replace `log.Printf` calls in `processMessages` with structured logger
- Use existing `s.logger` instance
- Add appropriate context (method, error details)

## Migration Path

### Phase 1: MCP Server Entry Point
1. Update `cmd/mcp-server/main.go`
2. Test startup, shutdown, and error scenarios
3. Verify log output format and content

### Phase 2: MCP Bridge Entry Point
1. Update `cmd/mcp-bridge/main.go`
2. Add logger fields to MCPBridge and MCPSession structs
3. Test bridge lifecycle and session management
4. Verify log output format and content

### Phase 3: Server Core
1. Update `internal/server/server.go`
2. Test JSON-RPC message processing
3. Verify error logging

### Phase 4: Verification
1. Run full test suite
2. Perform manual testing of both components
3. Verify no `log` package usage remains
4. Confirm all logs are properly structured

## Security Considerations

The existing `pkg/logging` package already handles:
- Sensitive data redaction (passwords, tokens, secrets)
- File path sanitization
- Long alphanumeric string masking

No additional security measures are needed for this cleanup, as we're simply extending the use of the existing secure logging system.

## Performance Considerations

The structured logging system is already optimized with:
- Log level filtering to avoid unnecessary work
- Efficient JSON serialization
- Minimal allocations for context building

The cleanup will not introduce performance regressions, as we're replacing simple `log.Printf` calls with equivalent structured logging calls.
