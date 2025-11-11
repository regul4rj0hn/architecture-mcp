# Requirements Document

## Introduction

This feature addresses the reorganization of MCP assets from `/docs` to `/mcp` to separate actual documentation from MCP resources. The system currently has hardcoded references to `docs/` and `prompts/` directories that need to be updated to `mcp/resources/` and `mcp/prompts/` respectively. This change affects file scanning, caching, monitoring, Docker volume mounting, and tests.

## Glossary

- **MCP Server**: The Model Context Protocol server that exposes architectural documentation as resources and prompts
- **Resource Directory**: The directory containing architectural documentation (guidelines, patterns, ADRs)
- **Prompts Directory**: The directory containing JSON prompt definition files
- **Document Scanner**: Component that scans and parses markdown files from the resource directory
- **File System Monitor**: Component that watches directories for file changes
- **Document Cache**: In-memory cache of parsed documentation
- **Prompt Manager**: Component that loads and manages prompt definitions

## Requirements

### Requirement 1

**User Story:** As a developer, I want the MCP server to read resources from `/mcp/resources/` instead of `/docs/`, so that I can use `/docs/` for actual project documentation

#### Acceptance Criteria

1. WHEN THE MCP Server initializes, THE Document Scanner SHALL scan directories under `mcp/resources/` instead of `docs/`
2. WHEN THE MCP Server processes resource URIs, THE Server SHALL resolve paths relative to `mcp/resources/` directory
3. WHEN THE File System Monitor watches for changes, THE Monitor SHALL watch `mcp/resources/` directory tree
4. THE MCP Server SHALL maintain the same subdirectory structure (`guidelines/`, `patterns/`, `adr/`) under `mcp/resources/`

### Requirement 2

**User Story:** As a developer, I want the MCP server to read prompts from `/mcp/prompts/` instead of `/prompts/`, so that all MCP assets are organized under a single `/mcp/` directory

#### Acceptance Criteria

1. WHEN THE Prompt Manager initializes, THE Manager SHALL load prompt definitions from `mcp/prompts/` directory
2. WHEN THE File System Monitor watches for prompt changes, THE Monitor SHALL watch `mcp/prompts/` directory
3. THE Prompt Manager SHALL detect and reload prompt files from `mcp/prompts/` within 2 seconds of changes

### Requirement 3

**User Story:** As a developer, I want all tests to pass with the new directory structure, so that I can verify the refactoring is correct

#### Acceptance Criteria

1. WHEN unit tests reference file paths, THE Tests SHALL use `mcp/resources/` and `mcp/prompts/` paths
2. WHEN integration tests create test fixtures, THE Tests SHALL create files under `mcp/resources/` and `mcp/prompts/`
3. WHEN benchmark tests measure performance, THE Tests SHALL use `mcp/resources/` paths for test data
4. THE Test Suite SHALL pass with 100% of existing tests passing after path updates

### Requirement 4

**User Story:** As a DevOps engineer, I want Docker containers to mount the correct directories, so that the containerized server can access MCP resources

#### Acceptance Criteria

1. WHEN THE Dockerfile copies resources, THE Dockerfile SHALL copy from `mcp/resources/` to `/app/mcp/resources/`
2. WHEN THE Dockerfile copies prompts, THE Dockerfile SHALL copy from `mcp/prompts/` to `/app/mcp/prompts/`
3. WHEN docker-compose mounts volumes, THE Configuration SHALL mount `./mcp/resources` and `./mcp/prompts` directories
4. THE Container SHALL successfully start and serve resources from the new paths

### Requirement 5

**User Story:** As a developer, I want documentation and configuration files updated, so that the new directory structure is properly documented

#### Acceptance Criteria

1. WHEN developers read README.md, THE Documentation SHALL reference `mcp/resources/` and `mcp/prompts/` directories
2. WHEN developers read steering files, THE Steering Documentation SHALL reference `mcp/resources/` and `mcp/prompts/` paths
3. WHEN developers read SECURITY.md, THE Security Documentation SHALL reference `mcp/resources/` as the restricted directory
4. THE Documentation SHALL accurately reflect the new directory structure throughout

### Requirement 6

**User Story:** As a developer, I want the codebase to follow Effective Go guidelines and modern Go idioms, so that the code is maintainable, readable, and performant

#### Acceptance Criteria

1. WHEN THE Codebase is formatted, THE Code SHALL pass `go fmt` checks for indentation, line length, and naming conventions
2. WHEN functions exceed 100 lines, THE Code SHALL be refactored into smaller, focused functions with single responsibilities
3. WHEN files exceed 500 lines, THE Code SHALL be split into multiple files with clear separation of concerns
4. WHEN error handling is implemented, THE Code SHALL use early returns to avoid nested if-else statements
5. WHEN nesting depth exceeds 3 levels, THE Code SHALL be refactored to reduce complexity
6. WHEN magic strings or numbers appear, THE Code SHALL extract them to named constants with clear intent
7. WHEN comments are written, THE Comments SHALL explain WHY decisions were made, not WHAT the code does
8. THE Code SHALL follow idiomatic Go patterns including proper interface usage, error wrapping, and concurrency patterns

### Requirement 7

**User Story:** As a developer, I want large files refactored into manageable modules, so that the codebase is easier to navigate and maintain

#### Acceptance Criteria

1. WHEN `internal/server/server.go` exceeds 500 lines, THE File SHALL be split into logical modules (handlers, initialization, lifecycle)
2. WHEN `pkg/` packages have complex logic, THE Packages SHALL be organized with clear file boundaries per responsibility
3. WHEN test files exceed 800 lines, THE Tests SHALL be split into multiple files by functional area
4. THE Codebase SHALL maintain clear package boundaries with no circular dependencies

### Requirement 8

**User Story:** As a developer, I want appropriate test coverage without over-testing, so that tests are maintainable and provide value

#### Acceptance Criteria

1. WHEN core business logic is implemented, THE Code SHALL have unit tests covering happy paths and critical error cases
2. WHEN integration points exist, THE Code SHALL have integration tests for key workflows
3. WHEN tests become repetitive, THE Tests SHALL use table-driven test patterns to reduce duplication
4. THE Test Suite SHALL maintain coverage above 70% for critical packages without excessive test file sizes
5. THE Tests SHALL focus on behavior verification rather than implementation details
