# Implementation Plan

- [x] 1. Create core tools infrastructure
  - Implement Tool interface and base types in `pkg/tools/definition.go`
  - Create ToolManager with registry and lifecycle management in `pkg/tools/manager.go`
  - Implement ToolExecutor with validation, timeout, and security in `pkg/tools/executor.go`
  - Add performance metrics tracking to ToolManager
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 2. Add MCP protocol support for tools
  - Define MCPTool, MCPToolsListResult, MCPToolsCallParams, MCPToolsCallResult types in `internal/models/tool.go`
  - Update MCPCapabilities to include Tools field in `internal/models/mcp.go`
  - Add MCPToolCapabilities struct with ListChanged field
  - _Requirements: 4.1, 4.2, 4.3_

- [x] 3. Implement tools protocol handlers
  - Add handleToolsList() method in `internal/server/handlers.go` to return available tools with schemas
  - Add handleToolsCall() method in `internal/server/handlers.go` to validate and execute tool invocations
  - Update handleMessage() switch statement to route tools/list and tools/call requests
  - Integrate with existing error handling patterns using createStructuredErrorResponse()
  - _Requirements: 4.1, 4.2, 4.3, 5.1, 5.2, 5.3, 5.4, 5.5_

- [x] 4. Initialize tools system in server
  - Add toolManager field to MCPServer struct in `internal/server/server.go`
  - Create initializeToolsSystem() method in `internal/server/initialization.go`
  - Call initializeToolsSystem() during server Start() sequence
  - Update server capabilities to include tools in handleInitialize()
  - Add tools metrics to handlePerformanceMetrics()
  - _Requirements: 7.1, 7.5_

- [x] 5. Implement ValidatePatternTool
- [x] 5.1 Create validate_pattern.go with ValidatePatternTool struct
  - Implement Tool interface methods (Name, Description, InputSchema, Execute)
  - Add constructor NewValidatePatternTool(cache *cache.DocumentCache)
  - Define input schema with code, pattern_name, and language fields
  - Implement pattern loading from cache
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 5.2 Implement pattern validation logic
  - Parse pattern document for validation rules
  - Analyze code structure against pattern expectations
  - Generate violation reports with severity levels
  - Create actionable suggestions for improvements
  - Return structured validation results
  - _Requirements: 1.1, 1.2_

- [x] 5.3 Write unit tests for ValidatePatternTool
  - Test pattern validation with compliant code
  - Test violation detection with non-compliant code
  - Test error handling for missing patterns
  - Test input validation and size limits
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 6. Implement SearchArchitectureTool
- [x] 6.1 Create search_architecture.go with SearchArchitectureTool struct
  - Implement Tool interface methods
  - Add constructor NewSearchArchitectureTool(cache *cache.DocumentCache)
  - Define input schema with query, resource_type, and max_results fields
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

- [x] 6.2 Implement search and ranking logic
  - Tokenize query and document content
  - Calculate relevance scores using keyword matching
  - Filter results by resource type
  - Extract relevant excerpts from matching documents
  - Sort and limit results
  - Return structured search results with URIs, titles, and excerpts
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

- [x] 6.3 Write unit tests for SearchArchitectureTool
  - Test search with various queries
  - Test resource type filtering
  - Test result limiting
  - Test relevance scoring
  - Test excerpt extraction
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

- [ ] 7. Implement CheckADRAlignmentTool
- [ ] 7.1 Create check_adr_alignment.go with CheckADRAlignmentTool struct
  - Implement Tool interface methods
  - Add constructor NewCheckADRAlignmentTool(cache *cache.DocumentCache)
  - Define input schema with decision_description and decision_context fields
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [ ] 7.2 Implement ADR alignment analysis
  - Extract keywords from decision description
  - Search ADR documents for related content
  - Analyze ADR status and decision text
  - Classify alignment as supports/conflicts/related
  - Identify potential conflicts with existing decisions
  - Generate suggestions for ADR references
  - Return structured alignment analysis
  - _Requirements: 3.1, 3.2, 3.3, 3.5_

- [ ]* 7.3 Write unit tests for CheckADRAlignmentTool
  - Test alignment detection with supporting ADRs
  - Test conflict identification
  - Test related ADR discovery
  - Test keyword extraction
  - Test input validation
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [ ] 8. Register tools in server initialization
  - Update initializeToolsSystem() to register ValidatePatternTool
  - Register SearchArchitectureTool
  - Register CheckADRAlignmentTool
  - Add error handling and logging for tool registration
  - Validate all tools on startup
  - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [ ] 9. Add security and validation
  - Implement path validation in ToolExecutor to prevent directory traversal
  - Enforce argument size limits (50KB for code, 500 chars for queries, 5KB for descriptions)
  - Add execution timeout enforcement (10 seconds default)
  - Implement argument sanitization for logging
  - Add circuit breaker integration for tool execution
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 5.4_

- [ ] 10. Implement prompt-tool integration
- [ ] 10.1 Add tool reference expansion to TemplateRenderer
  - Extend EmbedResources() or create EmbedTools() method in `pkg/prompts/renderer.go`
  - Parse {{tool:tool-name}} syntax in prompt templates
  - Look up tool from ToolManager
  - Expand reference to include tool description and schema
  - Validate tool references on prompt load
  - _Requirements: 8.1, 8.2, 8.3, 8.5_

- [ ] 10.2 Update PromptManager to support tool references
  - Inject ToolManager dependency into PromptManager
  - Pass ToolManager to TemplateRenderer
  - Validate tool references during prompt loading
  - Log warnings for references to non-existent tools
  - _Requirements: 8.5_

- [ ] 10.3 Create example prompt with tool integration
  - Create `mcp/prompts/guided-pattern-validation.json` demonstrating tool references
  - Include {{tool:validate-against-pattern}} reference
  - Add workflow instructions for using the tool
  - Test prompt rendering with tool expansion
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [ ]* 10.4 Implement workflow context management
  - Create WorkflowContext struct in `pkg/tools/executor.go`
  - Add ExecuteWithContext() method to ToolExecutor
  - Allow tools to access prompt arguments and previous results
  - Implement session-based context storage
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5_

- [ ]* 11. Write integration tests
  - Test full tool invocation flow (initialize → tools/list → tools/call) in `internal/server/integration_test.go`
  - Test tool execution with real cache data
  - Test error handling across protocol layers
  - Test prompt-tool integration scenarios
  - Test concurrent tool invocations
  - _Requirements: 4.1, 4.2, 4.3, 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ]* 12. Write unit tests for core infrastructure
  - Test ToolManager registration and lookup in `pkg/tools/manager_test.go`
  - Test ToolExecutor validation and execution in `pkg/tools/executor_test.go`
  - Test protocol handlers in `internal/server/handlers_test.go`
  - Test performance metrics tracking
  - Test error handling and conversion
  - _Requirements: 4.4, 4.5, 5.5, 7.1, 7.3, 7.4_

- [ ] 13. Update documentation
  - Add tools section to README.md explaining available tools
  - Document tool invocation examples
  - Update architecture diagrams to include tools
  - Add tool development guide for creating new tools
  - Document prompt-tool integration patterns
  - _Requirements: 7.1, 7.2, 8.1, 8.2_
