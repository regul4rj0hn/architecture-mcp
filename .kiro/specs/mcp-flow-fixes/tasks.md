# Implementation Plan

- [ ] 1. Add completion data models to MCP protocol types
  - Create MCPCompletionCompleteParams, MCPCompletionRef, MCPCompletionArgument structs in internal/models/mcp.go
  - Create MCPCompletionItem, MCPCompletionResult, MCPCompletion structs for response format
  - Add MCPCompletionCapabilities struct and integrate into MCPCapabilities
  - _Requirements: 1.1, 1.2, 4.2, 4.3, 4.4, 4.5, 4.6_

- [ ] 2. Implement completion endpoint handler
  - [ ] 2.1 Add handleCompletionComplete method to internal/server/handlers.go
    - Parse and validate completion request parameters
    - Validate ref.type is "ref/prompt"
    - Validate prompt exists using promptManager
    - Call generateCompletions with circuit breaker protection
    - Format and return MCPCompletionResult
    - _Requirements: 1.1, 1.3, 1.4, 1.5, 4.1, 4.2, 4.3, 4.4, 5.5_
  
  - [ ] 2.2 Implement completion generation logic
    - Add generateCompletions method that routes by argument name
    - Implement generatePatternCompletions for pattern_name arguments
    - Implement generateGuidelineCompletions for guideline_name arguments
    - Implement generateADRCompletions for adr_id arguments
    - Return empty list for unknown argument types
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  
  - [ ] 2.3 Add prefix filtering and metadata extraction
    - Implement case-insensitive prefix matching in completion generators
    - Extract document titles for completion descriptions
    - Format completion items with value, label, and description fields
    - _Requirements: 2.6, 2.7, 3.1, 3.2, 3.3, 3.4, 3.5_
  
  - [ ] 2.4 Add completion error handling
    - Implement validateCompletionParams helper method
    - Implement handleCompletionError for structured error responses
    - Add logging for completion errors with context
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [ ] 3. Wire completion endpoint into server message routing
  - Add "completion/complete" case to handleMessage switch in internal/server/server.go
  - Update server capabilities to include Completion capability in NewMCPServer
  - Set ArgumentCompletions to true in capability initialization
  - _Requirements: 4.1, 6.1, 6.2, 6.3, 6.4_

- [x] 4. Fix MCP Bridge graceful shutdown
  - Add shutdownFlag atomic.Bool field to MCPBridge struct in cmd/mcp-bridge/main.go
  - Update Shutdown method to set flag before closing listener
  - Update Start method Accept loop to check shutdown flag before logging errors
  - Return nil instead of logging when shutdown flag is set
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [ ] 5. Add logging level configuration
  - [ ] 5.1 Add log level support to logging manager
    - Add LogLevel type and constants (DEBUG, INFO, WARN, ERROR) to pkg/logging/manager.go
    - Add logLevel field to LoggingManager struct
    - Implement SetLogLevel method with string-to-enum conversion
    - Implement shouldLog method for level filtering
    - _Requirements: 9.2, 9.5_
  
  - [ ] 5.2 Update logger to respect log level
    - Modify Debug, Info, Warn, Error methods in pkg/logging/logger.go to check shouldLog
    - Return early if log level is below threshold
    - _Requirements: 9.5, 9.6, 9.7_
  
  - [ ] 5.3 Add command-line flag parsing
    - Add flag import and --log-level flag to cmd/mcp-server/main.go
    - Validate log level against allowed values (DEBUG, INFO, WARN, ERROR)
    - Exit with error message for invalid log levels
    - Default to INFO when flag not specified
    - _Requirements: 9.1, 9.3, 9.4_
  
  - [ ] 5.4 Create server constructor with log level
    - Add NewMCPServerWithLogLevel function to internal/server/server.go
    - Call SetLogLevel on loggingManager during initialization
    - Update main.go to use new constructor with parsed log level
    - _Requirements: 9.5_

- [ ]* 6. Add unit tests for completion endpoint
  - Write test for valid pattern_name completion with prefix filtering
  - Write test for invalid ref type error handling
  - Write test for prompt not found error handling
  - Write test for empty prefix returning all completions
  - Write test for unknown argument returning empty list
  - Write test for guideline_name and adr_id completions
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 2.6, 2.7_

- [ ]* 7. Add unit tests for bridge shutdown
  - Write test for graceful shutdown without error logs
  - Write test for shutdown with active connections
  - Write test for shutdown flag preventing error logging
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [ ]* 8. Add unit tests for logging configuration
  - Write test for SetLogLevel with valid levels
  - Write test for shouldLog filtering at different levels
  - Write test for logger methods respecting log level
  - Write test for invalid log level handling in main
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7_
