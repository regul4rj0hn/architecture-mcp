package tools

import (
	"context"
)

// Tool represents an executable function exposed via MCP
type Tool interface {
	// Name returns the unique identifier for the tool
	Name() string

	// Description returns a human-readable description
	Description() string

	// InputSchema returns JSON schema for tool parameters
	InputSchema() map[string]interface{}

	// Execute runs the tool with validated arguments
	// Returns result data or error
	Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error)
}

// ToolDefinition represents metadata about a tool
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// NewToolDefinition creates a ToolDefinition from a Tool
func NewToolDefinition(tool Tool) ToolDefinition {
	return ToolDefinition{
		Name:        tool.Name(),
		Description: tool.Description(),
		InputSchema: tool.InputSchema(),
	}
}
