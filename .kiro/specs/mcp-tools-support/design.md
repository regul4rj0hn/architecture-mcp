# Design Document: MCP Tools Support

## Overview

This design adds MCP tools support to the architecture MCP server, enabling AI models to perform executable actions beyond reading documentation. Tools complement the existing resources (read-only documentation) and prompts (guided workflows) by providing actionable operations like validation, search, and analysis.

The design follows the existing architecture patterns established for resources and prompts, using a registry-based approach with hot-reloading, structured error handling, and performance tracking.

## Architecture

### High-Level Design

```
┌───────────────────────────────────────────────────────────────┐
│                      MCP Protocol Layer                       │
│                  (JSON-RPC 2.0 over stdio)                    │
└──────────────────┬────────────────────────────────────────────┘
                   │
                   ├─── tools/list ──────────┐
                   ├─── tools/call ──────────┤
                   │                         │
┌──────────────────▼─────────────────────────▼────────────────┐
│                    internal/server/                           │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  handlers.go: handleToolsList(), handleToolsCall()      │  │
│  └─────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  server.go: MCPServer with toolManager field            │  │
│  └─────────────────────────────────────────────────────────┘  │
└──────────────────┬────────────────────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────────────────┐
│                    pkg/tools/                                 │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  manager.go: ToolManager (registry, lifecycle)          │  │
│  └─────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  definition.go: Tool interface, ToolDefinition          │  │
│  └─────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  executor.go: ToolExecutor (validation, execution)      │  │
│  └─────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  validate_pattern.go: ValidatePatternTool               │  │
│  │  search_architecture.go: SearchArchitectureTool         │  │
│  │  check_adr_alignment.go: CheckADRAlignmentTool          │  │
│  └─────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────┘
                   │
                   ├─── Uses cache.DocumentCache
                   ├─── Uses scanner.DocumentationScanner
                   └─── Uses prompts.PromptManager (for integration)
```

### Component Responsibilities

**pkg/tools/manager.go** - ToolManager
- Maintains registry of available tools
- Handles tool registration and discovery
- Provides tool listing and lookup
- Tracks performance metrics
- Coordinates with file system monitor for hot-reloading (future)

**pkg/tools/definition.go** - Tool Interface & Schema
- Defines Tool interface that all tools must implement
- Provides JSON schema generation for tool parameters
- Handles tool metadata (name, description, parameters)
- Validates tool definitions on registration

**pkg/tools/executor.go** - ToolExecutor
- Validates tool arguments against schema
- Executes tools with timeout protection
- Handles errors and converts to MCP format
- Enforces security constraints (path validation, size limits)
- Logs tool invocations for debugging

**pkg/tools/*.go** - Individual Tool Implementations
- Each tool implements the Tool interface
- Self-contained with dependencies injected via constructor
- Performs specific architectural operations
- Returns structured results

**internal/server/handlers.go** - Protocol Handlers
- `handleToolsList()`: Returns available tools with schemas
- `handleToolsCall()`: Validates and executes tool invocations
- Integrates with existing error handling patterns

**internal/models/tool.go** - MCP Protocol Types
- MCPTool: Tool definition for protocol
- MCPToolsListResult: Response for tools/list
- MCPToolsCallParams: Parameters for tools/call
- MCPToolsCallResult: Response for tools/call

## Components and Interfaces

### Tool Interface

```go
// Tool represents an executable function exposed via MCP
type Tool interface {
    // Name returns the unique identifier for the tool
    Name() string
    
    // Description returns a human-readable description
    Description() string
    
    // InputSchema returns JSON schema for tool parameters
    InputSchema() map[string]interface{}
    
    // Execute runs the tool with validated arguments
    // Returns result data or error
    Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error)
}
```

### ToolManager

```go
type ToolManager struct {
    registry map[string]Tool
    executor *ToolExecutor
    logger   *logging.StructuredLogger
    mu       sync.RWMutex
    
    // Performance metrics
    stats ToolStats
}

type ToolStats struct {
    TotalInvocations      int64
    FailedInvocations     int64
    InvocationsByName     map[string]int64
    TotalExecutionTimeMs  int64
    ExecutionTimeByName   map[string]int64
    mu                    sync.RWMutex
}

// Methods
func NewToolManager() *ToolManager
func (tm *ToolManager) RegisterTool(tool Tool) error
func (tm *ToolManager) GetTool(name string) (Tool, error)
func (tm *ToolManager) ListTools() []models.MCPTool
func (tm *ToolManager) ExecuteTool(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error)
func (tm *ToolManager) GetPerformanceMetrics() map[string]interface{}
```

### ToolExecutor

```go
type ToolExecutor struct {
    maxExecutionTime time.Duration
    logger           *logging.StructuredLogger
}

// Methods
func NewToolExecutor() *ToolExecutor
func (te *ToolExecutor) ValidateArguments(tool Tool, arguments map[string]interface{}) error
func (te *ToolExecutor) Execute(ctx context.Context, tool Tool, arguments map[string]interface{}) (interface{}, error)
func (te *ToolExecutor) sanitizeArguments(arguments map[string]interface{}) map[string]interface{}
```

### Initial Tool Implementations

#### 1. ValidatePatternTool

Validates code against documented architectural patterns.

```go
type ValidatePatternTool struct {
    cache  *cache.DocumentCache
    logger *logging.StructuredLogger
}

// Input Schema
{
    "type": "object",
    "properties": {
        "code": {
            "type": "string",
            "description": "Code to validate",
            "maxLength": 50000
        },
        "pattern_name": {
            "type": "string",
            "description": "Name of pattern to validate against (e.g., 'repository-pattern')"
        },
        "language": {
            "type": "string",
            "description": "Programming language (optional)"
        }
    },
    "required": ["code", "pattern_name"]
}

// Output
{
    "compliant": boolean,
    "pattern": string,
    "violations": [
        {
            "rule": string,
            "description": string,
            "severity": "error" | "warning"
        }
    ],
    "suggestions": [string]
}
```

**Implementation approach:**
- Load pattern document from cache
- Parse pattern for validation rules (heuristic-based)
- Check code structure against pattern expectations
- Return structured validation results

#### 2. SearchArchitectureTool

Searches architectural documentation by keywords.

```go
type SearchArchitectureTool struct {
    cache  *cache.DocumentCache
    logger *logging.StructuredLogger
}

// Input Schema
{
    "type": "object",
    "properties": {
        "query": {
            "type": "string",
            "description": "Search query",
            "maxLength": 500
        },
        "resource_type": {
            "type": "string",
            "enum": ["guidelines", "patterns", "adr", "all"],
            "description": "Filter by resource type (default: all)"
        },
        "max_results": {
            "type": "integer",
            "minimum": 1,
            "maximum": 20,
            "description": "Maximum results to return (default: 10)"
        }
    },
    "required": ["query"]
}

// Output
{
    "results": [
        {
            "uri": string,
            "title": string,
            "resource_type": string,
            "relevance_score": float,
            "excerpt": string
        }
    ],
    "total_matches": integer
}
```

**Implementation approach:**
- Tokenize query and document content
- Calculate relevance scores using TF-IDF or simple keyword matching
- Filter by resource type if specified
- Return top N results with excerpts

#### 3. CheckADRAlignmentTool

Checks if a decision aligns with existing ADRs.

```go
type CheckADRAlignmentTool struct {
    cache  *cache.DocumentCache
    logger *logging.StructuredLogger
}

// Input Schema
{
    "type": "object",
    "properties": {
        "decision_description": {
            "type": "string",
            "description": "Description of the proposed decision",
            "maxLength": 5000
        },
        "decision_context": {
            "type": "string",
            "description": "Context or problem being addressed (optional)",
            "maxLength": 2000
        }
    },
    "required": ["decision_description"]
}

// Output
{
    "related_adrs": [
        {
            "uri": string,
            "title": string,
            "adr_id": string,
            "status": string,
            "alignment": "supports" | "conflicts" | "related",
            "reason": string
        }
    ],
    "conflicts": [
        {
            "adr_uri": string,
            "conflict_description": string
        }
    ],
    "suggestions": [string]
}
```

**Implementation approach:**
- Extract keywords from decision description
- Search ADR documents for related content
- Analyze ADR status and decision text
- Identify potential conflicts or supporting decisions
- Return structured alignment analysis

## Data Models

### MCP Protocol Types (internal/models/tool.go)

```go
// MCPTool represents a tool definition in MCP protocol
type MCPTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolsListResult represents the result of tools/list
type MCPToolsListResult struct {
    Tools []MCPTool `json:"tools"`
}

// MCPToolsCallParams represents parameters for tools/call
type MCPToolsCallParams struct {
    Name      string                 `json:"name"`
    Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPToolsCallResult represents the result of tools/call
type MCPToolsCallResult struct {
    Content []MCPToolContent `json:"content"`
}

// MCPToolContent represents tool execution result content
type MCPToolContent struct {
    Type string `json:"type"` // "text" or "resource" or "image"
    Text string `json:"text,omitempty"`
}

// MCPToolCapabilities represents tool-related capabilities
type MCPToolCapabilities struct {
    ListChanged bool `json:"listChanged,omitempty"`
}
```

### Capabilities Update

Update `MCPCapabilities` in `internal/models/mcp.go`:

```go
type MCPCapabilities struct {
    Resources *MCPResourceCapabilities `json:"resources,omitempty"`
    Prompts   *MCPPromptCapabilities   `json:"prompts,omitempty"`
    Tools     *MCPToolCapabilities     `json:"tools,omitempty"`  // NEW
}
```

## Prompt-Tool Integration

### Tool References in Prompts

Prompts can reference tools using special syntax in templates:

```json
{
  "name": "guided-pattern-validation",
  "description": "Guided workflow for validating code against patterns",
  "arguments": [
    {
      "name": "code",
      "description": "Code to validate",
      "required": true
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "I'll help you validate this code against our patterns.\n\n{{tool:validate-against-pattern}}\n\nYou can use the validate-against-pattern tool with your code to check compliance."
      }
    }
  ]
}
```

### Tool Reference Expansion

The `TemplateRenderer` will expand `{{tool:tool-name}}` references:

```go
// In pkg/prompts/renderer.go
func (tr *TemplateRenderer) EmbedTools(text string) (string, error) {
    // Find all {{tool:name}} references
    // Look up tool from ToolManager
    // Replace with tool description and schema
    // Return expanded text
}
```

Expanded output:
```
Tool: validate-against-pattern
Description: Validates code against documented architectural patterns
Parameters:
  - code (required): Code to validate (max 50000 chars)
  - pattern_name (required): Name of pattern to validate against
  - language (optional): Programming language
```

### Workflow State Management

For multi-step workflows, the ToolExecutor can maintain context:

```go
type WorkflowContext struct {
    SessionID     string
    PromptName    string
    PromptArgs    map[string]interface{}
    ToolResults   map[string]interface{}
    CreatedAt     time.Time
}

// In ToolExecutor
func (te *ToolExecutor) ExecuteWithContext(ctx context.Context, tool Tool, arguments map[string]interface{}, workflowCtx *WorkflowContext) (interface{}, error)
```

This allows tools to access prompt arguments and previous tool results within a workflow session.

## Error Handling

### Error Types

Following existing patterns in `pkg/errors`:

```go
const (
    ErrCodeToolNotFound      = -32001
    ErrCodeToolExecutionFailed = -32002
    ErrCodeToolTimeout       = -32003
    ErrCodeInvalidToolSchema = -32004
)

// Tool-specific errors
type ToolError struct {
    *StructuredError
    ToolName string
}

func NewToolError(code int, message string, err error) *ToolError
func (te *ToolError) WithToolContext(toolName string, arguments map[string]interface{}) *ToolError
```

### Error Handling Flow

1. **Validation Errors** → Return `-32602` (Invalid params) with details
2. **Tool Not Found** → Return `-32001` (Tool not found)
3. **Execution Errors** → Return `-32002` (Tool execution failed) with sanitized message
4. **Timeout Errors** → Return `-32003` (Tool timeout)
5. **Internal Errors** → Return `-32603` (Internal error) with generic message

### Circuit Breaker Integration

Tools integrate with existing circuit breaker system:

```go
// In ToolExecutor.Execute()
circuitBreaker := s.circuitBreakerManager.GetOrCreate(
    fmt.Sprintf("tool_%s", toolName),
    errors.DefaultCircuitBreakerConfig(toolName))

err = circuitBreaker.Execute(func() error {
    result, execErr = tool.Execute(ctx, arguments)
    return execErr
})
```

## Testing Strategy

### Unit Tests

**pkg/tools/manager_test.go**
- Tool registration and lookup
- Duplicate tool name handling
- Tool listing and sorting
- Performance metrics tracking

**pkg/tools/executor_test.go**
- Argument validation against schema
- Execution timeout enforcement
- Error handling and conversion
- Argument sanitization for logging

**pkg/tools/validate_pattern_test.go**
- Pattern validation logic
- Violation detection
- Suggestion generation
- Edge cases (missing pattern, invalid code)

**pkg/tools/search_architecture_test.go**
- Search relevance scoring
- Resource type filtering
- Result limiting
- Query tokenization

**pkg/tools/check_adr_alignment_test.go**
- ADR relationship detection
- Conflict identification
- Alignment classification
- Keyword extraction

**internal/server/handlers_test.go** (additions)
- handleToolsList() response format
- handleToolsCall() with valid/invalid arguments
- Error responses for tool operations
- Integration with existing handlers

### Integration Tests

**internal/server/integration_test.go** (additions)
- Full tool invocation flow (list → call)
- Tool execution with real cache data
- Prompt-tool integration scenarios
- Error propagation through layers

### Performance Tests

**pkg/tools/executor_benchmark_test.go**
- Tool execution overhead
- Argument validation performance
- Concurrent tool invocations

## Security Considerations

### Path Validation

All tools that access files must validate paths:

```go
func validateResourcePath(path string) error {
    // Clean and normalize path
    cleanPath := filepath.Clean(path)
    
    // Ensure path is within mcp/resources/
    if !strings.HasPrefix(cleanPath, "mcp/resources/") {
        return errors.NewValidationError(
            errors.ErrCodeInvalidParams,
            "Path must be within mcp/resources/ directory",
            nil)
    }
    
    // Reject traversal attempts
    if strings.Contains(path, "..") {
        return errors.NewValidationError(
            errors.ErrCodeInvalidParams,
            "Path traversal not allowed",
            nil)
    }
    
    return nil
}
```

### Argument Size Limits

Enforce limits to prevent DoS:

```go
const (
    MaxCodeLength        = 50000  // 50KB
    MaxQueryLength       = 500
    MaxDescriptionLength = 5000
    MaxSearchResults     = 20
)
```

### Execution Timeouts

All tool executions have timeouts:

```go
const (
    DefaultToolTimeout = 10 * time.Second
    MaxToolTimeout     = 30 * time.Second
)

func (te *ToolExecutor) Execute(ctx context.Context, tool Tool, arguments map[string]interface{}) (interface{}, error) {
    ctx, cancel := context.WithTimeout(ctx, DefaultToolTimeout)
    defer cancel()
    
    return tool.Execute(ctx, arguments)
}
```

### Sanitized Logging

Never log sensitive data:

```go
func (te *ToolExecutor) sanitizeArguments(arguments map[string]interface{}) map[string]interface{} {
    sanitized := make(map[string]interface{})
    for key, value := range arguments {
        if strValue, ok := value.(string); ok && len(strValue) > 100 {
            sanitized[key] = fmt.Sprintf("%s... [%d chars]", strValue[:100], len(strValue))
        } else {
            sanitized[key] = value
        }
    }
    return sanitized
}
```

## Performance Optimizations

### Caching

- Tool definitions cached in memory (no file I/O per invocation)
- Document cache reused for all tool operations
- Search results can be cached with TTL (future enhancement)

### Lazy Loading

- Tools registered at startup but dependencies initialized on first use
- Heavy operations (like search indexing) deferred until needed

### Concurrent Execution

- ToolManager is thread-safe with RWMutex
- Multiple tools can execute concurrently
- Each tool execution has independent context and timeout

### Metrics Collection

Track performance for optimization:

```go
type ToolStats struct {
    TotalInvocations      int64
    FailedInvocations     int64
    InvocationsByName     map[string]int64
    TotalExecutionTimeMs  int64
    ExecutionTimeByName   map[string]int64
    TimeoutCount          int64
    mu                    sync.RWMutex
}
```

## Implementation Notes

### Tool Discovery

Initial implementation uses explicit registration:

```go
// In internal/server/initialization.go
func (s *MCPServer) initializeToolsSystem() error {
    s.toolManager = tools.NewToolManager()
    
    // Register built-in tools
    s.toolManager.RegisterTool(tools.NewValidatePatternTool(s.cache))
    s.toolManager.RegisterTool(tools.NewSearchArchitectureTool(s.cache))
    s.toolManager.RegisterTool(tools.NewCheckADRAlignmentTool(s.cache))
    
    return nil
}
```

Future enhancement: Auto-discovery using reflection or plugin system.

### Hot-Reloading

Initial implementation: Tools are static (loaded at startup).

Future enhancement: Watch tool definition files and reload on changes, similar to prompts.

### Extensibility

Adding new tools requires:
1. Create new file in `pkg/tools/` implementing Tool interface
2. Register in `initializeToolsSystem()`
3. Add tests in `pkg/tools/*_test.go`

No changes to protocol handlers or manager needed.

## Migration Path

### Phase 1: Core Infrastructure
- Implement Tool interface and ToolManager
- Add ToolExecutor with validation and timeout
- Update MCP protocol types and capabilities
- Add protocol handlers (tools/list, tools/call)

### Phase 2: Initial Tools
- Implement ValidatePatternTool
- Implement SearchArchitectureTool
- Implement CheckADRAlignmentTool

### Phase 3: Prompt Integration
- Add tool reference expansion in TemplateRenderer
- Implement workflow context management
- Update existing prompts to reference tools

### Phase 4: Enhancements
- Add hot-reloading for tool definitions
- Implement search result caching
- Add more sophisticated validation logic
- Create additional tools based on usage patterns

## Open Questions

1. **Tool Definition Format**: Should tools be defined in JSON files (like prompts) or purely in code?
   - **Decision**: Start with code-based definitions for simplicity. JSON definitions can be added later if needed for dynamic loading.

2. **Workflow State Persistence**: Should workflow context be persisted across server restarts?
   - **Decision**: No persistence initially. Workflows are session-based and ephemeral.

3. **Tool Versioning**: How to handle tool schema changes?
   - **Decision**: Tools are versioned with the server. Breaking changes require server version bump.

4. **Rate Limiting**: Should tool invocations be rate-limited per client?
   - **Decision**: Not initially. Can be added if abuse becomes an issue.
