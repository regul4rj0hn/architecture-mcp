package server

import (
	"encoding/json"
	"strings"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

// handleCompletionComplete handles the completion/complete method
func (s *MCPServer) handleCompletionComplete(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse request parameters
	var params models.MCPCompletionCompleteParams
	if message.Params != nil {
		paramsBytes, err := json.Marshal(message.Params)
		if err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters")
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters format")
		}
	}

	// Validate parameters
	if err := s.validateCompletionParams(&params); err != nil {
		return s.createStructuredErrorResponse(message.ID, err)
	}

	// Generate completions with circuit breaker protection
	circuitBreaker := s.circuitBreakerManager.GetOrCreate("completion",
		errors.DefaultCircuitBreakerConfig("completion"))

	var completions []models.MCPCompletionItem
	err := circuitBreaker.Execute(func() error {
		var genErr error
		completions, genErr = s.generateCompletions(&params)
		return genErr
	})

	if err != nil {
		return s.handleCompletionError(message.ID, params.Ref.Name, params.Argument.Name, err)
	}

	result := models.MCPCompletionResult{
		Completion: models.MCPCompletion{
			Values:  completions,
			Total:   len(completions),
			HasMore: false,
		},
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// validateCompletionParams validates completion request parameters
func (s *MCPServer) validateCompletionParams(params *models.MCPCompletionCompleteParams) *errors.StructuredError {
	// Validate ref type
	if params.Ref.Type != "ref/prompt" {
		return errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Invalid reference type", nil).
			WithContext("ref_type", params.Ref.Type).
			WithContext("expected", "ref/prompt")
	}

	// Validate prompt name
	if params.Ref.Name == "" {
		return errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Missing required parameter: ref.name", nil)
	}

	// Validate prompt exists
	prompts := s.promptManager.ListPrompts()
	promptExists := false
	for _, prompt := range prompts {
		if prompt.Name == params.Ref.Name {
			promptExists = true
			break
		}
	}

	if !promptExists {
		return errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Prompt not found", nil).
			WithContext("prompt_name", params.Ref.Name)
	}

	// Validate argument name
	if params.Argument.Name == "" {
		return errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Missing required parameter: argument.name", nil)
	}

	return nil
}

// generateCompletions generates completion suggestions based on argument type
func (s *MCPServer) generateCompletions(params *models.MCPCompletionCompleteParams) ([]models.MCPCompletionItem, error) {
	// Determine completion type based on argument name
	switch params.Argument.Name {
	case "pattern_name":
		return s.generatePatternCompletions(params.Argument.Value)
	case "guideline_name":
		return s.generateGuidelineCompletions(params.Argument.Value)
	case "adr_id":
		return s.generateADRCompletions(params.Argument.Value)
	default:
		// No completions for unknown argument types
		return []models.MCPCompletionItem{}, nil
	}
}

// generatePatternCompletions generates completions for pattern names
func (s *MCPServer) generatePatternCompletions(prefix string) ([]models.MCPCompletionItem, error) {
	allDocuments := s.cache.GetAllDocuments()
	var completions []models.MCPCompletionItem

	lowerPrefix := strings.ToLower(prefix)

	for _, doc := range allDocuments {
		if doc.Metadata.Category != config.CategoryPattern {
			continue
		}

		// Extract pattern name from path (e.g., "repository-pattern.md" → "repository pattern")
		patternName := strings.TrimSuffix(doc.Metadata.Path, config.MarkdownExtension)
		patternName = strings.TrimPrefix(patternName, config.PatternsPath+"/")
		patternName = strings.ReplaceAll(patternName, "-", " ")

		// Filter by prefix (case-insensitive)
		if prefix == "" || strings.HasPrefix(strings.ToLower(patternName), lowerPrefix) {
			completions = append(completions, models.MCPCompletionItem{
				Value:       patternName,
				Label:       patternName,
				Description: doc.Metadata.Title,
			})
		}
	}

	return completions, nil
}

// generateGuidelineCompletions generates completions for guideline names
func (s *MCPServer) generateGuidelineCompletions(prefix string) ([]models.MCPCompletionItem, error) {
	allDocuments := s.cache.GetAllDocuments()
	var completions []models.MCPCompletionItem

	lowerPrefix := strings.ToLower(prefix)

	for _, doc := range allDocuments {
		if doc.Metadata.Category != config.CategoryGuideline {
			continue
		}

		// Extract guideline name from path (e.g., "api-design.md" → "api design")
		guidelineName := strings.TrimSuffix(doc.Metadata.Path, config.MarkdownExtension)
		guidelineName = strings.TrimPrefix(guidelineName, config.GuidelinesPath+"/")
		guidelineName = strings.ReplaceAll(guidelineName, "-", " ")

		// Filter by prefix (case-insensitive)
		if prefix == "" || strings.HasPrefix(strings.ToLower(guidelineName), lowerPrefix) {
			completions = append(completions, models.MCPCompletionItem{
				Value:       guidelineName,
				Label:       guidelineName,
				Description: doc.Metadata.Title,
			})
		}
	}

	return completions, nil
}

// generateADRCompletions generates completions for ADR identifiers
func (s *MCPServer) generateADRCompletions(prefix string) ([]models.MCPCompletionItem, error) {
	allDocuments := s.cache.GetAllDocuments()
	var completions []models.MCPCompletionItem

	lowerPrefix := strings.ToLower(prefix)

	for _, doc := range allDocuments {
		if doc.Metadata.Category != config.CategoryADR {
			continue
		}

		// Extract ADR ID from path using the same logic as generateResourceURI
		cleanPath := strings.TrimSuffix(doc.Metadata.Path, config.MarkdownExtension)
		cleanPath = strings.TrimPrefix(cleanPath, config.ADRPath+"/")
		adrId := s.extractADRId(cleanPath)

		// Filter by prefix (case-insensitive)
		if prefix == "" || strings.HasPrefix(strings.ToLower(adrId), lowerPrefix) {
			completions = append(completions, models.MCPCompletionItem{
				Value:       adrId,
				Label:       adrId,
				Description: doc.Metadata.Title,
			})
		}
	}

	return completions, nil
}

// handleCompletionError creates appropriate error response for completion errors
func (s *MCPServer) handleCompletionError(id interface{}, promptName, argumentName string, err error) *models.MCPMessage {
	// Check if it's already a structured error
	if structuredErr, ok := err.(*errors.StructuredError); ok {
		s.logger.WithError(err).
			WithContext("prompt_name", promptName).
			WithContext("argument_name", argumentName).
			Error("Completion request failed")
		return s.createStructuredErrorResponse(id, structuredErr)
	}

	// Create generic internal error
	structuredErr := errors.NewSystemError("COMPLETION_FAILED",
		"Internal error", err).
		WithContext("prompt_name", promptName).
		WithContext("argument_name", argumentName)

	s.logger.WithError(err).
		WithContext("prompt_name", promptName).
		WithContext("argument_name", argumentName).
		Error("Completion request failed")

	return s.createStructuredErrorResponse(id, structuredErr)
}
