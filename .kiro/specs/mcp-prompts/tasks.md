# Implementation Plan

- [x] 1. Create prompt data models and protocol structures
  - Create `internal/models/prompt.go` with MCP prompt protocol structures (MCPPrompt, MCPPromptArgument, MCPPromptsListResult, MCPPromptsGetParams, MCPPromptMessage, MCPPromptContent, MCPPromptsGetResult)
  - Add MCPPromptCapabilities struct to support prompts capability declaration
  - _Requirements: 1.1, 1.2_

- [x] 2. Implement prompt definition and validation
  - [x] 2.1 Create prompt definition structures
    - Create `pkg/prompts/definition.go` with PromptDefinition, ArgumentDefinition, MessageTemplate, and ContentTemplate structs
    - Implement JSON unmarshaling for loading prompt definitions from files
    - _Requirements: 4.1, 4.2_
  
  - [x] 2.2 Implement prompt validation logic
    - Implement Validate() method on PromptDefinition to check structure integrity
    - Implement ValidateArguments() method to validate user-provided arguments against definition
    - Add validation for prompt name pattern (^[a-z0-9-]+$), required arguments, and max length constraints
    - _Requirements: 2.2, 2.3, 4.2, 9.2, 9.3_
  
  - [x] 2.3 Write unit tests for prompt definition
    - Create `pkg/prompts/definition_test.go` with tests for valid/invalid definitions and argument validation
    - _Requirements: 2.2, 2.3_

- [x] 3. Implement template rendering engine
  - [x] 3.1 Create template renderer
    - Create `pkg/prompts/renderer.go` with TemplateRenderer struct
    - Implement RenderTemplate() method for {{variable}} substitution
    - Implement EmbedResources() method for {{resource:uri}} pattern processing
    - Implement ResolveResourcePattern() method to match resource URIs and retrieve content from cache
    - _Requirements: 2.4, 2.5, 3.1, 3.2, 3.4, 3.5_
  
  - [x] 3.2 Add resource embedding logic
    - Implement wildcard pattern matching for resource URIs (e.g., architecture://patterns/*)
    - Implement resource content retrieval from DocumentCache
    - Add error handling for missing resources with clear error messages
    - Implement size limits (50 resources max, 1MB total content max)
    - _Requirements: 3.1, 3.2, 3.3, 3.4_
  
  - [x] 3.3 Write unit tests for template rendering
    - Create `pkg/prompts/renderer_test.go` with tests for variable substitution, resource embedding, and edge cases
    - _Requirements: 2.4, 2.5, 3.1, 3.2_

- [x] 4. Implement prompt manager
  - [x] 4.1 Create prompt manager structure
    - Create `pkg/prompts/manager.go` with PromptManager struct containing registry, cache reference, and mutex
    - Implement NewPromptManager() constructor
    - _Requirements: 4.1_
  
  - [x] 4.2 Implement prompt loading and registry management
    - Implement LoadPrompts() method to scan prompts/ directory and load JSON files
    - Implement GetPrompt() method to retrieve prompt definition by name
    - Implement ListPrompts() method to return all available prompts sorted alphabetically
    - Add error handling for malformed prompt files (log and skip)
    - _Requirements: 1.1, 1.4, 4.1, 4.2, 4.5_
  
  - [x] 4.3 Implement prompt rendering orchestration
    - Implement RenderPrompt() method that validates arguments, renders templates, and embeds resources
    - Integrate TemplateRenderer for template processing
    - Return MCPPromptsGetResult with rendered messages
    - _Requirements: 2.1, 2.4, 2.5, 3.1, 3.2_
  
  - [x] 4.4 Implement hot-reload functionality
    - Implement ReloadPrompts() method to refresh prompt registry
    - Add file system event handler for prompts directory changes
    - Integrate with existing FileSystemMonitor to watch prompts/ directory
    - Implement debouncing to batch rapid file changes (500ms delay)
    - _Requirements: 4.3, 4.4_
  
  - [x] 4.5 Write unit tests for prompt manager
    - Create `pkg/prompts/manager_test.go` with tests for loading, registry management, hot reload, and concurrent access
    - _Requirements: 4.1, 4.3, 4.4_

- [ ] 5. Integrate prompts into MCP server
  - [ ] 5.1 Add prompt manager to server
    - Update MCPServer struct in `internal/server/server.go` to include promptManager field
    - Initialize PromptManager in NewMCPServer() constructor
    - Load prompts during server initialization in Start() method
    - Set up prompts directory monitoring for hot-reload
    - _Requirements: 4.1, 4.3_
  
  - [ ] 5.2 Implement prompts/list handler
    - Create handlePromptsList() method in `internal/server/server.go`
    - Call promptManager.ListPrompts() and return MCPPromptsListResult
    - Add "prompts/list" case to handleMessage() switch statement
    - Ensure response time under 100ms
    - _Requirements: 1.1, 1.2, 1.3, 1.4_
  
  - [ ] 5.3 Implement prompts/get handler
    - Create handlePromptsGet() method in `internal/server/server.go`
    - Parse MCPPromptsGetParams from request
    - Call promptManager.RenderPrompt() with name and arguments
    - Return MCPPromptsGetResult with rendered prompt
    - Add "prompts/get" case to handleMessage() switch statement
    - _Requirements: 2.1, 2.4, 2.5, 5.5_
  
  - [ ] 5.4 Add error handling for prompt operations
    - Implement error responses for prompt not found (code -32602)
    - Implement error responses for missing required arguments (code -32602)
    - Implement error responses for argument too long (code -32602)
    - Implement error responses for missing resources during render
    - Use existing createStructuredErrorResponse() pattern
    - _Requirements: 2.3, 3.3, 9.1, 9.2, 9.3, 9.4, 9.5_
  
  - [ ] 5.5 Update server capabilities
    - Add Prompts field to MCPCapabilities struct in `internal/models/mcp.go`
    - Update handleInitialize() to include prompts capability in initialization response
    - _Requirements: 1.1_
  
  - [ ]* 5.6 Write integration tests for server handlers
    - Update `internal/server/server_test.go` with tests for prompts/list and prompts/get handlers
    - _Requirements: 1.1, 2.1_

- [ ] 6. Create built-in prompt definitions
  - [ ] 6.1 Create prompts directory structure
    - Create `prompts/` directory in project root
    - _Requirements: 4.1_
  
  - [ ] 6.2 Create review-code-against-patterns prompt
    - Create `prompts/review-code-against-patterns.json` with code and language arguments
    - Include resource embedding for architecture://patterns/* in template
    - Add instructions for comparing code against documented patterns
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_
  
  - [ ] 6.3 Create suggest-patterns prompt
    - Create `prompts/suggest-patterns.json` with problem description argument
    - Include resource embedding for complete patterns catalog
    - Add instructions for analyzing problem and recommending patterns
    - _Requirements: 6.1, 6.2, 6.3, 6.4_
  
  - [ ] 6.4 Create create-adr prompt
    - Create `prompts/create-adr.json` with decision topic argument
    - Include resource embedding for example ADRs
    - Add ADR template structure and generation instructions
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [ ] 7. Add logging and monitoring
  - [ ] 7.1 Implement prompt invocation logging
    - Add logging for prompt invocations with name, sanitized arguments, and execution duration
    - Use existing StructuredLogger pattern
    - Log errors with full context including prompt name
    - _Requirements: 8.1, 8.3, 9.4_
  
  - [ ] 7.2 Add performance metrics
    - Extend handlePerformanceMetrics() to include prompt statistics
    - Track total prompts loaded, invocation count by name, average render time
    - Track resource embedding cache hit rate for prompt operations
    - Track failed prompt invocations
    - _Requirements: 8.2, 8.4_

- [ ] 8. Update Docker and Kubernetes configurations
  - [ ] 8.1 Update Dockerfile
    - Add COPY instruction for prompts/ directory to Docker image
    - Add VOLUME declaration for prompts directory to support customization
    - _Requirements: 4.1_
  
  - [ ] 8.2 Update docker-compose.yml
    - Add volume mount for prompts directory
    - _Requirements: 4.1_
  
  - [ ] 8.3 Update Kubernetes deployment
    - Add ConfigMap definition for prompt definitions in `k8s-deployment.yaml`
    - Add volume mount for prompts ConfigMap
    - _Requirements: 4.1_

- [ ] 9. Update documentation
  - [ ] 9.1 Update README.md
    - Add section describing prompts capability
    - Document available built-in prompts
    - Provide examples of prompt invocation
    - _Requirements: 1.1, 2.1_
  
  - [ ] 9.2 Create prompt definition guide
    - Create `docs/prompts-guide.md` with prompt definition format documentation
    - Include JSON schema reference
    - Provide examples of custom prompt creation
    - Document template syntax ({{variable}}, {{resource:uri}})
    - _Requirements: 4.1, 4.2_
  
  - [ ] 9.3 Update SECURITY.md
    - Document input validation for prompt arguments
    - Document resource access restrictions
    - Document size limits and DoS prevention measures
    - _Requirements: 2.2, 2.3_
