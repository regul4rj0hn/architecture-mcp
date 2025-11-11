# Implementation Plan

- [x] 1. Create path configuration constants
  - Create `internal/server/config.go` with path constants for resources and prompts directories
  - Define constants: ResourcesBasePath, PromptsBasePath, GuidelinesPath, PatternsPath, ADRPath
  - Define category constants: categoryGuideline, categoryPattern, categoryADR, categoryUnknown
  - _Requirements: 1.1, 1.4, 2.1, 6.6_

- [-] 2. Update server initialization to use new paths
  - [x] 2.1 Update initializeDocumentationSystem in server.go
    - Replace hardcoded "docs/guidelines", "docs/patterns", "docs/adr" with constants from config.go
    - Update docDirs slice to use GuidelinesPath, PatternsPath, ADRPath constants
    - _Requirements: 1.1, 1.4, 6.6_
  
  - [x] 2.2 Update prompt manager initialization
    - Update NewPromptManager call to use PromptsBasePath constant instead of "prompts"
    - _Requirements: 2.1, 6.6_
  
  - [x] 2.3 Update file system monitoring setup
    - Ensure monitor watches mcp/resources/ and mcp/prompts/ directories
    - _Requirements: 1.3, 2.2_

- [x] 3. Update path resolution in server handlers
  - [x] 3.1 Update buildURIFromPath function
    - Replace "docs/guidelines/", "docs/patterns/", "docs/adr/" prefix trimming with new paths
    - Use constants from config.go for path operations
    - _Requirements: 1.2, 6.6_
  
  - [x] 3.2 Update resolveResourcePath function
    - Replace "docs/guidelines", "docs/patterns", "docs/adr" path construction with new paths
    - Use constants from config.go for path building
    - _Requirements: 1.2, 6.6_

- [x] 4. Update scanner package for new paths
  - Update getCategoryFromPath to be path-agnostic (already uses subdirectory names)
  - Verify scanner works correctly with mcp/resources/ base path
  - _Requirements: 1.1, 6.8_

- [x] 5. Split server.go into focused modules
  - [x] 5.1 Create internal/server/handlers.go
    - Move handleInitialize, handleResourcesList, handleResourcesRead functions
    - Move handlePromptsList, handlePromptsGet, handleServerPerformance functions
    - Move helper functions: createErrorResponse, createSuccessResponse
    - Ensure all handler functions are methods on MCPServer struct
    - _Requirements: 6.2, 6.3, 7.1_
  
  - [x] 5.2 Create internal/server/initialization.go
    - Move initializeDocumentationSystem and related concurrent initialization functions
    - Move initializeDocumentationSystemConcurrent function
    - Move setupFileSystemMonitoring and related helper functions
    - Keep initialization logic separate from request handling
    - _Requirements: 6.2, 6.3, 7.1_
  
  - [x] 5.3 Refactor server.go to core functionality
    - Keep MCPServer struct definition
    - Keep NewMCPServer constructor
    - Keep Start, Shutdown, and handleMessage functions
    - Keep coordination and lifecycle management
    - Remove moved functions, add imports for new files
    - _Requirements: 6.2, 6.3, 7.1_

- [x] 6. Update all test files with new paths
  - [x] 6.1 Update internal/server/server_test.go
    - Replace all "docs/guidelines/", "docs/patterns/", "docs/adr/" with mcp/resources/ paths
    - Update test fixture paths to use new directory structure
    - _Requirements: 3.1, 3.2_
  
  - [x] 6.2 Update internal/server/integration_test.go
    - Replace all docs/ path references with mcp/resources/ paths
    - Update test event paths to use new structure
    - _Requirements: 3.1, 3.2_
  
  - [x] 6.3 Update internal/server/server_benchmark_test.go
    - Replace all docs/ path references with mcp/resources/ paths
    - Update benchmark test data paths
    - _Requirements: 3.1_
  
  - [x] 6.4 Update internal/models/document_test.go
    - Replace all docs/ path references with mcp/resources/ paths
    - _Requirements: 3.1_
  
  - [x] 6.5 Update pkg/prompts tests
    - Verify prompt manager tests use correct paths
    - Update any hardcoded prompts/ references to mcp/prompts/
    - _Requirements: 3.1, 3.2_

- [ ] 7. Split large test files by functional area
  - [ ] 7.1 Create internal/server/server_lifecycle_test.go
    - Move Start, Shutdown, and initialization tests from server_test.go
    - Include graceful shutdown and signal handling tests
    - _Requirements: 7.3, 8.3_
  
  - [ ] 7.2 Create internal/server/server_handlers_test.go
    - Move protocol handler tests (initialize, resources, prompts) from server_test.go
    - Include error handling and validation tests for handlers
    - _Requirements: 7.3, 8.3_
  
  - [ ] 7.3 Create internal/server/server_resources_test.go
    - Move resource-specific tests (list, read, URI resolution) from server_test.go
    - Include cache interaction tests
    - _Requirements: 7.3, 8.3_
  
  - [ ] 7.4 Create internal/server/server_prompts_test.go
    - Move prompt-specific tests (list, get, rendering) from server_test.go
    - Include prompt validation and error tests
    - _Requirements: 7.3, 8.3_
  
  - [ ] 7.5 Remove original server_test.go
    - Verify all tests have been moved to new files
    - Delete original server_test.go file
    - Run full test suite to ensure nothing was missed
    - _Requirements: 7.3, 8.3_

- [ ] 8. Update Docker configuration
  - [ ] 8.1 Update Dockerfile
    - Replace separate COPY commands for docs/ and prompts/ with single COPY for mcp/
    - Update COPY command: `COPY --chown=mcpuser:mcpuser mcp/ /app/mcp/`
    - _Requirements: 4.1, 4.2_
  
  - [ ] 8.2 Update docker-compose.yml
    - Replace separate volume mounts with single mcp/ mount
    - Update volumes: `- ./mcp:/app/mcp:ro`
    - _Requirements: 4.3_
  
  - [ ] 8.3 Test Docker build and run
    - Build Docker image with new configuration
    - Run container and verify it can access resources and prompts
    - Test resource listing and prompt invocation in container
    - _Requirements: 4.4_

- [ ] 9. Update documentation files
  - [ ] 9.1 Update README.md
    - Update directory structure diagram to show mcp/ organization
    - Update example commands and paths
    - Update MCP configuration examples with new paths
    - _Requirements: 5.1_
  
  - [ ] 9.2 Update .kiro/steering/product.md
    - Update resource URI documentation to reference mcp/resources/
    - Update prompts directory references to mcp/prompts/
    - Update caching and monitoring documentation
    - _Requirements: 5.2_
  
  - [ ] 9.3 Update .kiro/steering/structure.md
    - Update directory structure diagram
    - Update package organization documentation
    - _Requirements: 5.2_
  
  - [ ] 9.4 Update SECURITY.md
    - Update path validation documentation to reference mcp/resources/
    - Update directory restriction references
    - _Requirements: 5.3_
  
  - [ ] 9.5 Update docs/prompts-guide.md
    - Update all prompts/ path references to mcp/prompts/
    - Update examples and instructions
    - _Requirements: 5.1_

- [ ] 10. Apply code quality improvements
  - [ ] 10.1 Run go fmt on all modified files
    - Run `go fmt ./...` to format all Go files
    - Verify formatting compliance
    - _Requirements: 6.1_
  
  - [ ] 10.2 Apply early return patterns
    - Review all handler functions for nested if-else
    - Refactor to use early returns for error cases
    - Reduce indentation levels in handler functions
    - _Requirements: 6.4, 6.5_
  
  - [ ] 10.3 Extract magic strings to constants
    - Identify remaining hardcoded strings in server package
    - Extract to constants in config.go or appropriate location
    - Update code to use constants
    - _Requirements: 6.6_
  
  - [ ] 10.4 Review and improve comments
    - Remove comments that explain WHAT code does (obvious from code)
    - Add comments that explain WHY decisions were made
    - Document non-obvious business logic
    - _Requirements: 6.7_
  
  - [ ] 10.5 Reduce nesting depth
    - Identify functions with nesting depth > 3
    - Refactor using early returns and helper functions
    - Improve readability and maintainability
    - _Requirements: 6.5_

- [ ] 11. Verify test coverage and run full test suite
  - Run `make test-coverage` to generate coverage report
  - Verify coverage is >70% for critical packages (server, cache, prompts, scanner)
  - Run full test suite and ensure 100% pass rate
  - Run benchmark tests to verify no performance regression
  - _Requirements: 3.4, 8.1, 8.4_

- [ ] 12. Final integration testing
  - Build and run server locally with new paths
  - Test resource listing and reading
  - Test prompt listing and invocation
  - Verify file system monitoring detects changes in new directories
  - Test Docker container end-to-end
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 4.4_
