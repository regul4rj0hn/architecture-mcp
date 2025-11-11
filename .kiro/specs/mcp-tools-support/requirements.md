# Requirements Document

## Introduction

This feature adds MCP tools support to the architecture MCP server, enabling AI models to perform executable actions beyond reading documentation. Tools will allow models to validate architectural decisions, search patterns, analyze code against guidelines, and perform other actionable operations that complement the existing resources and prompts.

## Glossary

- **MCP Server**: The Model Context Protocol server that exposes architectural documentation and interactive capabilities
- **Tool**: An executable function exposed via MCP that models can invoke to perform actions or retrieve computed information
- **Tool Definition**: JSON schema describing a tool's name, description, and input parameters
- **Tool Invocation**: A request from a model to execute a specific tool with provided arguments
- **Architecture Assets**: The collection of guidelines, patterns, and ADRs stored in `mcp/resources/`
- **Validation Tool**: A tool that checks code or decisions against architectural standards
- **Search Tool**: A tool that queries and filters architectural documentation
- **Analysis Tool**: A tool that performs computation or reasoning over architectural assets
- **Prompt-Tool Integration**: The coordination between prompts (which guide workflows) and tools (which execute actions) to enable complete interactive experiences

## Requirements

### Requirement 1

**User Story:** As an AI model, I want to validate code against architectural patterns, so that I can provide accurate feedback on whether implementations follow established guidelines

#### Acceptance Criteria

1. WHEN a model invokes the validate-against-pattern tool with code and pattern name, THE MCP Server SHALL return a validation result indicating compliance status
2. THE MCP Server SHALL include specific violations or deviations from the pattern in the validation result
3. IF the specified pattern does not exist, THEN THE MCP Server SHALL return an error with code -32602 indicating invalid parameters
4. THE MCP Server SHALL limit code input to 50,000 characters to prevent resource exhaustion
5. THE MCP Server SHALL complete validation within 5 seconds or return a timeout error

### Requirement 2

**User Story:** As an AI model, I want to search architectural documentation by keywords or tags, so that I can quickly find relevant patterns and guidelines without reading all resources

#### Acceptance Criteria

1. WHEN a model invokes the search-architecture tool with a query string, THE MCP Server SHALL return matching resources ranked by relevance
2. THE MCP Server SHALL support filtering by resource type (guidelines, patterns, adr)
3. THE MCP Server SHALL return resource URIs, titles, and matching excerpts for each result
4. THE MCP Server SHALL limit search results to 20 items maximum
5. WHILE processing a search query, THE MCP Server SHALL use cached documentation to ensure response time under 2 seconds

### Requirement 3

**User Story:** As an AI model, I want to check if an architectural decision aligns with existing ADRs, so that I can identify conflicts or redundancies before creating new decisions

#### Acceptance Criteria

1. WHEN a model invokes the check-adr-alignment tool with a decision description, THE MCP Server SHALL return related ADRs and their alignment status
2. THE MCP Server SHALL identify potential conflicts with existing decisions
3. THE MCP Server SHALL suggest related ADRs that should be referenced or superseded
4. THE MCP Server SHALL limit decision description input to 5,000 characters
5. THE MCP Server SHALL return alignment results within 3 seconds

### Requirement 4

**User Story:** As an AI model, I want to list available tools with their schemas, so that I can understand what actions I can perform

#### Acceptance Criteria

1. WHEN a model sends a tools/list request, THE MCP Server SHALL return all available tool definitions
2. THE MCP Server SHALL include name, description, and JSON schema for input parameters in each tool definition
3. THE MCP Server SHALL follow MCP protocol specification for tool listing format
4. THE MCP Server SHALL support hot-reloading of tool definitions when configuration changes
5. THE MCP Server SHALL validate tool definitions on startup and reject invalid schemas

### Requirement 5

**User Story:** As an AI model, I want to invoke tools with validated parameters, so that I can execute actions safely and receive structured results

#### Acceptance Criteria

1. WHEN a model sends a tools/call request with tool name and arguments, THE MCP Server SHALL validate arguments against the tool schema
2. IF arguments are invalid or missing required fields, THEN THE MCP Server SHALL return error code -32602 with validation details
3. THE MCP Server SHALL execute the tool and return results in MCP-compliant format
4. THE MCP Server SHALL handle tool execution errors gracefully and return error code -32603 for internal failures
5. THE MCP Server SHALL log all tool invocations with arguments and results for debugging

### Requirement 6

**User Story:** As a system administrator, I want tools to respect security constraints, so that models cannot access unauthorized resources or perform dangerous operations

#### Acceptance Criteria

1. THE MCP Server SHALL validate all file paths in tool arguments to prevent directory traversal
2. THE MCP Server SHALL restrict tool file access to the `mcp/resources/` directory only
3. THE MCP Server SHALL enforce argument size limits to prevent denial of service
4. THE MCP Server SHALL run all tool operations with the same non-root user (UID 1001) as the server
5. THE MCP Server SHALL timeout tool executions after 10 seconds maximum to prevent resource exhaustion

### Requirement 7

**User Story:** As a developer, I want to add new tools easily, so that I can extend the server's capabilities without modifying core protocol handling

#### Acceptance Criteria

1. THE MCP Server SHALL load tool definitions from a registry pattern that allows registration of new tools
2. THE MCP Server SHALL support defining tools in separate files under `pkg/tools/` directory
3. THE MCP Server SHALL automatically discover and register tools that implement the Tool interface
4. THE MCP Server SHALL validate tool implementations on startup and log warnings for invalid tools
5. WHERE a tool definition includes dependencies on other packages, THE MCP Server SHALL initialize those dependencies during tool registration

### Requirement 8

**User Story:** As an AI model, I want prompts to reference and suggest relevant tools, so that I understand which tools to invoke during guided workflows

#### Acceptance Criteria

1. WHEN a model retrieves a prompt that involves executable actions, THE MCP Server SHALL include tool suggestions in the prompt response
2. THE MCP Server SHALL support embedding tool references in prompt templates using syntax like `{{tool:tool-name}}`
3. THE MCP Server SHALL expand tool references to include the tool's name, description, and parameter schema
4. WHERE a prompt workflow requires multiple steps, THE MCP Server SHALL indicate which tools are appropriate for each step
5. THE MCP Server SHALL validate tool references in prompts on startup and log warnings for references to non-existent tools

### Requirement 9

**User Story:** As an AI model, I want to use tools within prompt-guided workflows, so that I can complete complex architectural tasks with guidance and validation

#### Acceptance Criteria

1. WHEN a model follows a prompt workflow that suggests a tool, THE MCP Server SHALL accept tool invocations that reference the prompt context
2. THE MCP Server SHALL allow tools to access prompt argument values when invoked within a prompt workflow
3. THE MCP Server SHALL maintain workflow state across multiple tool invocations within the same session
4. THE MCP Server SHALL provide tool results that can be referenced in subsequent prompt steps
5. WHERE a tool execution fails during a prompt workflow, THE MCP Server SHALL return actionable error messages that help the model recover or retry
