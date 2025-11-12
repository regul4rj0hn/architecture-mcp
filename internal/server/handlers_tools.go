package server

import (
	"context"
	"encoding/json"
	"strings"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
)

// handleToolsList handles the tools/list method
func (s *MCPServer) handleToolsList(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if toolManager is initialized
	if s.toolManager == nil {
		structuredErr := errors.NewSystemError("TOOLS_NOT_INITIALIZED",
			"Tools system not initialized", nil)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Get all available tools from the tool manager
	toolDefinitions := s.toolManager.ListTools()

	// Convert to MCP protocol format
	mcpTools := make([]models.MCPTool, 0, len(toolDefinitions))
	for _, toolDef := range toolDefinitions {
		mcpTools = append(mcpTools, models.MCPTool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			InputSchema: toolDef.InputSchema,
		})
	}

	result := models.MCPToolsListResult{
		Tools: mcpTools,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleToolsCall handles the tools/call method
func (s *MCPServer) handleToolsCall(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if toolManager is initialized
	if s.toolManager == nil {
		structuredErr := errors.NewSystemError("TOOLS_NOT_INITIALIZED",
			"Tools system not initialized", nil)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Parse request parameters
	var params models.MCPToolsCallParams
	if message.Params != nil {
		paramsBytes, err := json.Marshal(message.Params)
		if err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters")
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters format")
		}
	}

	// Validate tool name parameter
	if params.Name == "" {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Missing required parameter: name", nil)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Execute the tool with circuit breaker protection
	circuitBreaker := s.circuitBreakerManager.GetOrCreate(
		"tool_"+params.Name,
		errors.DefaultCircuitBreakerConfig("tool_"+params.Name))

	var result interface{}

	err := circuitBreaker.Execute(func() error {
		var ctxErr error
		// Create a context for tool execution
		ctx := context.Background()
		result, ctxErr = s.toolManager.ExecuteTool(ctx, params.Name, params.Arguments)
		return ctxErr
	})

	if err != nil {
		return s.handleToolExecutionError(message.ID, params.Name, err)
	}

	// Convert result to MCP tool content format
	// Result is expected to be a string or convertible to JSON
	var contentText string
	if strResult, ok := result.(string); ok {
		contentText = strResult
	} else {
		// Convert to JSON for structured results
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			structuredErr := errors.NewSystemError("TOOL_RESULT_SERIALIZATION_FAILED",
				"Failed to serialize tool result", err).
				WithContext("tool_name", params.Name)
			return s.createStructuredErrorResponse(message.ID, structuredErr)
		}
		contentText = string(jsonBytes)
	}

	toolResult := models.MCPToolsCallResult{
		Content: []models.MCPToolContent{
			{
				Type: "text",
				Text: contentText,
			},
		},
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  toolResult,
	}
}

// handleToolExecutionError creates appropriate error response based on tool execution error
func (s *MCPServer) handleToolExecutionError(id interface{}, toolName string, err error) *models.MCPMessage {
	// Check if it's already a structured error
	if structuredErr, ok := err.(*errors.StructuredError); ok {
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	// Check for specific error types
	if strings.Contains(err.Error(), "tool not found") {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Tool not found", err).WithContext("tool_name", toolName)
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	if strings.Contains(err.Error(), "validation failed") ||
		strings.Contains(err.Error(), "invalid argument") ||
		strings.Contains(err.Error(), "required field") {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Tool argument validation failed", err).WithContext("tool_name", toolName)
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	if strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded") {
		structuredErr := errors.NewSystemError("TOOL_EXECUTION_TIMEOUT",
			"Tool execution timeout", err).WithContext("tool_name", toolName)
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	// Generic tool execution error
	structuredErr := errors.NewSystemError("TOOL_EXECUTION_FAILED",
		"Tool execution failed", err).WithContext("tool_name", toolName)
	return s.createStructuredErrorResponse(id, structuredErr)
}
