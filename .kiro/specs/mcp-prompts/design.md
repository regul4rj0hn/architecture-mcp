# Design Document

## Overview

This design document outlines the implementation of MCP Prompts capability for the MCP Architecture Service. The prompts feature will enable users to invoke pre-defined, interactive templates that combine instructions with architectural documentation resources. This creates guided workflows like code reviews, pattern suggestions, and ADR creation assistance.

The design follows the existing architecture patterns in the codebase, integrating prompts as a new primitive alongside the existing resources capability. Prompts will be defined in JSON configuration files, loaded at startup, and served through new MCP protocol handlers.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        MCP Client                            │
│                    (IDE / AI Assistant)                      │
└────────────────────┬────────────────────────────────────────┘
                     │ JSON-RPC over stdio
                     │
┌────────────────────▼────────────────────────────────────────┐
│                    MCP Server                                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Message Handler (server.go)                  │   │
│  │  - prompts/list                                      │   │
│  │  - prompts/get                                       │   │
│  └──────────────┬───────────────────────────────────────┘   │
│                 │                                            │
│  ┌──────────────▼───────────────────────────────────────┐   │
│  │         Prompt Manager (pkg/prompts/)                │   │
│  │  - Load prompt definitions                           │   │
│  │  - Validate arguments                                │   │
│  │  - Render templates                                  │   │
│  │  - Hot-reload support                                │   │
│  └──────────────┬───────────────────────────────────────┘   │
│                 │                                            │
│  ┌──────────────▼───────────────────────────────────────┐   │
│  │      Document Cache (pkg/cache/)                     │   │
│  │  - Retrieve resource content                         │   │
│  │  - Provide to prompt renderer                        │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Component Interaction Flow

1. **Prompt Discovery**: Client sends `prompts/list` → Server returns available prompts from registry
2. **Prompt Invocation**: Client sends `prompts/get` with name and arguments → Server validates, retrieves resources, renders template → Returns complete prompt
3. **Hot Reload**: File monitor detects prompt definition changes → Prompt manager reloads definitions → Registry updated

## Components and Interfaces

### 1. Prompt Models (`internal/models/prompt.go`)

New data structures for representing prompts in the MCP protocol:

```go
// MCPPrompt represents a prompt available to clients
type MCPPrompt struct {
    Name        string                `json:"name"`
    Description string                `json:"description,omitempty"`
    Arguments   []MCPPromptArgument   `json:"arguments,omitempty"`
}

// MCPPromptArgument represents an argument that a prompt accepts
type MCPPromptArgument struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Required    bool   `json:"required"`
}

// MCPPromptsListResult represents the result of prompts/list
type MCPPromptsListResult struct {
    Prompts []MCPPrompt `json:"prompts"`
}

// MCPPromptsGetParams represents parameters for prompts/get
type MCPPromptsGetParams struct {
    Name      string                 `json:"name"`
    Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPPromptMessage represents a message in the prompt response
type MCPPromptMessage struct {
    Role    string                 `json:"role"`
    Content MCPPromptContent       `json:"content"`
}

// MCPPromptContent represents the content of a prompt message
type MCPPromptContent struct {
    Type string `json:"type"` // "text" or "resource"
    Text string `json:"text,omitempty"`
}

// MCPPromptsGetResult represents the result of prompts/get
type MCPPromptsGetResult struct {
    Description string             `json:"description,omitempty"`
    Messages    []MCPPromptMessage `json:"messages"`
}
```

### 2. Prompt Definition Format

Prompts are defined in JSON files stored in `prompts/` directory:

```json
{
  "name": "review-code-against-patterns",
  "description": "Review code against documented architectural patterns",
  "arguments": [
    {
      "name": "code",
      "description": "The code snippet to review",
      "required": true,
      "maxLength": 10000
    },
    {
      "name": "language",
      "description": "Programming language of the code",
      "required": false
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Review the following {{language}} code against our architectural patterns:\n\n```{{language}}\n{{code}}\n```\n\nConsider the following patterns from our documentation:\n\n{{resource:architecture://patterns/*}}"
      }
    }
  ]
}
```

### 3. Prompt Manager (`pkg/prompts/manager.go`)

Core component responsible for managing prompt lifecycle:

```go
type PromptManager struct {
    registry      map[string]*PromptDefinition
    promptsDir    string
    cache         *cache.DocumentCache
    monitor       *monitor.FileSystemMonitor
    mu            sync.RWMutex
    logger        *logging.StructuredLogger
}

// Key methods:
func NewPromptManager(promptsDir string, cache *cache.DocumentCache, monitor *monitor.FileSystemMonitor) *PromptManager
func (pm *PromptManager) LoadPrompts() error
func (pm *PromptManager) GetPrompt(name string) (*PromptDefinition, error)
func (pm *PromptManager) ListPrompts() []models.MCPPrompt
func (pm *PromptManager) RenderPrompt(name string, arguments map[string]interface{}) (*models.MCPPromptsGetResult, error)
func (pm *PromptManager) ReloadPrompts() error
```

### 4. Prompt Definition (`pkg/prompts/definition.go`)

Internal representation of a prompt with validation and rendering logic:

```go
type PromptDefinition struct {
    Name        string
    Description string
    Arguments   []ArgumentDefinition
    Messages    []MessageTemplate
}

type ArgumentDefinition struct {
    Name        string
    Description string
    Required    bool
    MaxLength   int
}

type MessageTemplate struct {
    Role    string
    Content ContentTemplate
}

type ContentTemplate struct {
    Type            string
    Text            string
    ResourcePattern string // e.g., "architecture://patterns/*"
}

// Key methods:
func (pd *PromptDefinition) Validate() error
func (pd *PromptDefinition) ValidateArguments(args map[string]interface{}) error
func (pd *PromptDefinition) Render(args map[string]interface{}, cache *cache.DocumentCache) (*models.MCPPromptsGetResult, error)
```

### 5. Template Renderer (`pkg/prompts/renderer.go`)

Handles template variable substitution and resource embedding:

```go
type TemplateRenderer struct {
    cache *cache.DocumentCache
}

// Key methods:
func NewTemplateRenderer(cache *cache.DocumentCache) *TemplateRenderer
func (tr *TemplateRenderer) RenderTemplate(template string, args map[string]interface{}) (string, error)
func (tr *TemplateRenderer) EmbedResources(template string) (string, error)
func (tr *TemplateRenderer) ResolveResourcePattern(pattern string) ([]*models.Document, error)
```

Template syntax:
- `{{argumentName}}` - Substitutes argument value
- `{{resource:architecture://patterns/*}}` - Embeds all matching resources
- `{{resource:architecture://adr/001}}` - Embeds specific resource

### 6. Server Integration (`internal/server/server.go`)

Add prompt support to the existing MCP server:

```go
// Add to MCPServer struct:
type MCPServer struct {
    // ... existing fields ...
    promptManager *prompts.PromptManager
}

// New handler methods:
func (s *MCPServer) handlePromptsList(message *models.MCPMessage) *models.MCPMessage
func (s *MCPServer) handlePromptsGet(message *models.MCPMessage) *models.MCPMessage

// Update capabilities in handleInitialize:
capabilities: models.MCPCapabilities{
    Resources: &models.MCPResourceCapabilities{...},
    Prompts: &models.MCPPromptCapabilities{
        ListChanged: false,
    },
}
```

## Data Models

### Prompt Definition File Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["name", "messages"],
  "properties": {
    "name": {
      "type": "string",
      "pattern": "^[a-z0-9-]+$"
    },
    "description": {
      "type": "string"
    },
    "arguments": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "required"],
        "properties": {
          "name": {"type": "string"},
          "description": {"type": "string"},
          "required": {"type": "boolean"},
          "maxLength": {"type": "integer", "minimum": 1}
        }
      }
    },
    "messages": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["role", "content"],
        "properties": {
          "role": {"type": "string", "enum": ["user", "assistant"]},
          "content": {
            "type": "object",
            "required": ["type"],
            "properties": {
              "type": {"type": "string", "enum": ["text"]},
              "text": {"type": "string"}
            }
          }
        }
      }
    }
  }
}
```

### Built-in Prompt Definitions

Three initial prompts will be provided:

1. **review-code-against-patterns**: Reviews code against documented patterns
2. **suggest-patterns**: Suggests relevant patterns for a problem description
3. **create-adr**: Assists in creating a new ADR following the template

## Error Handling

### Error Codes

Extend the existing error code system in `pkg/errors/`:

```go
const (
    // Prompt-specific error codes
    ErrCodePromptNotFound      = "PROMPT_NOT_FOUND"
    ErrCodeInvalidPromptDef    = "INVALID_PROMPT_DEFINITION"
    ErrCodePromptArgMissing    = "PROMPT_ARGUMENT_MISSING"
    ErrCodePromptArgTooLong    = "PROMPT_ARGUMENT_TOO_LONG"
    ErrCodePromptRenderFailed  = "PROMPT_RENDER_FAILED"
)
```

### Error Scenarios

1. **Prompt Not Found**: Return -32602 with "Prompt not found" message
2. **Missing Required Argument**: Return -32602 with list of missing arguments
3. **Argument Too Long**: Return -32602 with length limit information
4. **Resource Not Found During Render**: Return -32602 with missing resource details
5. **Invalid Prompt Definition**: Log error, exclude from registry, continue server operation

### Graceful Degradation

- If prompt definitions directory doesn't exist, log warning and continue with empty registry
- If individual prompt file is malformed, log error and skip that prompt
- If resource embedding fails, include error message in rendered prompt instead of failing completely

## Testing Strategy

### Unit Tests

1. **Prompt Definition Validation** (`pkg/prompts/definition_test.go`)
   - Valid definition parsing
   - Invalid definition rejection
   - Argument validation logic

2. **Template Rendering** (`pkg/prompts/renderer_test.go`)
   - Variable substitution
   - Resource pattern matching
   - Resource embedding
   - Edge cases (missing variables, invalid patterns)

3. **Prompt Manager** (`pkg/prompts/manager_test.go`)
   - Prompt loading from files
   - Registry management
   - Hot reload functionality
   - Concurrent access safety

4. **Server Handlers** (`internal/server/server_test.go`)
   - prompts/list request handling
   - prompts/get request handling
   - Error response formatting
   - Integration with existing handlers

### Integration Tests

1. **End-to-End Prompt Flow** (`cmd/mcp-server/main_test.go`)
   - Load prompts from test fixtures
   - Send prompts/list request
   - Send prompts/get request with arguments
   - Verify resource embedding
   - Verify rendered output format

2. **Hot Reload Testing**
   - Modify prompt definition file
   - Verify automatic reload
   - Verify updated prompt available

### Test Fixtures

Create test prompt definitions in `testdata/prompts/`:
- `test-simple.json` - Basic prompt without resources
- `test-with-resources.json` - Prompt with resource embedding
- `test-invalid.json` - Malformed prompt for error testing

## Performance Considerations

### Caching Strategy

- Prompt definitions cached in memory after loading
- Resource content retrieved from existing document cache
- No additional caching layer needed for prompt rendering

### Resource Embedding Optimization

- Limit resource pattern matching to prevent excessive memory usage
- Cap total embedded content size at 1MB per prompt
- Use streaming for large resource sets if needed in future

### Concurrent Access

- Use read-write mutex for prompt registry access
- Allow concurrent prompt rendering (read-only operations)
- Serialize prompt definition reloads (write operations)

### Monitoring Metrics

Add to existing performance metrics:
- Total prompts loaded
- Prompt invocation count by name
- Average render time per prompt
- Resource embedding cache hit rate
- Failed prompt invocations

## Security Considerations

### Input Validation

- Validate prompt names against whitelist pattern: `^[a-z0-9-]+$`
- Enforce maximum argument lengths (default 10,000 characters)
- Sanitize argument values to prevent injection attacks
- Validate resource patterns to prevent path traversal

### Resource Access Control

- Prompts can only access resources already available through resources/list
- No file system access beyond configured documentation directories
- Resource patterns validated against allowed URI schemes

### Denial of Service Prevention

- Limit maximum number of resources embedded per prompt (default: 50)
- Limit total rendered prompt size (default: 1MB)
- Rate limiting handled at client level (out of scope for server)

## Deployment Considerations

### Directory Structure

```
.
├── prompts/                    # Prompt definition files
│   ├── review-code-against-patterns.json
│   ├── suggest-patterns.json
│   └── create-adr.json
├── docs/                       # Existing documentation
│   ├── guidelines/
│   ├── patterns/
│   └── adr/
└── ...
```

### Configuration

No new configuration required. Prompts directory location hardcoded to `prompts/` relative to server root.

### Docker Integration

- Add prompts directory to Docker image
- Mount prompts directory as volume for customization
- Update Dockerfile to copy default prompts

### Kubernetes Integration

- Add ConfigMap for prompt definitions
- Mount ConfigMap to prompts directory
- Support hot-reload when ConfigMap updates

## Migration Path

### Phase 1: Core Implementation
- Implement prompt models and manager
- Add server handlers
- Create three built-in prompts

### Phase 2: Hot Reload
- Integrate with file system monitor
- Implement automatic reload on file changes

### Phase 3: Enhanced Features (Future)
- Prompt versioning
- Prompt composition (prompts referencing other prompts)
- Dynamic resource filtering based on arguments
- Prompt usage analytics

## Open Questions

1. **Should prompts support multiple message roles?** 
   - Decision: Start with user role only, add assistant role if needed later

2. **Should we support prompt templates in formats other than JSON?**
   - Decision: JSON only for initial implementation, YAML support can be added later

3. **How should we handle very large resource sets in patterns?**
   - Decision: Implement size limits and provide clear error messages when exceeded

4. **Should prompts be able to invoke other prompts?**
   - Decision: Not in initial implementation, consider for future enhancement
