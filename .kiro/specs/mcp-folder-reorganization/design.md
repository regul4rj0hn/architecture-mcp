# Design Document

## Overview

This design addresses two major improvements to the MCP Architecture Service:

1. **Directory Reorganization**: Migrate from `docs/` and `prompts/` to `mcp/resources/` and `mcp/prompts/` to separate MCP assets from project documentation
2. **Code Quality Refactoring**: Apply Effective Go principles to improve maintainability, readability, and performance

The refactoring will be done incrementally to ensure the system remains functional throughout the process.

## Architecture

### Current State

```
Project Root
├── docs/                    # MCP resources (to be moved)
│   ├── guidelines/
│   ├── patterns/
│   └── adr/
├── prompts/                 # MCP prompts (to be moved)
├── internal/server/
│   └── server.go           # 1200+ lines, needs splitting
├── pkg/
│   ├── cache/
│   ├── prompts/
│   ├── scanner/
│   └── ...
└── cmd/mcp-server/
    └── main.go
```

### Target State

```
Project Root
├── mcp/                     # All MCP assets consolidated
│   ├── resources/
│   │   ├── guidelines/
│   │   ├── patterns/
│   │   └── adr/
│   └── prompts/
├── docs/                    # Available for project documentation
├── internal/server/
│   ├── server.go           # Core server struct and lifecycle
│   ├── handlers.go         # MCP protocol handlers
│   └── initialization.go   # System initialization logic
├── pkg/                     # Refactored packages
└── cmd/mcp-server/
    └── main.go
```

## Components and Interfaces

### 1. Path Configuration

**Design Decision**: Introduce path constants to centralize directory configuration

```go
// internal/server/config.go (new file)
package server

const (
    ResourcesBasePath = "mcp/resources"
    PromptsBasePath   = "mcp/prompts"
    
    GuidelinesPath = ResourcesBasePath + "/guidelines"
    PatternsPath   = ResourcesBasePath + "/patterns"
    ADRPath        = ResourcesBasePath + "/adr"
)
```

**Rationale**: Centralizing paths makes future changes easier and eliminates magic strings throughout the codebase.

### 2. Server Package Refactoring

**Design Decision**: Split `internal/server/server.go` into three focused files

#### server.go (Core)
- Server struct definition
- Constructor (NewMCPServer)
- Lifecycle methods (Start, Shutdown)
- Core coordination logic
- ~200-300 lines

#### handlers.go (Protocol Handlers)
- handleInitialize
- handleResourcesList
- handleResourcesRead
- handlePromptsList
- handlePromptsGet
- handleServerPerformance
- ~300-400 lines

#### initialization.go (System Setup)
- initializeDocumentationSystem
- initializePromptSystem
- setupFileSystemMonitoring
- Helper functions for concurrent initialization
- ~200-300 lines

**Rationale**: This separation follows single responsibility principle and makes the codebase easier to navigate. Each file has a clear purpose.

### 3. Scanner Package Improvements

**Current Issues**:
- `getCategoryFromPath` uses string matching on full paths
- Hardcoded "docs/" references

**Design Decision**: Update path detection to use new base paths

```go
// pkg/scanner/scanner.go
func (ds *DocumentationScanner) getCategoryFromPath(path string) string {
    normalizedPath := filepath.ToSlash(strings.ToLower(path))
    
    // Use constants from server package or pass as parameter
    if strings.Contains(normalizedPath, "guidelines") {
        return "guideline"
    }
    if strings.Contains(normalizedPath, "patterns") {
        return "pattern"
    }
    if strings.Contains(normalizedPath, "adr") {
        return "adr"
    }
    return "unknown"
}
```

**Rationale**: Category detection should be based on subdirectory names, not full paths, making it resilient to base path changes.

### 4. Prompt Manager Updates

**Current State**: Hardcoded "prompts" directory in NewPromptManager call

**Design Decision**: Pass prompts directory as parameter (already done), update call sites

```go
// internal/server/server.go
promptManager := prompts.NewPromptManager(PromptsBasePath, docCache, fileMonitor)
```

**Rationale**: The PromptManager is already designed to accept a configurable path; we just need to update the call site.

### 5. Test Refactoring Strategy

**Design Decision**: Split large test files by functional area

#### Current Large Files:
- `internal/server/server_test.go` (~900 lines)
- `internal/server/integration_test.go` (~800 lines)

#### Proposed Split:

**server_test.go** → Split into:
- `server_lifecycle_test.go` - Start, Shutdown, initialization tests
- `server_handlers_test.go` - Protocol handler tests
- `server_resources_test.go` - Resource-specific tests
- `server_prompts_test.go` - Prompt-specific tests

**integration_test.go** → Keep as is (integration tests benefit from being together)

**Rationale**: Splitting by functional area makes tests easier to find and maintain while keeping related tests together.

### 6. Code Quality Improvements

#### Early Returns Pattern

**Before**:
```go
func (s *MCPServer) handleResourcesRead(message *models.MCPMessage) *models.MCPMessage {
    if !s.initialized {
        return createErrorResponse(message.ID, -32002, "Server not initialized", nil)
    } else {
        // 50 lines of logic
    }
}
```

**After**:
```go
func (s *MCPServer) handleResourcesRead(message *models.MCPMessage) *models.MCPMessage {
    if !s.initialized {
        return createErrorResponse(message.ID, -32002, "Server not initialized", nil)
    }
    
    // 50 lines of logic at base indentation level
}
```

#### Extract Magic Strings

**Before**:
```go
if strings.Contains(normalizedPath, "docs/guidelines") {
    // ...
}
```

**After**:
```go
const (
    categoryGuideline = "guideline"
    categoryPattern   = "pattern"
    categoryADR       = "adr"
)

if strings.Contains(normalizedPath, "guidelines") {
    return categoryGuideline
}
```

#### Reduce Nesting

**Before**:
```go
if condition1 {
    if condition2 {
        if condition3 {
            // logic
        }
    }
}
```

**After**:
```go
if !condition1 {
    return earlyExit
}
if !condition2 {
    return earlyExit
}
if !condition3 {
    return earlyExit
}
// logic at base level
```

## Data Models

No changes to data models are required. The existing models in `internal/models/` are well-structured and follow Go conventions.

## Error Handling

**Current State**: Error handling is well-implemented with structured errors in `pkg/errors/`

**Design Decision**: Maintain existing error handling patterns, ensure all new code follows the same approach

**Rationale**: The existing error handling with circuit breakers and graceful degradation is solid and doesn't need changes.

## Testing Strategy

### Unit Tests
- Update all path references from `docs/` to `mcp/resources/`
- Update all path references from `prompts/` to `mcp/prompts/`
- Ensure table-driven tests use constants for paths
- Split large test files as described above

### Integration Tests
- Update test fixtures to use new directory structure
- Verify file system monitoring works with new paths
- Test Docker container with new volume mounts

### Benchmark Tests
- Update benchmark test paths
- Verify performance is maintained or improved after refactoring

### Coverage Goals
- Maintain >70% coverage for critical packages
- Focus on behavior verification
- Avoid over-testing implementation details

## Docker and Deployment

### Dockerfile Changes

**Before**:
```dockerfile
COPY --chown=mcpuser:mcpuser docs/ /app/docs/
COPY --chown=mcpuser:mcpuser prompts/ /app/prompts/
```

**After**:
```dockerfile
COPY --chown=mcpuser:mcpuser mcp/ /app/mcp/
```

**Rationale**: Simpler, copies entire MCP directory structure in one operation.

### docker-compose.yml Changes

**Before**:
```yaml
volumes:
  - ./docs:/app/docs:ro
  - ./prompts:/app/prompts:ro
```

**After**:
```yaml
volumes:
  - ./mcp:/app/mcp:ro
```

**Rationale**: Single volume mount for all MCP assets, simpler configuration.

## Documentation Updates

### Files to Update

1. **README.md**
   - Update directory structure diagram
   - Update example commands
   - Update MCP configuration examples

2. **.kiro/steering/product.md**
   - Update resource URI documentation
   - Update directory references
   - Update caching documentation

3. **.kiro/steering/structure.md**
   - Update directory structure diagram
   - Update package organization

4. **SECURITY.md**
   - Update path validation documentation
   - Update directory restriction references

5. **docs/prompts-guide.md**
   - Move to `mcp/docs/prompts-guide.md` or keep in root docs/
   - Update all path references

## Migration Strategy

### Phase 1: Path Constants and Configuration
1. Create `internal/server/config.go` with path constants
2. Update server initialization to use constants
3. Run tests to verify no regressions

### Phase 2: Server Package Refactoring
1. Create `internal/server/handlers.go` and move handler functions
2. Create `internal/server/initialization.go` and move init functions
3. Update `server.go` to reference new files
4. Run tests after each file split

### Phase 3: Test Updates
1. Update all test path references
2. Split large test files
3. Verify 100% test pass rate

### Phase 4: Docker and Documentation
1. Update Dockerfile and docker-compose.yml
2. Update all documentation files
3. Test Docker build and run

### Phase 5: Code Quality Pass
1. Run `go fmt` on all files
2. Apply early return patterns
3. Extract magic strings to constants
4. Reduce nesting where found
5. Review and improve comments

## Performance Considerations

### Expected Impact
- **Neutral**: Path changes should have no performance impact
- **Positive**: Code refactoring may improve readability but shouldn't affect runtime performance
- **Monitoring**: Use existing performance metrics to verify no degradation

### Benchmarks to Run
- Document scanning performance
- Cache lookup performance
- Prompt rendering performance
- Overall request handling latency

## Rollback Plan

If issues are discovered:
1. All changes are in version control
2. Can revert to previous commit
3. Tests provide safety net for incremental changes
4. Each phase is independently testable

## Success Criteria

1. All tests pass with new directory structure
2. Docker container builds and runs successfully
3. `go fmt` passes on all files
4. No files exceed 500 lines
5. No functions exceed 100 lines (with reasonable exceptions)
6. Test coverage remains >70% for critical packages
7. Documentation accurately reflects new structure
8. Performance benchmarks show no regression
