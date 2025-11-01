# MCP Architecture Service Design Document

## Overview

The MCP Architecture Service is a Model Context Protocol (MCP) server that provides structured access to architectural documentation stored in a Git repository. The service implements the MCP specification to enable AI agents to discover and retrieve guidelines, design patterns, and Architecture Decision Records (ADRs) as contextual resources.

The service operates as an MCP server that communicates via JSON-RPC over stdio, automatically monitors local documentation files, and maintains an in-memory cache for fast resource retrieval. It exposes documentation as MCP resources that can be discovered and read by MCP clients.

## Architecture

### High-Level Architecture

```mermaid
graph TB
    subgraph "MCP Client Environment"
        IDE[IDE/Editor with MCP Client]
        AI[AI Agent with MCP Client]
    end
    
    subgraph "Container Platform"
        subgraph "MCP Service Pod"
            MCP[MCP Server (JSON-RPC over stdio)]
            Cache[In-Memory Cache]
            Monitor[File System Monitor]
            Scanner[Documentation Scanner]
        end
        
        subgraph "File System"
            Docs["/docs Directory"]
            Guidelines["/docs/guidelines"]
            Patterns["/docs/patterns"] 
            ADRs["/docs/adr"]
        end
    end
    
    subgraph "CI/CD Pipeline"
        Git[Git Repository]
        Deploy[Deployment System]
    end
    
    IDE -.->|MCP Protocol| MCP
    AI -.->|MCP Protocol| MCP
    MCP --> Cache
    Monitor --> Cache
    Scanner --> Cache
    Scanner --> Docs
    Monitor --> Docs
    Cache --> Guidelines
    Cache --> Patterns
    Cache --> ADRs
    Git --> Deploy
    Deploy --> MCP
```

### Service Architecture Patterns

- **Microservice Architecture**: Single-purpose service with well-defined API boundaries
- **Cache-Aside Pattern**: In-memory caching with automatic invalidation on file changes
- **Observer Pattern**: File system monitoring for automatic cache updates
- **Repository Pattern**: Abstracted data access layer for documentation retrieval

## Components and Interfaces

### MCP Protocol Layer

**Primary Interface**: JSON-RPC over stdio implementing MCP specification

**Required MCP Methods**:
- `initialize` - Server initialization and capability negotiation
- `resources/list` - List all available documentation resources
- `resources/read` - Read specific documentation resource content
- `notifications/initialized` - Acknowledge successful initialization

**MCP Resource URIs**:
- `architecture://guidelines/{path}` - Guidelines documentation
- `architecture://patterns/{path}` - Design patterns documentation  
- `architecture://adr/{adr_id}` - Architecture Decision Records

**MCP Message Format**: JSON-RPC 2.0 compliant messages:
```json
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "method": "resources/read",
  "params": {
    "uri": "architecture://guidelines/api-design"
  }
}
```

**Resource Response Format**:
```json
{
  "jsonrpc": "2.0", 
  "id": "request-id",
  "result": {
    "contents": [
      {
        "uri": "architecture://guidelines/api-design",
        "mimeType": "text/markdown",
        "text": "# API Design Guidelines\n..."
      }
    ]
  }
}
```

### Documentation Scanner

**Purpose**: Scans local documentation directories and builds structured indexes

**Responsibilities**:
- Recursive directory scanning of `/docs` subdirectories
- Markdown file parsing and metadata extraction
- Index building with categorization
- Error handling for malformed documents

**Interface**:
```go
type DocumentationScanner struct {
  // internal fields
}

func (ds *DocumentationScanner) ScanDirectory(path string) (*DocumentIndex, error)
func (ds *DocumentationScanner) ParseMarkdownFile(filePath string) (*DocumentMetadata, error)
func (ds *DocumentationScanner) ExtractMetadata(content string) *DocumentMetadata
```

### File System Monitor

**Purpose**: Monitors documentation directories for changes and triggers cache updates

**Responsibilities**:
- File system event monitoring (create, modify, delete)
- Debounced change detection to avoid excessive updates
- Cache invalidation and refresh coordination
- Error handling for file system access issues

**Interface**:
```go
type FileSystemMonitor struct {
  // internal fields
}

func (fsm *FileSystemMonitor) WatchDirectory(path string, callback func(event FileEvent)) error
func (fsm *FileSystemMonitor) StopWatching() error
```

### In-Memory Cache

**Purpose**: High-performance caching layer for parsed documentation

**Responsibilities**:
- Document storage with structured indexing
- Fast retrieval by path and category
- Automatic invalidation on file changes
- Memory management and cleanup

**Interface**:
```go
type DocumentCache struct {
  // internal fields with sync.RWMutex for concurrency
}

func (dc *DocumentCache) Get(key string) (*Document, error)
func (dc *DocumentCache) Set(key string, document *Document)
func (dc *DocumentCache) Invalidate(key string)
func (dc *DocumentCache) Clear()
func (dc *DocumentCache) GetIndex(category string) *DocumentIndex
```

## Data Models

### Document Metadata
```go
type DocumentMetadata struct {
  Title        string    `json:"title"`
  Category     string    `json:"category"` // "guideline", "pattern", "adr"
  Path         string    `json:"path"`
  LastModified time.Time `json:"lastModified"`
  Size         int64     `json:"size"`
  Checksum     string    `json:"checksum"`
}
```

### Document Content
```go
type DocumentContent struct {
  Sections   []DocumentSection `json:"sections"`
  RawContent string            `json:"rawContent"`
}

type DocumentSection struct {
  Heading     string             `json:"heading"`
  Level       int                `json:"level"`
  Content     string             `json:"content"`
  Subsections []DocumentSection  `json:"subsections,omitempty"`
}
```

### ADR Specific Model
```go
type ADRDocument struct {
  DocumentMetadata
  ADRId         string    `json:"adrId"`
  Status        string    `json:"status"` // "Pending", "Deciding", "Accepted", "Superseded"
  Date          time.Time `json:"date"`
  Deciders      []Decider `json:"deciders"`
  TechnicalStory string   `json:"technicalStory"`
}

type Decider struct {
  FullName string `json:"fullName"`
  Role     string `json:"role"`
  RACI     string `json:"raci"` // "Accountable", "Responsible", "Consulted", "Informed"
}
```

### MCP Protocol Models
```go
// MCP JSON-RPC Message
type MCPMessage struct {
  JSONRPC string      `json:"jsonrpc"`
  ID      interface{} `json:"id,omitempty"`
  Method  string      `json:"method,omitempty"`
  Params  interface{} `json:"params,omitempty"`
  Result  interface{} `json:"result,omitempty"`
  Error   *MCPError   `json:"error,omitempty"`
}

// MCP Resource
type MCPResource struct {
  URI         string            `json:"uri"`
  Name        string            `json:"name"`
  Description string            `json:"description,omitempty"`
  MimeType    string            `json:"mimeType,omitempty"`
  Annotations map[string]string `json:"annotations,omitempty"`
}

// MCP Resource Content
type MCPResourceContent struct {
  URI      string `json:"uri"`
  MimeType string `json:"mimeType"`
  Text     string `json:"text,omitempty"`
  Blob     string `json:"blob,omitempty"`
}

// MCP Error
type MCPError struct {
  Code    int         `json:"code"`
  Message string      `json:"message"`
  Data    interface{} `json:"data,omitempty"`
}

// MCP Server Info
type MCPServerInfo struct {
  Name    string `json:"name"`
  Version string `json:"version"`
}
```

## Error Handling

### Error Categories

1. **File System Errors**
   - File not found (404)
   - Permission denied (403)
   - File system unavailable (503)

2. **Parsing Errors**
   - Malformed Markdown (422)
   - Invalid metadata (422)
   - Encoding issues (422)

3. **Cache Errors**
   - Memory exhaustion (503)
   - Cache corruption (500)
   - Concurrent access issues (500)

4. **API Errors**
   - Invalid request parameters (400)
   - Path traversal attempts (400)
   - Rate limiting (429)

### Error Handling Strategy

- **Graceful Degradation**: Service continues with partial functionality when non-critical errors occur
- **Circuit Breaker**: Temporary failure isolation for file system operations
- **Retry Logic**: Exponential backoff for transient failures
- **Structured Logging**: Comprehensive error logging without sensitive information exposure

### Error Response Format
```json
{
  "error": "Resource not found",
  "code": 404,
  "timestamp": "2024-01-15T10:30:00Z",
  "path": "/api/v1/context/guideline/nonexistent",
  "requestId": "uuid"
}
```

## Testing Strategy

### Unit Testing
- **Component Isolation**: Test individual components with mocked dependencies
- **Coverage Target**: 80% code coverage for core business logic
- **Test Categories**:
  - Document parsing and validation
  - Cache operations and invalidation
  - API endpoint behavior
  - Error handling scenarios

### Integration Testing
- **File System Integration**: Test with real file system operations
- **Cache Integration**: Test cache behavior with file system changes
- **API Integration**: End-to-end API testing with real documentation files

### Performance Testing
- **Load Testing**: API response times under concurrent requests
- **Memory Testing**: Cache behavior with large documentation sets
- **Startup Testing**: Service initialization time with various documentation sizes

### Container Testing
- **Docker Build**: Verify container builds successfully
- **Runtime Testing**: Test container startup and health checks
- **Security Testing**: Verify non-root user execution and minimal privileges

## Deployment Architecture

### Container Specification
- **Base Image**: Alpine Linux for minimal attack surface
- **Runtime**: Go runtime with static binary compilation
- **User**: Non-root user for security compliance
- **Communication**: JSON-RPC over stdio (no network ports required)
- **Health Check**: Process health monitoring and documentation directory access

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: service-architecture-mcp
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  template:
    spec:
      containers:
      - name: mcp-service
        image: service-architecture-mcp:latest
        ports:
        - containerPort: 3000
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 3000
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 3000
```

### CI/CD Pipeline
1. **Source Control**: Git webhook triggers on monitored branch
2. **Build Stage**: Docker image build with documentation files
3. **Test Stage**: Automated testing suite execution
4. **Deploy Stage**: Rolling deployment to container platform
5. **Verification**: Health check validation and traffic routing

## Security Considerations

### Input Validation
- Path parameter sanitization to prevent directory traversal
- Request size limits to prevent DoS attacks
- Content-Type validation for API requests

### Access Control
- No authentication required for read-only operations
- Rate limiting to prevent abuse
- CORS configuration for browser-based IDE integration

### Container Security
- Non-root user execution
- Minimal base image with security updates
- Read-only file system where possible
- Resource limits to prevent resource exhaustion

### Monitoring and Observability
- Structured logging for security events
- Request tracing for audit purposes
- Performance metrics for anomaly detection