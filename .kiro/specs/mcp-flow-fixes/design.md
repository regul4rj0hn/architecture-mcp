# Design Document

## Overview

This design addresses three interconnected issues in the MCP Architecture Service:

1. **Completion Endpoint**: Implements the `completion/complete` MCP method to provide autocomplete suggestions for prompt arguments
2. **Graceful Shutdown**: Fixes TCP connection error logging during mcp-bridge shutdown
3. **Logging Configuration**: Adds command-line flag for configurable log levels

The design follows the existing architecture patterns and integrates seamlessly with the current codebase structure.

## Architecture

### Component Interaction

```
┌─────────────┐
│ MCP Client  │
└──────┬──────┘
       │ TCP (localhost:8080)
       ▼
┌─────────────────┐
│  MCP Bridge     │  ← Handles TCP connections
│  (cmd/mcp-      │  ← Spawns MCP Server per session
│   bridge)       │  ← Needs graceful shutdown fix
└──────┬──────────┘
       │ stdio
       ▼
┌─────────────────┐
│  MCP Server     │  ← Handles JSON-RPC methods
│  (cmd/mcp-      │  ← Needs completion endpoint
│   server)       │  ← Needs log level flag
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  Internal       │
│  Components     │
│  - Cache        │
│  - Prompts      │
│  - Tools        │
└─────────────────┘
```

### Data Flow for Completion Requests

```
Client → Bridge → Server → handleMessage()
                              ↓
                         handleCompletionComplete()
                              ↓
                    ┌─────────┴─────────┐
                    ▼                   ▼
            Validate Request    Get Prompt Definition
                    ↓                   ↓
            Extract Argument    Determine Completion Type
                    ↓                   ↓
            Query Cache         Generate Completions
                    ↓                   ↓
            Filter by Prefix    Format Response
                    └─────────┬─────────┘
                              ▼
                         Return Result
```

## Components and Interfaces

### 1. Completion Endpoint (MCP Server)

#### New Models (internal/models/mcp.go)

```go
// MCPCompletionCompleteParams represents parameters for completion/complete
type MCPCompletionCompleteParams struct {
    Ref      MCPCompletionRef           `json:"ref"`
    Argument MCPCompletionArgument      `json:"argument"`
    Context  map[string]interface{}     `json:"context,omitempty"`
}

// MCPCompletionRef represents a reference to a prompt
type MCPCompletionRef struct {
    Type string `json:"type"` // Must be "ref/prompt"
    Name string `json:"name"` // Prompt name
}

// MCPCompletionArgument represents the argument being completed
type MCPCompletionArgument struct {
    Name  string `json:"name"`  // Argument name
    Value string `json:"value"` // Current partial value
}

// MCPCompletionItem represents a single completion suggestion
type MCPCompletionItem struct {
    Value       string `json:"value"`
    Label       string `json:"label,omitempty"`
    Description string `json:"description,omitempty"`
}

// MCPCompletionResult represents the result of completion/complete
type MCPCompletionResult struct {
    Completion MCPCompletion `json:"completion"`
}

// MCPCompletion contains the completion values
type MCPCompletion struct {
    Values []MCPCompletionItem `json:"values"`
    Total  int                 `json:"total,omitempty"`
    HasMore bool               `json:"hasMore,omitempty"`
}
```

#### Handler Implementation (internal/server/handlers.go)

Add new handler method:

```go
// handleCompletionComplete handles the completion/complete method
func (s *MCPServer) handleCompletionComplete(message *models.MCPMessage) *models.MCPMessage {
    s.mu.RLock()
    defer s.mu.RUnlock()

    // Parse request parameters
    var params models.MCPCompletionCompleteParams
    if message.Params != nil {
        paramsBytes, err := json.Marshal(message.Params)
        if err != nil {
            return s.createErrorResponse(message.ID, -32602, "Invalid parameters")
        }
        if err := json.Unmarshal(paramsBytes, &params); err != nil {
            return s.createErrorResponse(message.ID, -32602, "Invalid parameters format")
        }
    }

    // Validate parameters
    if err := s.validateCompletionParams(&params); err != nil {
        return s.createStructuredErrorResponse(message.ID, err)
    }

    // Generate completions with circuit breaker protection
    circuitBreaker := s.circuitBreakerManager.GetOrCreate("completion",
        errors.DefaultCircuitBreakerConfig("completion"))

    var completions []models.MCPCompletionItem
    err := circuitBreaker.Execute(func() error {
        var genErr error
        completions, genErr = s.generateCompletions(&params)
        return genErr
    })

    if err != nil {
        return s.handleCompletionError(message.ID, params.Ref.Name, params.Argument.Name, err)
    }

    result := models.MCPCompletionResult{
        Completion: models.MCPCompletion{
            Values:  completions,
            Total:   len(completions),
            HasMore: false,
        },
    }

    return &models.MCPMessage{
        JSONRPC: "2.0",
        ID:      message.ID,
        Result:  result,
    }
}
```

#### Completion Generation Logic

```go
// generateCompletions generates completion suggestions based on argument type
func (s *MCPServer) generateCompletions(params *models.MCPCompletionCompleteParams) ([]models.MCPCompletionItem, error) {
    // Determine completion type based on argument name
    switch params.Argument.Name {
    case "pattern_name":
        return s.generatePatternCompletions(params.Argument.Value)
    case "guideline_name":
        return s.generateGuidelineCompletions(params.Argument.Value)
    case "adr_id":
        return s.generateADRCompletions(params.Argument.Value)
    default:
        // No completions for unknown argument types
        return []models.MCPCompletionItem{}, nil
    }
}

// generatePatternCompletions generates completions for pattern names
func (s *MCPServer) generatePatternCompletions(prefix string) ([]models.MCPCompletionItem, error) {
    allDocuments := s.cache.GetAllDocuments()
    var completions []models.MCPCompletionItem

    lowerPrefix := strings.ToLower(prefix)

    for _, doc := range allDocuments {
        if doc.Category != "patterns" {
            continue
        }

        // Extract pattern name from path (e.g., "repository-pattern.md" → "repository pattern")
        patternName := strings.TrimSuffix(doc.Path, ".md")
        patternName = strings.ReplaceAll(patternName, "-", " ")

        // Filter by prefix (case-insensitive)
        if prefix == "" || strings.HasPrefix(strings.ToLower(patternName), lowerPrefix) {
            completions = append(completions, models.MCPCompletionItem{
                Value:       patternName,
                Label:       patternName,
                Description: doc.Content.Title,
            })
        }
    }

    return completions, nil
}
```

#### Server Message Routing Update (internal/server/server.go)

Add case to handleMessage switch:

```go
case "completion/complete":
    response = s.handleCompletionComplete(message)
```

#### Capability Advertisement

Update MCPCapabilities in internal/models/mcp.go:

```go
type MCPCapabilities struct {
    Resources  *MCPResourceCapabilities  `json:"resources,omitempty"`
    Prompts    *MCPPromptCapabilities    `json:"prompts,omitempty"`
    Tools      *MCPToolCapabilities      `json:"tools,omitempty"`
    Completion *MCPCompletionCapabilities `json:"completion,omitempty"`
}

type MCPCompletionCapabilities struct {
    ArgumentCompletions bool `json:"argumentCompletions"`
}
```

Update server initialization in internal/server/server.go:

```go
capabilities: models.MCPCapabilities{
    // ... existing capabilities ...
    Completion: &models.MCPCompletionCapabilities{
        ArgumentCompletions: true,
    },
}
```

### 2. Graceful Shutdown (MCP Bridge)

#### Problem Analysis

Current code in cmd/mcp-bridge/main.go:

```go
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        conn, err := b.listener.Accept()
        if err != nil {
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
                log.Printf("Accept error: %v", err)  // ← Logs error even during shutdown
                continue
            }
        }
        // ...
    }
}
```

The issue: When `Shutdown()` closes the listener, the `Accept()` call returns an error. The code checks `ctx.Done()` but the context might not be cancelled yet, causing the error to be logged.

#### Solution Design

Use a dedicated shutdown flag and check it before logging:

```go
type MCPBridge struct {
    // ... existing fields ...
    shutdownFlag atomic.Bool  // Add atomic flag for shutdown state
}

func (b *MCPBridge) Start(ctx context.Context) error {
    // ... existing setup ...

    for {
        select {
        case <-ctx.Done():
            return nil  // Clean exit
        default:
            conn, err := b.listener.Accept()
            if err != nil {
                // Check if we're shutting down
                if b.shutdownFlag.Load() {
                    return nil  // Clean exit during shutdown
                }
                
                // Check context again
                select {
                case <-ctx.Done():
                    return nil
                default:
                    log.Printf("Accept error: %v", err)
                    continue
                }
            }

            go b.handleConnection(conn)
        }
    }
}

func (b *MCPBridge) Shutdown() error {
    // Set shutdown flag BEFORE closing listener
    b.shutdownFlag.Store(true)
    
    if b.listener != nil {
        b.listener.Close()
    }

    // ... rest of shutdown logic ...
}
```

### 3. Logging Configuration (MCP Server)

#### Command-Line Flag

Add flag parsing in cmd/mcp-server/main.go:

```go
import (
    "flag"
    // ... other imports ...
)

func main() {
    // Parse command-line flags
    logLevel := flag.String("log-level", "INFO", "Logging level (DEBUG, INFO, WARN, ERROR)")
    flag.Parse()

    // Validate log level
    validLevels := map[string]bool{
        "DEBUG": true,
        "INFO":  true,
        "WARN":  true,
        "ERROR": true,
    }

    upperLogLevel := strings.ToUpper(*logLevel)
    if !validLevels[upperLogLevel] {
        log.Fatalf("Invalid log level: %s. Must be one of: DEBUG, INFO, WARN, ERROR", *logLevel)
    }

    // Create server with log level
    server := server.NewMCPServerWithLogLevel(upperLogLevel)
    
    // ... rest of main ...
}
```

#### Logging Manager Enhancement

Update pkg/logging/manager.go:

```go
type LoggingManager struct {
    // ... existing fields ...
    logLevel LogLevel
}

type LogLevel int

const (
    LogLevelDEBUG LogLevel = iota
    LogLevelINFO
    LogLevelWARN
    LogLevelERROR
)

func (lm *LoggingManager) SetLogLevel(level string) {
    switch strings.ToUpper(level) {
    case "DEBUG":
        lm.logLevel = LogLevelDEBUG
    case "INFO":
        lm.logLevel = LogLevelINFO
    case "WARN":
        lm.logLevel = LogLevelWARN
    case "ERROR":
        lm.logLevel = LogLevelERROR
    default:
        lm.logLevel = LogLevelINFO
    }
}

func (lm *LoggingManager) shouldLog(level LogLevel) bool {
    return level >= lm.logLevel
}
```

Update pkg/logging/logger.go to check log level:

```go
func (sl *StructuredLogger) Debug(message string) {
    if !sl.manager.shouldLog(LogLevelDEBUG) {
        return
    }
    // ... existing debug logging ...
}

func (sl *StructuredLogger) Info(message string) {
    if !sl.manager.shouldLog(LogLevelINFO) {
        return
    }
    // ... existing info logging ...
}
```

#### Server Constructor Update

Add new constructor in internal/server/server.go:

```go
// NewMCPServerWithLogLevel creates a new MCP server with specified log level
func NewMCPServerWithLogLevel(logLevel string) *MCPServer {
    server := NewMCPServer()
    server.loggingManager.SetLogLevel(logLevel)
    return server
}
```

## Data Models

### Completion Request Example

```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "completion/complete",
  "params": {
    "ref": {
      "type": "ref/prompt",
      "name": "guided-pattern-validation"
    },
    "argument": {
      "name": "pattern_name",
      "value": "rep"
    },
    "context": {
      "arguments": {
        "code": "Microservice pattern"
      }
    }
  }
}
```

### Completion Response Example

```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "result": {
    "completion": {
      "values": [
        {
          "value": "repository pattern",
          "label": "repository pattern",
          "description": "Repository Pattern for Data Access"
        }
      ],
      "total": 1,
      "hasMore": false
    }
  }
}
```

## Error Handling

### Completion Errors

1. **Invalid Reference Type**: Return -32602 if `ref.type` is not "ref/prompt"
2. **Prompt Not Found**: Return -32602 if prompt doesn't exist
3. **Invalid Argument**: Return -32602 if argument doesn't exist for prompt
4. **Cache Error**: Return -32603 for internal cache access failures
5. **Circuit Breaker Open**: Return -32603 when circuit breaker prevents execution

### Bridge Shutdown Errors

- Suppress "Accept error" when shutdown flag is set
- Log only unexpected errors during normal operation
- Ensure clean context cancellation propagation

### Logging Configuration Errors

- Validate log level at startup
- Exit with error message for invalid log levels
- Default to INFO if validation fails in library code

## Testing Strategy

### Unit Tests

#### Completion Endpoint Tests (internal/server/server_test.go)

```go
func TestHandleCompletionComplete_ValidPatternName(t *testing.T)
func TestHandleCompletionComplete_InvalidRefType(t *testing.T)
func TestHandleCompletionComplete_PromptNotFound(t *testing.T)
func TestHandleCompletionComplete_PrefixFiltering(t *testing.T)
func TestHandleCompletionComplete_EmptyPrefix(t *testing.T)
func TestHandleCompletionComplete_NoCompletionsForUnknownArgument(t *testing.T)
```

#### Bridge Shutdown Tests (cmd/mcp-bridge/main_test.go)

```go
func TestBridge_GracefulShutdown(t *testing.T)
func TestBridge_NoErrorLogDuringShutdown(t *testing.T)
func TestBridge_ShutdownWithActiveConnections(t *testing.T)
```

#### Logging Level Tests (pkg/logging/manager_test.go)

```go
func TestLoggingManager_SetLogLevel(t *testing.T)
func TestLoggingManager_ShouldLog(t *testing.T)
func TestStructuredLogger_RespectsLogLevel(t *testing.T)
```

### Integration Tests

#### End-to-End Completion Flow

Test completion request through bridge to server:
1. Start bridge and server
2. Connect TCP client
3. Send completion request
4. Verify response format and content
5. Test with various argument types

#### Shutdown Behavior

Test clean shutdown without error logs:
1. Start bridge with active connections
2. Send SIGTERM
3. Verify no "Accept error" in logs
4. Verify all sessions cleaned up

#### Log Level Configuration

Test different log levels:
1. Start server with DEBUG level
2. Verify detailed logging
3. Restart with ERROR level
4. Verify minimal logging

## Performance Considerations

### Completion Generation

- **Cache Access**: O(n) where n = number of documents in category
- **Prefix Filtering**: O(m) where m = matching documents
- **Memory**: Minimal - reuses existing cache data
- **Optimization**: Consider caching completion lists if performance becomes an issue

### Shutdown Performance

- **Target**: Complete shutdown within 5 seconds
- **Mechanism**: Atomic flag check is O(1)
- **Session Cleanup**: Parallel goroutine termination

### Logging Overhead

- **Level Check**: O(1) atomic operation
- **DEBUG Mode**: Higher overhead due to detailed logging
- **Production**: Use INFO or WARN to minimize overhead

## Security Considerations

### Completion Endpoint

- **Input Validation**: Validate all parameters before processing
- **Resource Access**: Only expose document metadata, not file paths
- **Rate Limiting**: Circuit breaker prevents abuse
- **Information Disclosure**: Completions only reveal document titles and names

### Bridge Shutdown

- **No Security Impact**: Shutdown fix is purely operational
- **Session Cleanup**: Ensure all processes terminated properly

### Logging Configuration

- **Sensitive Data**: Avoid logging sensitive information even in DEBUG mode
- **Log Injection**: Sanitize all logged values
- **File Permissions**: Ensure log files have appropriate permissions

## Migration and Deployment

### Backward Compatibility

- **Completion Endpoint**: Optional feature, existing clients unaffected
- **Bridge Shutdown**: Internal change, no API impact
- **Logging Flag**: Optional flag, defaults to current behavior (INFO)

### Deployment Steps

1. Update steering rules (already done)
2. Implement completion endpoint in MCP Server
3. Update bridge shutdown logic
4. Add logging configuration
5. Run full test suite
6. Build and deploy new binaries
7. Update documentation

### Rollback Plan

- All changes are additive or internal
- Can rollback to previous version without data loss
- No database or persistent state changes

## Open Questions and Future Enhancements

### Completion Enhancements

- **Context-Aware Completions**: Use `context.arguments` to provide smarter suggestions
- **Fuzzy Matching**: Support fuzzy search instead of just prefix matching
- **Completion Caching**: Cache completion lists for frequently accessed prompts
- **Custom Completions**: Allow prompts to define custom completion logic

### Logging Enhancements

- **Structured Log Output**: Support JSON log format for log aggregation
- **Log Rotation**: Implement log file rotation for long-running processes
- **Dynamic Log Level**: Support changing log level without restart via signal

### Bridge Enhancements

- **Connection Pooling**: Reuse MCP Server processes for multiple connections
- **Health Checks**: Add health check endpoint for monitoring
- **Metrics**: Expose Prometheus metrics for observability
