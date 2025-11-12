# Implementation Plan

- [ ] 1. Update MCP Server entry point logging
  - Replace standard library `log` package with structured logging in `cmd/mcp-server/main.go`
  - Initialize LoggingManager early in main() with global context
  - Configure log level from command-line flag
  - Replace all log.Printf/log.Println calls with structured logger calls
  - Add appropriate context fields to each log message
  - Use correct log levels (INFO for normal operations, ERROR for failures)
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3, 3.2, 4.1, 4.3, 4.5_

- [ ] 2. Update MCP Bridge entry point logging
  - Replace standard library `log` package with structured logging in `cmd/mcp-bridge/main.go`
  - Add logger field to MCPBridge struct
  - Add logger field to MCPSession struct
  - Initialize LoggingManager early in main() with global context
  - Pass logger to MCPBridge and MCPSession instances
  - Replace all log.Printf/log.Println calls with structured logger calls
  - Add session_id and remote_addr context to session logs
  - Use correct log levels throughout (INFO for normal, WARN for degraded, ERROR for failures)
  - _Requirements: 1.1, 1.2, 1.3, 1.5, 2.1, 2.2, 2.3, 2.4, 3.2, 3.3, 3.5, 4.2, 4.3, 4.4, 4.5_

- [ ] 3. Update Server Core message processing logging
  - Replace log.Printf calls in internal/server/server.go processMessages method
  - Use existing s.logger instance with appropriate context
  - Add method and error details to log context
  - Use ERROR level for message decoding failures
  - _Requirements: 1.1, 1.2, 1.3, 2.3, 3.1, 3.3_

- [ ] 4. Verify logging cleanup completeness
  - Search entire codebase for any remaining `log` package imports
  - Search entire codebase for any remaining log.Print* calls
  - Verify all logging goes through pkg/logging
  - Run manual tests to verify log output format and content
  - Verify log levels are correctly applied
  - Verify context fields are present in all log messages
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 2.5, 3.1, 3.2, 3.3, 3.4, 3.5_
