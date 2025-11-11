package server

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

// handleInitialize handles the MCP initialize method
func (s *MCPServer) handleInitialize(message *models.MCPMessage) *models.MCPMessage {
	result := models.MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    s.capabilities,
		ServerInfo:      s.serverInfo,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleInitialized handles the notifications/initialized method
func (s *MCPServer) handleInitialized(message *models.MCPMessage) *models.MCPMessage {
	s.initialized = true
	s.logger.WithContext("request_id", message.ID).Info("MCP server initialized successfully")
	return nil // No response for notifications
}

// handleResourcesList handles the resources/list method
func (s *MCPServer) handleResourcesList(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var resources []models.MCPResource

	// Get all cached documents and convert them to MCP resources
	allDocuments := s.cache.GetAllDocuments()

	for _, doc := range allDocuments {
		resource := s.createMCPResourceFromDocument(doc)
		resources = append(resources, resource)
	}

	result := models.MCPResourcesListResult{
		Resources: resources,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleResourcesRead handles the resources/read method
func (s *MCPServer) handleResourcesRead(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse request parameters
	var params models.MCPResourcesReadParams
	if message.Params != nil {
		paramsBytes, err := json.Marshal(message.Params)
		if err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters")
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters format")
		}
	}

	// Validate URI parameter
	if params.URI == "" {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Missing required parameter: uri", nil)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Parse the MCP resource URI
	category, path, err := s.parseResourceURI(params.URI)
	if err != nil {
		// If it's already a structured error, use it directly
		if structuredErr, ok := err.(*errors.StructuredError); ok {
			return s.createStructuredErrorResponse(message.ID, structuredErr)
		}
		// Otherwise, wrap it as a validation error
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid resource URI", err).WithContext("uri", params.URI)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Find the document in cache with circuit breaker protection
	circuitBreaker := s.circuitBreakerManager.GetOrCreate("resource_read",
		errors.DefaultCircuitBreakerConfig("resource_read"))

	var document *models.Document
	err = circuitBreaker.Execute(func() error {
		var findErr error
		document, findErr = s.findDocumentByResourcePath(category, path)
		return findErr
	})

	if err != nil {
		if structuredErr, ok := err.(*errors.StructuredError); ok {
			return s.createStructuredErrorResponse(message.ID, structuredErr)
		}

		structuredErr := errors.NewMCPError(errors.ErrCodeResourceNotFound,
			"Resource not found", err).
			WithContext("uri", params.URI).
			WithContext("category", category).
			WithContext("path", path)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Create resource content response
	content := models.MCPResourceContent{
		URI:      params.URI,
		MimeType: config.MimeTypeMarkdown,
		Text:     document.Content.RawContent,
	}

	result := models.MCPResourcesReadResult{
		Contents: []models.MCPResourceContent{content},
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

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

// handlePerformanceMetrics handles requests for server performance metrics
func (s *MCPServer) handlePerformanceMetrics(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect performance metrics from various components
	cacheMetrics := s.cache.GetPerformanceMetrics()
	promptMetrics := s.promptManager.GetPerformanceMetrics()

	// Add server-level metrics
	serverMetrics := map[string]interface{}{
		"server_info":    s.serverInfo,
		"initialized":    s.initialized,
		"cache_metrics":  cacheMetrics,
		"prompt_metrics": promptMetrics,
		"goroutines":     runtime.NumGoroutine(),
		"memory_stats":   getMemoryStats(),
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	// Add tool metrics if toolManager is initialized
	if s.toolManager != nil {
		serverMetrics["tool_metrics"] = s.toolManager.GetPerformanceMetrics()
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  serverMetrics,
	}
}

// createErrorResponse creates an MCP error response
func (s *MCPServer) createErrorResponse(id interface{}, code int, message string) *models.MCPMessage {
	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &models.MCPError{
			Code:    code,
			Message: message,
		},
	}
}

// createStructuredErrorResponse creates an MCP error response from a structured error
func (s *MCPServer) createStructuredErrorResponse(id interface{}, structuredErr *errors.StructuredError) *models.MCPMessage {
	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   structuredErr.ToMCPError(),
	}
}

// getMemoryStats returns current memory statistics
func getMemoryStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"alloc_bytes":       m.Alloc,
		"total_alloc_bytes": m.TotalAlloc,
		"sys_bytes":         m.Sys,
		"num_gc":            m.NumGC,
		"gc_cpu_fraction":   m.GCCPUFraction,
	}
}
