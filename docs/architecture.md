# Architecture Overview

This document provides architectural diagrams for the MCP Architecture Service, including the tools subsystem.

## System Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        AI[AI Agent/IDE]
    end
    
    subgraph "MCP Protocol Layer"
        JSONRPC[JSON-RPC 2.0<br/>stdio/TCP]
    end
    
    subgraph "Server Layer"
        INIT[Initialize Handler]
        RES[Resources Handler]
        PROMPT[Prompts Handler]
        TOOL[Tools Handler]
    end
    
    subgraph "Core Services"
        CACHE[Document Cache]
        SCANNER[Documentation Scanner]
        MONITOR[File System Monitor]
        PMGR[Prompt Manager]
        TMGR[Tool Manager]
    end
    
    subgraph "Storage"
        RESOURCES[mcp/resources/<br/>guidelines, patterns, adr]
        PROMPTS[mcp/prompts/<br/>*.json]
    end
    
    AI -->|JSON-RPC| JSONRPC
    JSONRPC --> INIT
    JSONRPC --> RES
    JSONRPC --> PROMPT
    JSONRPC --> TOOL
    
    INIT --> CACHE
    RES --> CACHE
    PROMPT --> PMGR
    TOOL --> TMGR
    
    PMGR --> CACHE
    PMGR --> TMGR
    TMGR --> CACHE
    
    SCANNER --> CACHE
    MONITOR --> SCANNER
    MONITOR --> RESOURCES
    MONITOR --> PROMPTS
    
    SCANNER --> RESOURCES
    PMGR --> PROMPTS
```

## Tools Subsystem Architecture

```mermaid
graph TB
    subgraph "Protocol Layer"
        LIST[tools/list]
        CALL[tools/call]
    end
    
    subgraph "Tool Management"
        TMGR[Tool Manager<br/>Registry & Lifecycle]
        EXEC[Tool Executor<br/>Validation & Execution]
    end
    
    subgraph "Tool Implementations"
        VALIDATE[Validate Pattern Tool]
        SEARCH[Search Architecture Tool]
        CHECK[Check ADR Alignment Tool]
        CUSTOM[Custom Tools...]
    end
    
    subgraph "Dependencies"
        CACHE[Document Cache]
        CB[Circuit Breaker]
        METRICS[Performance Metrics]
    end
    
    LIST --> TMGR
    CALL --> TMGR
    TMGR --> EXEC
    
    EXEC --> VALIDATE
    EXEC --> SEARCH
    EXEC --> CHECK
    EXEC --> CUSTOM
    
    VALIDATE --> CACHE
    SEARCH --> CACHE
    CHECK --> CACHE
    
    EXEC --> CB
    EXEC --> METRICS
    TMGR --> METRICS
```

## Tool Execution Flow

```mermaid
sequenceDiagram
    participant AI as AI Agent
    participant Server as MCP Server
    participant Manager as Tool Manager
    participant Executor as Tool Executor
    participant Tool as Tool Implementation
    participant Cache as Document Cache
    
    AI->>Server: tools/list
    Server->>Manager: ListTools()
    Manager-->>Server: Tool definitions with schemas
    Server-->>AI: Available tools
    
    AI->>Server: tools/call (name, arguments)
    Server->>Manager: ExecuteTool(name, args)
    Manager->>Executor: ValidateArguments(tool, args)
    Executor-->>Manager: Validation OK
    
    Manager->>Executor: Execute(tool, args)
    Executor->>Tool: Execute(ctx, args)
    Tool->>Cache: GetAll() / Get(uri)
    Cache-->>Tool: Documents
    Tool->>Tool: Process & Analyze
    Tool-->>Executor: Result
    Executor-->>Manager: Result
    Manager->>Manager: Track Metrics
    Manager-->>Server: Result
    Server-->>AI: Tool execution result
```

## Prompt-Tool Integration Flow

```mermaid
sequenceDiagram
    participant AI as AI Agent
    participant Server as MCP Server
    participant PromptMgr as Prompt Manager
    participant Renderer as Template Renderer
    participant ToolMgr as Tool Manager
    participant Tool as Tool Implementation
    
    AI->>Server: prompts/get (name, args)
    Server->>PromptMgr: GetPrompt(name, args)
    PromptMgr->>Renderer: Render(template, args)
    
    Note over Renderer: Find {{tool:name}} references
    Renderer->>ToolMgr: GetTool(name)
    ToolMgr-->>Renderer: Tool definition & schema
    Renderer->>Renderer: Expand tool reference
    
    Renderer-->>PromptMgr: Rendered prompt with tool info
    PromptMgr-->>Server: Prompt messages
    Server-->>AI: Prompt with embedded tool details
    
    Note over AI: AI decides to use tool
    AI->>Server: tools/call (name, args)
    Server->>ToolMgr: ExecuteTool(name, args)
    ToolMgr->>Tool: Execute(ctx, args)
    Tool-->>ToolMgr: Result
    ToolMgr-->>Server: Result
    Server-->>AI: Tool result
    
    Note over AI: AI uses result in workflow
```

## Component Responsibilities

### MCP Server (`internal/server/`)
- Protocol message routing
- Request/response handling
- Error conversion to MCP format
- Capability negotiation

### Tool Manager (`pkg/tools/manager.go`)
- Tool registration and discovery
- Tool lookup by name
- Performance metrics tracking
- Coordination with executor

### Tool Executor (`pkg/tools/executor.go`)
- Argument validation against JSON schema
- Timeout enforcement (10s default)
- Security checks (path validation, size limits)
- Error handling and sanitization

### Tool Implementations (`pkg/tools/*.go`)
- Self-contained functionality
- Cache-based data access
- Structured result generation
- Context-aware execution

### Document Cache (`pkg/cache/`)
- Thread-safe in-memory storage
- Fast document lookup by URI
- Automatic invalidation on file changes
- Shared across all tools

### Prompt Manager (`pkg/prompts/`)
- Prompt loading and validation
- Template rendering with tool references
- Argument substitution
- Resource embedding

## Data Flow

### Resource Access
```
File System → Scanner → Cache → Tools → Results
     ↓
  Monitor (fsnotify)
     ↓
  Auto-refresh
```

### Tool Invocation
```
AI Agent → JSON-RPC → Handler → Manager → Executor → Tool → Cache
                                    ↓
                              Metrics Tracking
                                    ↓
                              Circuit Breaker
```

### Prompt-Tool Workflow
```
AI Agent → Prompt Request → Template Renderer → Tool Reference Expansion
                                                        ↓
                                                  Tool Definitions
                                                        ↓
AI Agent ← Rendered Prompt ← Embedded Tool Info ←──────┘
    ↓
Tool Invocation → Tool Execution → Result → Workflow Continuation
```

## Security Boundaries

```mermaid
graph TB
    subgraph "Trusted Zone"
        SERVER[MCP Server<br/>UID 1001]
        TOOLS[Tool Implementations]
        CACHE[Document Cache]
    end
    
    subgraph "Restricted Access"
        RESOURCES[mcp/resources/<br/>Read-Only]
    end
    
    subgraph "Validation Layer"
        PATH[Path Validation]
        SIZE[Size Limits]
        TIMEOUT[Timeout Protection]
        SCHEMA[Schema Validation]
    end
    
    subgraph "External"
        AI[AI Agent<br/>Untrusted Input]
    end
    
    AI -->|JSON-RPC| SCHEMA
    SCHEMA --> PATH
    PATH --> SIZE
    SIZE --> TIMEOUT
    TIMEOUT --> SERVER
    
    SERVER --> TOOLS
    TOOLS -->|Validated Paths| RESOURCES
    TOOLS --> CACHE
    CACHE -->|Scanned Content| RESOURCES
```

## Performance Optimizations

### Caching Strategy
- Documents cached in memory at startup
- Hot-reload on file system changes (< 2s)
- Thread-safe concurrent access with RWMutex
- No disk I/O during tool execution

### Concurrent Execution
- Multiple tools can execute simultaneously
- Each tool has independent context and timeout
- Shared cache with read-write locking
- Circuit breaker per tool to prevent cascading failures

### Metrics Collection
- Tool invocation counts by name
- Execution time tracking
- Failure rate monitoring
- Timeout detection

## Extensibility Points

### Adding New Tools
1. Implement `Tool` interface in `pkg/tools/`
2. Register in `initializeToolsSystem()`
3. Add tests in `pkg/tools/*_test.go`
4. Document in README and tools guide

### Adding New Prompt Templates
1. Create JSON file in `mcp/prompts/`
2. Use `{{tool:name}}` syntax for tool references
3. Auto-discovered by Prompt Manager
4. Hot-reloaded on file changes

### Custom Validation Logic
1. Extend Tool implementations
2. Add custom schema constraints
3. Implement domain-specific checks
4. Return structured violation reports

## Deployment Architecture

```mermaid
graph TB
    subgraph "Container"
        SERVER[MCP Server Process<br/>Non-root UID 1001]
        RESOURCES[/mcp/resources<br/>Read-Only Mount]
        PROMPTS[/mcp/prompts<br/>Read-Only Mount]
    end
    
    subgraph "Host System"
        DOCS[Documentation Files]
        IDE[IDE/AI Agent]
    end
    
    DOCS -->|Volume Mount| RESOURCES
    DOCS -->|Volume Mount| PROMPTS
    IDE -->|stdio/TCP| SERVER
    
    SERVER -->|Read| RESOURCES
    SERVER -->|Read| PROMPTS
```

### Container Constraints
- Alpine Linux base image
- Non-root user (UID 1001)
- Read-only root filesystem
- No network listeners (stdio only)
- Resource limits: 256M memory, 0.2 CPU
- Static binary (CGO disabled)
