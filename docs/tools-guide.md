# MCP Tools Development Guide

This guide explains how to create new tools for the MCP Architecture Service. Tools are executable functions that enable AI agents to perform actions on architectural documentation beyond simple reading.

## Overview

Tools in the MCP Architecture Service follow a registry-based pattern with:
- Interface-based design for extensibility
- JSON schema validation for arguments
- Timeout protection and error handling
- Security constraints (path validation, size limits)
- Performance metrics tracking

## Prompt-Tool Integration

Prompts can reference tools to create guided workflows that combine instructions with executable actions. This enables AI agents to follow structured processes while validating decisions and code.

### Tool References in Prompts

Use the `{{tool:tool-name}}` syntax in prompt templates to embed tool information. When a prompt is retrieved, tool references are automatically expanded to include the tool's description and parameter schema.

**Example prompt with tool reference:**

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
        "text": "I'll help you validate this code against our patterns.\n\n{{tool:validate-against-pattern}}\n\nUse the validate-against-pattern tool with your code to check compliance."
      }
    }
  ]
}
```

When the AI agent retrieves this prompt, the `{{tool:validate-against-pattern}}` reference is expanded to include:
- Tool name and description
- Complete parameter schema with types and constraints
- Expected return value structure

### Workflow Patterns

**1. Sequential Tool Invocation**

Prompts guide the AI through multiple tool calls in sequence:

```json
{
  "name": "comprehensive-architecture-review",
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Let's review your architectural decision:\n\n1. First, search for related patterns:\n{{tool:search-architecture}}\n\n2. Then validate your code:\n{{tool:validate-against-pattern}}\n\n3. Finally, check alignment with existing ADRs:\n{{tool:check-adr-alignment}}"
      }
    }
  ]
}
```

**2. Conditional Tool Usage**

Prompts can suggest tools based on context:

```json
{
  "name": "smart-code-review",
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "I'll review your code. If you mention a specific pattern, I'll use:\n{{tool:validate-against-pattern}}\n\nIf you need pattern suggestions, I'll use:\n{{tool:search-architecture}}"
      }
    }
  ]
}
```

**3. Tool-Driven Workflows**

Complex workflows where tool results inform next steps:

```
AI retrieves prompt → Prompt suggests tool → AI invokes tool → 
Tool returns results → AI analyzes results → AI invokes next tool → 
AI provides final recommendations
```

### Integration Examples

**Example 1: Code Review with Pattern Validation**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/get",
  "params": {
    "name": "review-code-against-patterns",
    "arguments": {
      "code": "func ProcessOrder(order Order) error { ... }",
      "language": "go"
    }
  }
}
```

Response includes embedded tool reference. AI then calls:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "validate-against-pattern",
    "arguments": {
      "code": "func ProcessOrder(order Order) error { ... }",
      "pattern_name": "repository-pattern",
      "language": "go"
    }
  }
}
```

**Example 2: Decision Analysis with ADR Alignment**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "prompts/get",
  "params": {
    "name": "create-adr",
    "arguments": {
      "topic": "Adopting GraphQL for API layer"
    }
  }
}
```

Prompt guides AI to check alignment:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "check-adr-alignment",
    "arguments": {
      "decision_description": "Adopt GraphQL for our API layer",
      "decision_context": "Current REST API requires multiple round trips"
    }
  }
}
```

**Example 3: Architecture Exploration with Guided Search**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "prompts/get",
  "params": {
    "name": "suggest-patterns",
    "arguments": {
      "problem": "Need to handle multiple payment providers"
    }
  }
}
```

AI uses search tool to find relevant patterns:

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "search-architecture",
    "arguments": {
      "query": "payment provider abstraction",
      "resource_type": "patterns",
      "max_results": 5
    }
  }
}
```

### Benefits of Prompt-Tool Integration

1. **Structured Workflows** - Prompts provide step-by-step guidance while tools execute actions
2. **Context Awareness** - Tools have access to full documentation cache for informed analysis
3. **Validation** - Tools enforce constraints and validate inputs before processing
4. **Consistency** - Standardized tool interfaces ensure predictable behavior
5. **Extensibility** - New tools automatically available to all prompts via reference syntax

### Best Practices

**For Prompt Authors:**
- Reference tools explicitly with `{{tool:name}}` syntax
- Explain when and why to use each tool
- Provide context about expected tool inputs
- Guide interpretation of tool outputs

**For Tool Developers:**
- Design tools to work independently or as part of workflows
- Return structured, parseable results
- Include helpful metadata (relevance scores, confidence levels)
- Document expected use cases in tool descriptions

**For Workflow Design:**
- Start with search/discovery tools
- Follow with validation/analysis tools
- End with recommendation/decision tools
- Allow AI flexibility in tool selection

## Tool Interface

All tools must implement the `Tool` interface defined in `pkg/tools/definition.go`:

```go
type Tool interface {
    // Name returns the unique identifier for the tool
    Name() string
    
    // Description returns a human-readable description
    Description() string
    
    // InputSchema returns JSON schema for tool parameters
    InputSchema() map[string]interface{}
    
    // Execute runs the tool with validated arguments
    Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error)
}
```

## Creating a New Tool

### Step 1: Define the Tool Struct

Create a new file in `pkg/tools/` for your tool:

```go
package tools

import (
    "context"
    "fmt"
    
    "github.com/regul4rj0hn/architecture-mcp/pkg/cache"
    "github.com/regul4rj0hn/architecture-mcp/pkg/logging"
)

type MyCustomTool struct {
    cache  *cache.DocumentCache
    logger *logging.StructuredLogger
}

func NewMyCustomTool(cache *cache.DocumentCache) *MyCustomTool {
    return &MyCustomTool{
        cache:  cache,
        logger: logging.NewStructuredLogger("my-custom-tool"),
    }
}
```

### Step 2: Implement the Tool Interface

```go
func (t *MyCustomTool) Name() string {
    return "my-custom-tool"
}

func (t *MyCustomTool) Description() string {
    return "Performs custom analysis on architectural documentation"
}

func (t *MyCustomTool) InputSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input_param": map[string]interface{}{
                "type":        "string",
                "description": "Description of the parameter",
                "maxLength":   1000,
            },
            "optional_param": map[string]interface{}{
                "type":        "string",
                "description": "Optional parameter",
            },
        },
        "required": []string{"input_param"},
    }
}

func (t *MyCustomTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
    // Extract and validate arguments
    inputParam, ok := arguments["input_param"].(string)
    if !ok {
        return nil, fmt.Errorf("input_param must be a string")
    }
    
    // Perform tool logic
    result := t.performAnalysis(inputParam)
    
    // Return structured result
    return map[string]interface{}{
        "status": "success",
        "result": result,
    }, nil
}

func (t *MyCustomTool) performAnalysis(input string) string {
    // Implementation details
    return "analysis result"
}
```

### Step 3: Register the Tool

Add your tool to the server initialization in `internal/server/initialization.go`:

```go
func (s *MCPServer) initializeToolsSystem() error {
    s.toolManager = tools.NewToolManager()
    
    // Existing tools
    s.toolManager.RegisterTool(tools.NewValidatePatternTool(s.cache))
    s.toolManager.RegisterTool(tools.NewSearchArchitectureTool(s.cache))
    s.toolManager.RegisterTool(tools.NewCheckADRAlignmentTool(s.cache))
    
    // Your new tool
    s.toolManager.RegisterTool(tools.NewMyCustomTool(s.cache))
    
    return nil
}
```

### Step 4: Write Tests

Create a test file `pkg/tools/my_custom_tool_test.go`:

```go
package tools

import (
    "context"
    "testing"
    
    "github.com/regul4rj0hn/architecture-mcp/pkg/cache"
)

func TestMyCustomTool_Execute(t *testing.T) {
    tests := []struct {
        name      string
        arguments map[string]interface{}
        wantErr   bool
    }{
        {
            name: "valid input",
            arguments: map[string]interface{}{
                "input_param": "test value",
            },
            wantErr: false,
        },
        {
            name: "missing required parameter",
            arguments: map[string]interface{}{},
            wantErr: true,
        },
    }
    
    cache := cache.NewDocumentCache()
    tool := NewMyCustomTool(cache)
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            _, err := tool.Execute(ctx, tt.arguments)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Best Practices

### Input Validation

Always validate and sanitize inputs:

```go
// Check type
value, ok := arguments["param"].(string)
if !ok {
    return nil, fmt.Errorf("param must be a string")
}

// Check size limits
if len(value) > MaxParamLength {
    return nil, fmt.Errorf("param exceeds maximum length of %d", MaxParamLength)
}

// Validate paths (if accessing files)
if err := validateResourcePath(value); err != nil {
    return nil, err
}
```

### Error Handling

Return descriptive errors that help users understand what went wrong:

```go
if pattern == nil {
    return nil, fmt.Errorf("pattern '%s' not found in cache", patternName)
}

if err := t.performValidation(code); err != nil {
    return nil, fmt.Errorf("validation failed: %w", err)
}
```

### Context Handling

Respect context cancellation and timeouts:

```go
func (t *MyCustomTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Perform work
    result := t.doWork()
    
    return result, nil
}
```

### Structured Results

Return consistent, well-structured results:

```go
return map[string]interface{}{
    "status": "success",
    "data": map[string]interface{}{
        "field1": value1,
        "field2": value2,
    },
    "metadata": map[string]interface{}{
        "execution_time_ms": elapsed.Milliseconds(),
    },
}, nil
```

## Security Considerations

### Path Validation

Never trust user-provided paths. Always validate:

```go
func validateResourcePath(path string) error {
    cleanPath := filepath.Clean(path)
    
    // Ensure path is within mcp/resources/
    if !strings.HasPrefix(cleanPath, "mcp/resources/") {
        return fmt.Errorf("path must be within mcp/resources/ directory")
    }
    
    // Reject traversal attempts
    if strings.Contains(path, "..") {
        return fmt.Errorf("path traversal not allowed")
    }
    
    return nil
}
```

### Size Limits

Enforce limits to prevent resource exhaustion:

```go
const (
    MaxCodeLength        = 50000  // 50KB
    MaxQueryLength       = 500
    MaxDescriptionLength = 5000
)

if len(code) > MaxCodeLength {
    return nil, fmt.Errorf("code exceeds maximum length of %d characters", MaxCodeLength)
}
```

### Timeout Protection

The ToolExecutor automatically enforces timeouts (default 10 seconds), but you can check context in long-running operations:

```go
for i, item := range largeList {
    if i%100 == 0 {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
    }
    // Process item
}
```

## Testing Guidelines

### Unit Tests

Test core functionality in isolation:

```go
func TestMyCustomTool_Name(t *testing.T) {
    tool := NewMyCustomTool(nil)
    if got := tool.Name(); got != "my-custom-tool" {
        t.Errorf("Name() = %v, want %v", got, "my-custom-tool")
    }
}

func TestMyCustomTool_InputSchema(t *testing.T) {
    tool := NewMyCustomTool(nil)
    schema := tool.InputSchema()
    
    if schema["type"] != "object" {
        t.Error("schema type should be object")
    }
    
    props := schema["properties"].(map[string]interface{})
    if _, ok := props["input_param"]; !ok {
        t.Error("schema should include input_param property")
    }
}
```

### Integration Tests

Test with real cache data:

```go
func TestMyCustomTool_Integration(t *testing.T) {
    // Setup cache with test data
    cache := cache.NewDocumentCache()
    // ... populate cache
    
    tool := NewMyCustomTool(cache)
    
    ctx := context.Background()
    result, err := tool.Execute(ctx, map[string]interface{}{
        "input_param": "test value",
    })
    
    if err != nil {
        t.Fatalf("Execute() failed: %v", err)
    }
    
    // Verify result structure
    resultMap := result.(map[string]interface{})
    if resultMap["status"] != "success" {
        t.Error("expected success status")
    }
}
```

### Error Cases

Test error handling thoroughly:

```go
func TestMyCustomTool_ErrorCases(t *testing.T) {
    tests := []struct {
        name      string
        arguments map[string]interface{}
        wantErr   string
    }{
        {
            name:      "missing required param",
            arguments: map[string]interface{}{},
            wantErr:   "input_param",
        },
        {
            name: "invalid param type",
            arguments: map[string]interface{}{
                "input_param": 123,
            },
            wantErr: "must be a string",
        },
    }
    
    tool := NewMyCustomTool(cache.NewDocumentCache())
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := tool.Execute(context.Background(), tt.arguments)
            if err == nil {
                t.Error("expected error but got none")
            }
            if !strings.Contains(err.Error(), tt.wantErr) {
                t.Errorf("error = %v, want substring %v", err, tt.wantErr)
            }
        })
    }
}
```

## Performance Considerations

### Caching

Leverage the document cache for repeated access:

```go
func (t *MyCustomTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
    // Cache is already populated and thread-safe
    documents := t.cache.GetAll()
    
    // Process cached documents
    for _, doc := range documents {
        // ...
    }
    
    return result, nil
}
```

### Lazy Computation

Defer expensive operations until needed:

```go
func (t *MyCustomTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
    // Quick validation first
    if err := t.validateInput(arguments); err != nil {
        return nil, err
    }
    
    // Expensive computation only if validation passes
    result := t.performExpensiveAnalysis(arguments)
    
    return result, nil
}
```

### Metrics

The ToolManager automatically tracks performance metrics. You can add custom logging:

```go
func (t *MyCustomTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
    start := time.Now()
    defer func() {
        t.logger.Info("tool execution completed", map[string]interface{}{
            "duration_ms": time.Since(start).Milliseconds(),
        })
    }()
    
    // Tool logic
    return result, nil
}
```

## Examples

See the built-in tools for complete examples:

- `pkg/tools/validate_pattern.go` - Pattern validation with heuristic analysis
- `pkg/tools/search_architecture.go` - Full-text search with relevance scoring
- `pkg/tools/check_adr_alignment.go` - ADR relationship analysis

## Tool Lifecycle

1. **Registration** - Tool registered in `initializeToolsSystem()`
2. **Discovery** - Tool appears in `tools/list` response
3. **Invocation** - AI agent calls `tools/call` with tool name and arguments
4. **Validation** - ToolExecutor validates arguments against schema
5. **Execution** - Tool's `Execute()` method runs with timeout protection
6. **Response** - Result returned to agent in MCP format

## Troubleshooting

### Tool Not Appearing in List

- Verify tool is registered in `initializeToolsSystem()`
- Check server logs for registration errors
- Ensure tool implements all interface methods

### Validation Errors

- Verify InputSchema() returns valid JSON schema
- Check that required fields are specified
- Ensure argument types match schema

### Execution Timeouts

- Default timeout is 10 seconds
- Optimize expensive operations
- Check for blocking operations without context checks
- Consider breaking work into smaller chunks

### Cache Access Issues

- Ensure cache is populated before tool execution
- Check that document URIs match expected patterns
- Verify file system permissions for resource directories
