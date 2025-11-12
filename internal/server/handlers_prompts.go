package server

import (
	"encoding/json"
	"strings"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
)

// handlePromptsList handles the prompts/list method
func (s *MCPServer) handlePromptsList(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get all available prompts from the prompt manager
	prompts := s.promptManager.ListPrompts()

	result := models.MCPPromptsListResult{
		Prompts: prompts,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handlePromptsGet handles the prompts/get method
func (s *MCPServer) handlePromptsGet(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse request parameters
	var params models.MCPPromptsGetParams
	if message.Params != nil {
		paramsBytes, err := json.Marshal(message.Params)
		if err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters")
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters format")
		}
	}

	// Validate prompt name parameter
	if params.Name == "" {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Missing required parameter: name", nil)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Render the prompt with provided arguments
	result, err := s.promptManager.RenderPrompt(params.Name, params.Arguments)
	if err != nil {
		return s.handlePromptRenderError(message.ID, params.Name, err)
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handlePromptRenderError creates appropriate error response based on prompt rendering error
func (s *MCPServer) handlePromptRenderError(id interface{}, promptName string, err error) *models.MCPMessage {
	if strings.Contains(err.Error(), "prompt not found") {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Prompt not found", err).WithContext("prompt_name", promptName)
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	if strings.Contains(err.Error(), "argument validation failed") ||
		strings.Contains(err.Error(), "required argument missing") ||
		strings.Contains(err.Error(), "exceeds maximum length") {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			err.Error(), err).WithContext("prompt_name", promptName)
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	if strings.Contains(err.Error(), "failed to embed resources") ||
		strings.Contains(err.Error(), "resource not found") {
		structuredErr := errors.NewMCPError(errors.ErrCodeResourceNotFound,
			"Failed to embed resources", err).WithContext("prompt_name", promptName)
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	structuredErr := errors.NewMCPError(errors.ErrCodeInvalidParams,
		"Failed to render prompt", err).WithContext("prompt_name", promptName)
	return s.createStructuredErrorResponse(id, structuredErr)
}
