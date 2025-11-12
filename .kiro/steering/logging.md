---
inclusion: always
---

# Logging Standards

## Logging Package

**Always use `pkg/logging` for all logging needs**:
- Never use `log`, `fmt.Print*`, or other standard library logging directly
- Never write custom logging implementations
- All log output must go through the structured logging system
- `fmt.Errorf` is OK - it creates errors, not logs

## Structured Logging

**Use the structured logger from `pkg/logging`**:
```go
import "mcp-architecture-service/pkg/logging"

// Get a logger for your component
logger := loggingManager.GetLogger("component-name")

// Log with context
logger.WithContext("key", "value").Info("Message")

// Chain multiple context values
logger.WithContext("user_id", 123).
    WithContext("action", "login").
    Info("User logged in")

// Log errors with full context
logger.WithError(err).
    WithContext("operation", "cache_refresh").
    Error("Operation failed")
```

## Log Levels

**Four log levels available**:
- `DEBUG` - Detailed diagnostic information for development
- `INFO` - General informational messages (default)
- `WARN` - Warning messages for degraded functionality
- `ERROR` - Error messages for failures

**Level filtering**:
- Set via `--log-level` command-line flag
- Handled automatically by the logging manager
- Invalid levels default to INFO

## Output Format

**All logs output to stdout as JSON**:
- Structured JSON format for machine parsing
- Includes timestamp, level, component, message, and context
- No file logging - use container orchestration for log collection
- No log rotation - handled by container runtime

## Best Practices

**DO**:
- Use component-specific loggers: `loggingManager.GetLogger("scanner")`
- Chain context with `.WithContext("key", value)`
- Use appropriate log levels based on severity
- Log errors with `.WithError(err)`
- Keep it simple - just use the core API

**DON'T**:
- Don't use `fmt.Printf` or `log.Printf` for logging
- Don't log sensitive information (passwords, tokens, keys)
- Don't log at DEBUG level in production-critical paths
- Don't create custom loggers outside of `pkg/logging`
- Don't write logs to files directly

## Security

**Sensitive data handling**:
- Logging package automatically redacts sensitive keys (password, token, secret, key, auth, credential)
- Long alphanumeric strings (>32 chars) are masked automatically
- Never log authentication credentials or API keys

## Core API

The logging package provides a minimal, focused API:

```go
// Manager methods
manager := logging.NewLoggingManager()
manager.SetLogLevel("INFO")                    // Set log level
manager.SetGlobalContext("service", "my-app")  // Add global context
logger := manager.GetLogger("component")       // Get component logger

// Logger methods
logger.WithContext(key, value)  // Add context (returns new logger)
logger.WithError(err)            // Add error context (returns new logger)
logger.Debug(message)            // Log at DEBUG level
logger.Info(message)             // Log at INFO level
logger.Warn(message)             // Log at WARN level
logger.Error(message)            // Log at ERROR level
```

## Common Patterns

**Startup/Shutdown logging**:
```go
startupLogger := loggingManager.GetLogger("startup")
startupLogger.WithContext("phase", "initialization").
    WithContext("duration_ms", duration.Milliseconds()).
    Info("Server start")
```

**MCP protocol logging**:
```go
mcpLogger := loggingManager.GetLogger("mcp_protocol").
    WithContext("mcp_method", method).
    WithContext("request_id", requestID).
    WithContext("duration_ms", duration.Milliseconds()).
    WithContext("success", true)

if success {
    mcpLogger.Info("MCP message processed successfully")
} else {
    mcpLogger.Warn("MCP message processing failed")
}
```

**Cache operations**:
```go
cacheLogger := loggingManager.GetLogger("cache").
    WithContext("cache_operation", "batch_refresh").
    WithContext("affected_files_count", len(files)).
    WithContext("duration_ms", duration.Milliseconds())

cacheLogger.Info("Cache refresh completed")
```

**Error logging with context**:
```go
if err != nil {
    logger.WithError(err).
        WithContext("operation", "file_read").
        WithContext("file_path", path).
        Error("Failed to read file")
}
```

## Integration

**Server initialization**:
```go
// Create logging manager
loggingManager := logging.NewLoggingManager()

// Set log level from command-line flag
loggingManager.SetLogLevel(logLevel)

// Set global context
loggingManager.SetGlobalContext("service", "mcp-server")
loggingManager.SetGlobalContext("version", "1.0.0")

// Get component loggers as needed
serverLogger := loggingManager.GetLogger("server")
cacheLogger := loggingManager.GetLogger("cache")
```

## Testing

**In tests, use the same logging package**:
- Tests can create their own LoggingManager instances
- Capture output by providing custom slog.Handler
- Test log level filtering and context propagation
- Verify sensitive data redaction

```go
func TestLogging(t *testing.T) {
    manager := logging.NewLoggingManager()
    logger := manager.GetLogger("test")
    
    logger.WithContext("test_id", 123).Info("Test message")
}
```
