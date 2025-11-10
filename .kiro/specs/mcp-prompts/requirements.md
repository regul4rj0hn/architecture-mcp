# Requirements Document

## Introduction

This document specifies the requirements for adding MCP Prompts capability to the MCP Architecture Service. Prompts are user-controlled, interactive templates that guide language model interactions by providing pre-defined instructions and context from architectural documentation. This feature will enable users to invoke structured workflows like "Review code against guidelines" or "Suggest patterns for a problem" through their IDE or AI assistant.

## Glossary

- **MCP Server**: The Model Context Protocol server implementation that provides architectural documentation access
- **Prompt**: A pre-defined template or instruction set that guides language model interactions, invoked by user choice
- **Prompt Template**: A structured format containing instructions, argument placeholders, and embedded resource references
- **Prompt Argument**: A user-provided input value that customizes a prompt's behavior (e.g., code snippet, problem description)
- **Resource**: Existing MCP primitive providing structured architectural documentation content
- **Client**: The IDE or AI assistant that communicates with the MCP Server via JSON-RPC over stdio
- **Prompt Registry**: The in-memory collection of available prompts maintained by the MCP Server

## Requirements

### Requirement 1

**User Story:** As a developer using an AI assistant, I want to discover available architectural prompts, so that I can understand what guided workflows are available to me.

#### Acceptance Criteria

1. WHEN the Client sends a "prompts/list" request, THE MCP Server SHALL return a list of all available prompts with their metadata
2. THE MCP Server SHALL include the prompt name, description, and required arguments in each prompt listing
3. THE MCP Server SHALL respond within 100 milliseconds for prompt discovery requests
4. THE MCP Server SHALL return prompts in a consistent order based on alphabetical sorting by name

### Requirement 2

**User Story:** As a developer, I want to invoke a prompt with my specific inputs, so that I can receive contextual guidance based on architectural documentation.

#### Acceptance Criteria

1. WHEN the Client sends a "prompts/get" request with a valid prompt name and required arguments, THE MCP Server SHALL return the rendered prompt content
2. THE MCP Server SHALL validate that all required arguments are provided before processing the prompt
3. IF a required argument is missing, THEN THE MCP Server SHALL return a validation error with code -32602
4. THE MCP Server SHALL substitute argument placeholders in the prompt template with the provided argument values
5. THE MCP Server SHALL embed relevant resource content into the prompt based on the prompt's resource references

### Requirement 3

**User Story:** As a developer, I want prompts to automatically include relevant architectural guidelines, so that I receive accurate guidance without manually searching for documentation.

#### Acceptance Criteria

1. WHEN a prompt references architectural resources, THE MCP Server SHALL retrieve the current content from the cache
2. THE MCP Server SHALL embed resource content at the designated locations within the prompt template
3. IF a referenced resource is not found in the cache, THEN THE MCP Server SHALL return an error indicating the missing resource
4. THE MCP Server SHALL support embedding multiple resources within a single prompt
5. THE MCP Server SHALL preserve markdown formatting when embedding resource content

### Requirement 4

**User Story:** As a system administrator, I want prompts to be defined in configuration files, so that I can customize and extend available prompts without code changes.

#### Acceptance Criteria

1. THE MCP Server SHALL load prompt definitions from JSON files in the "prompts/" directory during initialization
2. THE MCP Server SHALL validate prompt definition structure and report errors for malformed definitions
3. WHEN a prompt definition file is added or modified, THE MCP Server SHALL reload the prompt registry within 2 seconds
4. THE MCP Server SHALL support hot-reloading of prompt definitions without requiring server restart
5. IF a prompt definition contains syntax errors, THEN THE MCP Server SHALL log the error and exclude that prompt from the registry

### Requirement 5

**User Story:** As a developer, I want to use a prompt that reviews my code against architectural patterns, so that I can ensure my implementation follows established best practices.

#### Acceptance Criteria

1. THE MCP Server SHALL provide a "review-code-against-patterns" prompt that accepts a code snippet argument
2. WHEN invoked, THE MCP Server SHALL embed relevant design pattern documentation into the prompt
3. THE MCP Server SHALL include instructions for the language model to compare the code against documented patterns
4. THE MCP Server SHALL support code snippets up to 10,000 characters in length
5. THE MCP Server SHALL return the complete prompt within 500 milliseconds

### Requirement 6

**User Story:** As a developer, I want to use a prompt that suggests architectural patterns for a problem, so that I can discover relevant solutions from our documentation.

#### Acceptance Criteria

1. THE MCP Server SHALL provide a "suggest-patterns" prompt that accepts a problem description argument
2. WHEN invoked, THE MCP Server SHALL embed the complete patterns documentation catalog into the prompt
3. THE MCP Server SHALL include instructions for the language model to analyze the problem and recommend relevant patterns
4. THE MCP Server SHALL support problem descriptions up to 2,000 characters in length

### Requirement 7

**User Story:** As a developer, I want to use a prompt that helps me create a new ADR, so that I can document architectural decisions following our established format.

#### Acceptance Criteria

1. THE MCP Server SHALL provide a "create-adr" prompt that accepts a decision topic argument
2. WHEN invoked, THE MCP Server SHALL embed example ADRs from the documentation into the prompt
3. THE MCP Server SHALL include the ADR template structure in the prompt instructions
4. THE MCP Server SHALL include instructions for the language model to generate a draft ADR following the template

### Requirement 8

**User Story:** As a system operator, I want prompt invocations to be logged with performance metrics, so that I can monitor usage patterns and identify performance issues.

#### Acceptance Criteria

1. WHEN a prompt is invoked, THE MCP Server SHALL log the prompt name, arguments (sanitized), and execution duration
2. THE MCP Server SHALL record cache hit rates for resource retrievals during prompt processing
3. THE MCP Server SHALL log errors encountered during prompt processing with full context
4. THE MCP Server SHALL include prompt invocation metrics in the performance metrics endpoint

### Requirement 9

**User Story:** As a developer, I want prompt errors to provide clear guidance, so that I can quickly correct my inputs and retry.

#### Acceptance Criteria

1. IF a prompt name is not found, THEN THE MCP Server SHALL return error code -32602 with message "Prompt not found"
2. IF required arguments are missing, THEN THE MCP Server SHALL return error code -32602 listing the missing argument names
3. IF an argument value exceeds maximum length, THEN THE MCP Server SHALL return error code -32602 with the length limit
4. THE MCP Server SHALL include the prompt name in all error responses for context
5. THE MCP Server SHALL use structured error responses consistent with existing MCP error handling
