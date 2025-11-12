# Requirements Document

## Introduction

This feature addresses two critical issues in the MCP Architecture Service:

1. **Missing Completion Endpoint**: Adds support for the `completion/complete` endpoint to enable autocomplete suggestions for prompt arguments, improving the user experience when interacting with prompts.

2. **TCP Connection Error Handling**: Fixes the "Accept error: accept tcp 127.0.0.1:8080: use of closed network connection" error that occurs during mcp-bridge shutdown by implementing proper graceful shutdown handling.

## Glossary

- **MCP Server**: The Model Context Protocol server that exposes architectural documentation and prompts via stdio
- **MCP Bridge**: The TCP-to-stdio bridge service that allows network clients to connect to the MCP Server
- **Completion Endpoint**: The `completion/complete` JSON-RPC method that provides autocomplete suggestions
- **Prompt Argument**: A named parameter that a prompt accepts (e.g., `pattern_name`, `code`)
- **Completion Context**: The current state of argument values when requesting completions
- **Completion Item**: A single autocomplete suggestion with a value and optional label/description
- **Graceful Shutdown**: The process of cleanly terminating a service without generating error messages

## Requirements

### Requirement 1

**User Story:** As an MCP client, I want to request autocomplete suggestions for prompt arguments, so that I can provide valid values without knowing all options in advance

#### Acceptance Criteria

1. WHEN the MCP Server receives a `completion/complete` request with valid parameters, THE MCP Server SHALL return a list of completion suggestions
2. WHEN the `completion/complete` request includes a prompt reference, THE MCP Server SHALL validate that the prompt exists
3. WHEN the `completion/complete` request includes an argument name, THE MCP Server SHALL validate that the argument exists for the specified prompt
4. IF the prompt does not exist, THEN THE MCP Server SHALL return a JSON-RPC error with code -32602 and message "Prompt not found"
5. IF the argument does not exist for the prompt, THEN THE MCP Server SHALL return a JSON-RPC error with code -32602 and message "Invalid argument name"

### Requirement 2

**User Story:** As an MCP client, I want to receive contextual completion suggestions based on the argument type, so that suggestions are relevant to what I'm trying to complete

#### Acceptance Criteria

1. WHEN requesting completions for a `pattern_name` argument, THE MCP Server SHALL return a list of available pattern names from the resources
2. WHEN requesting completions for a `guideline_name` argument, THE MCP Server SHALL return a list of available guideline names from the resources
3. WHEN requesting completions for an `adr_id` argument, THE MCP Server SHALL return a list of available ADR identifiers from the resources
4. WHEN requesting completions for arguments without predefined values, THE MCP Server SHALL return an empty completion list
5. THE MCP Server SHALL extract completion values from the document cache without performing additional filesystem scans
6. WHEN the argument value is provided, THE MCP Server SHALL filter completion suggestions to match the prefix of the current value
7. THE MCP Server SHALL perform case-insensitive prefix matching when filtering completions

### Requirement 3

**User Story:** As an MCP client, I want completion suggestions to include helpful metadata, so that I can understand what each suggestion represents

#### Acceptance Criteria

1. THE MCP Server SHALL return each completion item with a `value` field containing the completion text
2. THE MCP Server SHALL include an optional `label` field when the display text differs from the value
3. THE MCP Server SHALL include an optional `description` field with additional context about the completion
4. WHEN completing pattern names, THE MCP Server SHALL include the pattern title as the description
5. WHEN completing ADR identifiers, THE MCP Server SHALL include the ADR title as the description

### Requirement 4

**User Story:** As a developer, I want the completion endpoint to follow MCP protocol standards, so that it integrates seamlessly with MCP clients

#### Acceptance Criteria

1. THE MCP Server SHALL implement the `completion/complete` method according to MCP specification version 2024-11-05
2. THE MCP Server SHALL accept a `ref` parameter containing the prompt reference with `type` field set to "ref/prompt" and `name` field containing the prompt name
3. THE MCP Server SHALL accept an `argument` parameter containing the argument `name` and current `value`
4. THE MCP Server SHALL accept an optional `context` parameter containing the current state of all argument values
5. THE MCP Server SHALL return a `completion` object with a `values` array containing completion items
6. THE MCP Server SHALL return completion items with `value` as a required field and `label`/`description` as optional fields
7. WHEN the `ref.type` field is not "ref/prompt", THE MCP Server SHALL return error code -32602 with message "Invalid reference type"

### Requirement 5

**User Story:** As a developer, I want the completion system to handle errors gracefully, so that invalid requests don't crash the server

#### Acceptance Criteria

1. WHEN the `completion/complete` request is missing required parameters, THE MCP Server SHALL return error code -32602 with message "Invalid parameters"
2. WHEN the completion system encounters an internal error, THE MCP Server SHALL return error code -32603 with message "Internal error"
3. THE MCP Server SHALL log all completion errors with structured context including prompt name and argument name
4. THE MCP Server SHALL continue serving other requests even if completion requests fail
5. THE MCP Server SHALL use circuit breaker protection for completion operations to prevent cascading failures

### Requirement 6

**User Story:** As a developer, I want completion capabilities to be advertised during initialization, so that clients know the server supports completions

#### Acceptance Criteria

1. THE MCP Server SHALL include a `completion` capability in the initialization response
2. THE MCP Server SHALL set the completion capability to indicate support for argument completions
3. WHEN the MCP Server does not support completions for a specific prompt, THE MCP Server SHALL still accept completion requests and return an empty list
4. THE MCP Server SHALL document completion support in the server capabilities structure

### Requirement 7

**User Story:** As a system operator, I want the MCP Bridge to shut down gracefully without generating error messages, so that logs remain clean and monitoring systems don't trigger false alerts

#### Acceptance Criteria

1. WHEN the MCP Bridge receives a shutdown signal, THE MCP Bridge SHALL stop accepting new connections before closing the listener
2. WHEN the listener is closed during shutdown, THE MCP Bridge SHALL not log "Accept error" messages for expected connection closure
3. THE MCP Bridge SHALL use a context cancellation mechanism to signal the Accept loop to exit cleanly
4. WHEN the Accept call returns an error during shutdown, THE MCP Bridge SHALL check if the context is cancelled before logging the error
5. THE MCP Bridge SHALL complete shutdown within 5 seconds of receiving the shutdown signal

### Requirement 8

**User Story:** As a developer, I want the steering rules to accurately reflect both MCP Server and MCP Bridge architecture, so that future development follows the correct patterns

#### Acceptance Criteria

1. THE steering rules SHALL document that the MCP Server communicates via stdio only
2. THE steering rules SHALL document that the MCP Bridge provides TCP-to-stdio bridging on port 8080
3. THE steering rules SHALL clarify that network connections are handled by the MCP Bridge, not the MCP Server
4. THE steering rules SHALL document the relationship between MCP Server and MCP Bridge components
5. THE steering rules SHALL include guidance on when to modify each component
6. THE steering rules SHALL document the completion endpoint and ref/prompt reference type

### Requirement 9

**User Story:** As a system operator, I want to control the logging verbosity of the MCP Server, so that I can adjust log output based on debugging needs without recompiling

#### Acceptance Criteria

1. THE MCP Server SHALL accept a command-line flag `--log-level` to set the logging verbosity
2. THE MCP Server SHALL support the following log levels: DEBUG, INFO, WARN, ERROR
3. WHEN no log level is specified, THE MCP Server SHALL default to INFO level
4. THE MCP Server SHALL validate the log level parameter and return an error for invalid values
5. THE MCP Server SHALL apply the log level to all logger instances created by the logging manager
6. WHEN the log level is set to DEBUG, THE MCP Server SHALL log all MCP request/response messages with full details
7. WHEN the log level is set to ERROR, THE MCP Server SHALL only log error conditions and critical failures
