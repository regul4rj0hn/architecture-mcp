package server

import (
	"os"
	"path/filepath"
	"testing"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/logging"
	"mcp-architecture-service/pkg/prompts"
)

// setupTestPrompt creates a temporary prompt directory with a test prompt
func setupTestPrompt(t *testing.T, server *MCPServer, promptName string, argumentName string) {
	tmpDir := t.TempDir()
	testPromptContent := `{
		"name": "` + promptName + `",
		"description": "A test prompt",
		"arguments": [
			{
				"name": "` + argumentName + `",
				"description": "Test argument",
				"required": true
			}
		],
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Test: {{` + argumentName + `}}"
				}
			}
		]
	}`

	if err := os.WriteFile(filepath.Join(tmpDir, promptName+".json"), []byte(testPromptContent), 0644); err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Update prompt manager to use test directory
	logger := logging.NewStructuredLogger("test")
	server.promptManager = prompts.NewPromptManager(tmpDir, server.cache, server.monitor, logger)

	// Load prompts from the test directory
	if err := server.promptManager.LoadPrompts(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}
}

// TestHandleCompletionComplete_ValidPatternName tests pattern_name completion with prefix filtering
func TestHandleCompletionComplete_ValidPatternName(t *testing.T) {
	server := NewMCPServer()

	// Setup test prompt
	setupTestPrompt(t, server, "test-prompt", "pattern_name")

	// Add test documents to cache
	testDoc := &models.Document{
		Content: models.DocumentContent{
			RawContent: "# Repository Pattern\nTest content",
		},
		Metadata: models.DocumentMetadata{
			Path:     "mcp/resources/patterns/repository-pattern.md",
			Category: "pattern",
			Title:    "Repository Pattern",
		},
	}
	server.cache.Set("architecture://patterns/repository-pattern", testDoc)

	// Test with prefix filtering
	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-1",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "ref/prompt",
				Name: "test-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "pattern_name",
				Value: "repo",
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error != nil {
		t.Fatalf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(models.MCPCompletionResult)
	if !ok {
		t.Fatal("Expected result to be MCPCompletionResult")
	}

	if len(result.Completion.Values) == 0 {
		t.Error("Expected at least one completion value")
	}

	// Verify completion item structure
	if len(result.Completion.Values) > 0 {
		item := result.Completion.Values[0]
		if item.Value == "" {
			t.Error("Completion item value should not be empty")
		}
		if item.Label == "" {
			t.Error("Completion item label should not be empty")
		}
	}
}

// TestHandleCompletionComplete_InvalidRefType tests error handling for invalid ref type
func TestHandleCompletionComplete_InvalidRefType(t *testing.T) {
	server := NewMCPServer()

	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-2",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "invalid/type",
				Name: "test-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "pattern_name",
				Value: "test",
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error == nil {
		t.Fatal("Expected error for invalid ref type")
	}

	if response.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response.Error.Code)
	}
}

// TestHandleCompletionComplete_PromptNotFound tests error handling when prompt doesn't exist
func TestHandleCompletionComplete_PromptNotFound(t *testing.T) {
	server := NewMCPServer()

	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-3",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "ref/prompt",
				Name: "nonexistent-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "pattern_name",
				Value: "test",
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error == nil {
		t.Fatal("Expected error for nonexistent prompt")
	}

	if response.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response.Error.Code)
	}
}

// TestHandleCompletionComplete_EmptyPrefix tests that empty prefix returns all completions
func TestHandleCompletionComplete_EmptyPrefix(t *testing.T) {
	server := NewMCPServer()

	// Setup test prompt
	setupTestPrompt(t, server, "test-prompt", "pattern_name")

	// Add multiple test documents
	docs := []*models.Document{
		{
			Content: models.DocumentContent{
				RawContent: "# Repository Pattern",
			},
			Metadata: models.DocumentMetadata{
				Path:     "mcp/resources/patterns/repository-pattern.md",
				Category: "pattern",
				Title:    "Repository Pattern",
			},
		},
		{
			Content: models.DocumentContent{
				RawContent: "# Factory Pattern",
			},
			Metadata: models.DocumentMetadata{
				Path:     "mcp/resources/patterns/factory-pattern.md",
				Category: "pattern",
				Title:    "Factory Pattern",
			},
		},
	}

	for _, doc := range docs {
		key := "architecture://patterns/" + doc.Metadata.Path[len("mcp/resources/patterns/"):]
		server.cache.Set(key, doc)
	}

	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-4",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "ref/prompt",
				Name: "test-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "pattern_name",
				Value: "", // Empty prefix
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error != nil {
		t.Fatalf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(models.MCPCompletionResult)
	if !ok {
		t.Fatal("Expected result to be MCPCompletionResult")
	}

	// Should return all patterns when prefix is empty
	if len(result.Completion.Values) != 2 {
		t.Errorf("Expected 2 completions for empty prefix, got %d", len(result.Completion.Values))
	}
}

// TestHandleCompletionComplete_UnknownArgument tests that unknown arguments return empty list
func TestHandleCompletionComplete_UnknownArgument(t *testing.T) {
	server := NewMCPServer()

	// Setup test prompt
	setupTestPrompt(t, server, "test-prompt", "unknown_argument")

	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-5",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "ref/prompt",
				Name: "test-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "unknown_argument",
				Value: "test",
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error != nil {
		t.Fatalf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(models.MCPCompletionResult)
	if !ok {
		t.Fatal("Expected result to be MCPCompletionResult")
	}

	// Should return empty list for unknown argument types
	if len(result.Completion.Values) != 0 {
		t.Errorf("Expected 0 completions for unknown argument, got %d", len(result.Completion.Values))
	}
}

// TestHandleCompletionComplete_GuidelineCompletions tests guideline_name completions
func TestHandleCompletionComplete_GuidelineCompletions(t *testing.T) {
	server := NewMCPServer()

	// Setup test prompt
	setupTestPrompt(t, server, "test-prompt", "guideline_name")

	// Add test guideline document
	testDoc := &models.Document{
		Content: models.DocumentContent{
			RawContent: "# API Design Guidelines",
		},
		Metadata: models.DocumentMetadata{
			Path:     "mcp/resources/guidelines/api-design.md",
			Category: "guideline",
			Title:    "API Design Guidelines",
		},
	}
	server.cache.Set("architecture://guidelines/api-design", testDoc)

	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-6",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "ref/prompt",
				Name: "test-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "guideline_name",
				Value: "api",
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error != nil {
		t.Fatalf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(models.MCPCompletionResult)
	if !ok {
		t.Fatal("Expected result to be MCPCompletionResult")
	}

	if len(result.Completion.Values) == 0 {
		t.Error("Expected at least one guideline completion")
	}
}

// TestHandleCompletionComplete_ADRCompletions tests adr_id completions
func TestHandleCompletionComplete_ADRCompletions(t *testing.T) {
	server := NewMCPServer()

	// Setup test prompt
	setupTestPrompt(t, server, "test-prompt", "adr_id")

	// Add test ADR document
	testDoc := &models.Document{
		Content: models.DocumentContent{
			RawContent: "# ADR 001: Microservices Architecture",
		},
		Metadata: models.DocumentMetadata{
			Path:     "mcp/resources/adr/001-microservices-architecture.md",
			Category: "adr",
			Title:    "ADR 001: Microservices Architecture",
		},
	}
	server.cache.Set("architecture://adr/001-microservices-architecture", testDoc)

	message := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-7",
		Method:  "completion/complete",
		Params: models.MCPCompletionCompleteParams{
			Ref: models.MCPCompletionRef{
				Type: "ref/prompt",
				Name: "test-prompt",
			},
			Argument: models.MCPCompletionArgument{
				Name:  "adr_id",
				Value: "001",
			},
		},
	}

	response := server.handleCompletionComplete(message)

	if response == nil {
		t.Fatal("handleCompletionComplete() returned nil")
	}

	if response.Error != nil {
		t.Fatalf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(models.MCPCompletionResult)
	if !ok {
		t.Fatal("Expected result to be MCPCompletionResult")
	}

	if len(result.Completion.Values) == 0 {
		t.Error("Expected at least one ADR completion")
	}
}

// TestGeneratePatternCompletions_PrefixFiltering tests case-insensitive prefix filtering
func TestGeneratePatternCompletions_PrefixFiltering(t *testing.T) {
	server := NewMCPServer()

	// Add test documents with different names
	docs := []*models.Document{
		{
			Content: models.DocumentContent{
				RawContent: "# Repository Pattern",
			},
			Metadata: models.DocumentMetadata{
				Path:     "mcp/resources/patterns/repository-pattern.md",
				Category: "pattern",
				Title:    "Repository Pattern",
			},
		},
		{
			Content: models.DocumentContent{
				RawContent: "# Factory Pattern",
			},
			Metadata: models.DocumentMetadata{
				Path:     "mcp/resources/patterns/factory-pattern.md",
				Category: "pattern",
				Title:    "Factory Pattern",
			},
		},
		{
			Content: models.DocumentContent{
				RawContent: "# Singleton Pattern",
			},
			Metadata: models.DocumentMetadata{
				Path:     "mcp/resources/patterns/singleton-pattern.md",
				Category: "pattern",
				Title:    "Singleton Pattern",
			},
		},
	}

	for _, doc := range docs {
		key := "architecture://patterns/" + doc.Metadata.Path[len("mcp/resources/patterns/"):]
		server.cache.Set(key, doc)
	}

	tests := []struct {
		name          string
		prefix        string
		expectedCount int
		shouldContain string
	}{
		{
			name:          "Empty prefix returns all",
			prefix:        "",
			expectedCount: 3,
			shouldContain: "",
		},
		{
			name:          "Lowercase prefix matches",
			prefix:        "repo",
			expectedCount: 1,
			shouldContain: "repository",
		},
		{
			name:          "Uppercase prefix matches (case-insensitive)",
			prefix:        "REPO",
			expectedCount: 1,
			shouldContain: "repository",
		},
		{
			name:          "Partial match",
			prefix:        "fact",
			expectedCount: 1,
			shouldContain: "factory",
		},
		{
			name:          "No match",
			prefix:        "xyz",
			expectedCount: 0,
			shouldContain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completions, err := server.generatePatternCompletions(tt.prefix)
			if err != nil {
				t.Fatalf("generatePatternCompletions() error = %v", err)
			}

			if len(completions) != tt.expectedCount {
				t.Errorf("Expected %d completions, got %d", tt.expectedCount, len(completions))
			}

			if tt.shouldContain != "" && len(completions) > 0 {
				found := false
				for _, c := range completions {
					if contains(c.Value, tt.shouldContain) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected completions to contain '%s'", tt.shouldContain)
				}
			}
		})
	}
}

// TestValidateCompletionParams tests parameter validation
func TestValidateCompletionParams(t *testing.T) {
	server := NewMCPServer()

	// Setup test prompt
	setupTestPrompt(t, server, "test-prompt", "pattern_name")

	tests := []struct {
		name        string
		params      models.MCPCompletionCompleteParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "Invalid ref type",
			params: models.MCPCompletionCompleteParams{
				Ref: models.MCPCompletionRef{
					Type: "invalid/type",
					Name: "test-prompt",
				},
				Argument: models.MCPCompletionArgument{
					Name:  "pattern_name",
					Value: "test",
				},
			},
			expectError: true,
			errorMsg:    "Invalid reference type",
		},
		{
			name: "Missing prompt name",
			params: models.MCPCompletionCompleteParams{
				Ref: models.MCPCompletionRef{
					Type: "ref/prompt",
					Name: "",
				},
				Argument: models.MCPCompletionArgument{
					Name:  "pattern_name",
					Value: "test",
				},
			},
			expectError: true,
			errorMsg:    "Missing required parameter: ref.name",
		},
		{
			name: "Missing argument name",
			params: models.MCPCompletionCompleteParams{
				Ref: models.MCPCompletionRef{
					Type: "ref/prompt",
					Name: "test-prompt",
				},
				Argument: models.MCPCompletionArgument{
					Name:  "",
					Value: "test",
				},
			},
			expectError: true,
			errorMsg:    "Missing required parameter: argument.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateCompletionParams(&tt.params)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if err.Message != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Message)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && contains(s[1:], substr)))
}
