---
inclusion: always
---

# Logging Standards

## Logging Package

**Always use `pkg/logging` for all logging needs**:
- Never use `log`, `fmt.Print*`, or other standard library logging directly
- Never write custom logging implementations
- All log output must go through the structured logging system

## Structured Logging

**Use the structured logger from `pkg/logging`**:
```go
import "mcp-architecture-service/pkg/logging"

// Get a logger for your component
logger := loggingManager.GetLogger("component-name")

// Log with context
logger.WithContext("key", "value").Info("Message")

// Log errors with full context
logger.WithError(err).Error("Operation failed")
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
- Add context to logs: `.WithContext("file", filename)`
- Use appropriate log levels based on severity
- Log errors with full context: `.WithError(err)`
- Use specialized logging methods when available (LogMCPMessage, LogCacheOperation, etc.)

**DON'T**:
- Don't use `fmt.Printf` or `log.Printf` for logging
- Don't log sensitive information (passwords, tokens, keys)
- Don't log at DEBUG level in production-critical paths
- Don't create custom loggers outside of `pkg/logging`
- Don't write logs to files directly

## Security

**Sensitive data handling**:
- Logging package automatically redacts sensitive keys (password, token, secret, etc.)
- File paths are sanitized to show only relative paths within `mcp/resources/`
- Long alphanumeric strings are masked automatically
- Never log authentication credentials or API keys

## Examples

```go
// Basic logging
logger := loggingManager.GetLogger("server")
logger.Info("Server started")

// With context
logger.WithContext("port", 8080).
    WithContext("protocol", "stdio").
    Info("Listening for connections")

// Error logging
if err != nil {
    logger.WithError(err).
        WithContext("operation", "cache_refresh").
        Error("Failed to refresh cache")
}

// MCP protocol logging
logger.LogMCPMessage("resources/list", requestID, duration, true)

// Performance metrics
logger.LogPerformanceMetric("cache_hit_ratio", 0.95, "percentage")
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
loggingManager.SetGlobalContext("version", version)

// Get component loggers
serverLogger := loggingManager.GetLogger("server")
cacheLogger := loggingManager.GetLogger("cache")
```

## Testing

**In tests, use the same logging package**:
- Tests can create their own LoggingManager instances
- Capture output by providing custom slog.Handler
- Test log level filtering and context propagation
- Verify sensitive data redaction
