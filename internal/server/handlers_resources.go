package server

import (
	"encoding/json"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

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
