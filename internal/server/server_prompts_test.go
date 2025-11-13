package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/logging"
	"mcp-architecture-service/pkg/prompts"
)

// setupTestPromptFromJSON creates a test prompt file from JSON content and initializes the prompt manager
func setupTestPromptFromJSON(t *testing.T, server *MCPServer, promptName, promptContent string) string {
	t.Helper()
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, promptName+".json"), []byte(promptContent), 0644); err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	logger := logging.NewStructuredLogger("test")
	server.promptManager = prompts.NewPromptManager(tmpDir, server.cache, server.monitor, logger)

	if err := server.promptManager.LoadPrompts(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	return tmpDir
}

// validatePromptsGetResponse validates the basic structure of a prompts/get response
func validatePromptsGetResponse(t *testing.T, response *models.MCPMessage, expectedID string) models.MCPPromptsGetResult {
	t.Helper()

	if response == nil {
		t.Fatal("handlePromptsGet() returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != expectedID {
		t.Errorf("Expected ID '%s', got '%v'", expectedID, response.ID)
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}

	// Verify result structure - try both pointer and value types
	var result models.MCPPromptsGetResult
	if resultPtr, ok := response.Result.(*models.MCPPromptsGetResult); ok {
		result = *resultPtr
	} else if resultVal, ok := response.Result.(models.MCPPromptsGetResult); ok {
		result = resultVal
	} else {
		t.Fatalf("Expected result to be MCPPromptsGetResult, got %T", response.Result)
	}

	if len(result.Messages) == 0 {
		t.Error("Expected at least one message in result")
	}

	return result
}

// validateMessageStructure validates the structure of prompt messages
func validateMessageStructure(t *testing.T, messages []models.MCPPromptMessage) {
	t.Helper()

	for _, msg := range messages {
		if msg.Role == "" {
			t.Error("Message role should not be empty")
		}
		if msg.Content.Type == "" {
			t.Error("Message content type should not be empty")
		}
		if msg.Content.Text == "" {
			t.Error("Message content text should not be empty")
		}
	}
}

// validateArgumentSubstitution validates that template variables were substituted correctly
func validateArgumentSubstitution(t *testing.T, messages []models.MCPPromptMessage, templateVar, expectedValue string) {
	t.Helper()

	for _, msg := range messages {
		if strings.Contains(msg.Content.Text, "{{"+templateVar+"}}") {
			t.Errorf("Template variable {{%s}} was not substituted", templateVar)
		}
		if !strings.Contains(msg.Content.Text, expectedValue) {
			t.Errorf("Expected rendered text to contain '%s'", expectedValue)
		}
	}
}

func TestHandlePromptsList(t *testing.T) {
	server := NewMCPServer()

	testPromptContent := `{
		"name": "test-list-prompt",
		"description": "A test prompt for list testing",
		"arguments": [
			{
				"name": "input",
				"description": "Test input",
				"required": true
			}
		],
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Test: {{input}}"
				}
			}
		]
	}`

	setupTestPromptFromJSON(t, server, "test-list-prompt", testPromptContent)

	listMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-prompts-list",
		Method:  "prompts/list",
	}

	response := server.handlePromptsList(listMessage)

	if response == nil {
		t.Fatal("handlePromptsList() returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != "test-prompts-list" {
		t.Errorf("Expected ID 'test-prompts-list', got '%v'", response.ID)
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}

	// Verify result structure
	result, ok := response.Result.(models.MCPPromptsListResult)
	if !ok {
		t.Fatal("Expected result to be MCPPromptsListResult")
	}

	// Should return at least the test prompt
	if len(result.Prompts) == 0 {
		t.Error("Expected at least one prompt in the list")
	}

	// Verify prompt properties
	namePattern := regexp.MustCompile("^[a-z0-9-]+$")
	for _, prompt := range result.Prompts {
		if prompt.Name == "" {
			t.Error("Prompt name should not be empty")
		}
		// Verify name follows pattern
		if !namePattern.MatchString(prompt.Name) {
			t.Errorf("Prompt name '%s' does not match pattern ^[a-z0-9-]+$", prompt.Name)
		}
	}

	// Verify prompts are sorted alphabetically
	for i := 1; i < len(result.Prompts); i++ {
		if result.Prompts[i-1].Name > result.Prompts[i].Name {
			t.Errorf("Prompts not sorted alphabetically: '%s' comes after '%s'",
				result.Prompts[i-1].Name, result.Prompts[i].Name)
		}
	}
}

func TestHandlePromptsGet(t *testing.T) {
	server := NewMCPServer()

	testPromptContent := `{
		"name": "test-get-prompt",
		"description": "A test prompt for get testing",
		"arguments": [
			{
				"name": "input",
				"description": "Test input",
				"required": true,
				"maxLength": 100
			}
		],
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Test message with {{input}}"
				}
			}
		]
	}`

	setupTestPromptFromJSON(t, server, "test-get-prompt", testPromptContent)

	// Test successful prompt retrieval with valid arguments
	getMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-prompts-get",
		Method:  "prompts/get",
		Params: models.MCPPromptsGetParams{
			Name: "test-get-prompt",
			Arguments: map[string]interface{}{
				"input": "test value",
			},
		},
	}

	response := server.handlePromptsGet(getMessage)
	result := validatePromptsGetResponse(t, response, "test-prompts-get")
	validateMessageStructure(t, result.Messages)
	validateArgumentSubstitution(t, result.Messages, "input", "test value")
}

func TestHandlePromptsGetErrors(t *testing.T) {
	server := NewMCPServer()

	testPromptContent := `{
		"name": "test-error-prompt",
		"description": "A test prompt for error testing",
		"arguments": [
			{
				"name": "input",
				"description": "Test input",
				"required": true,
				"maxLength": 100
			}
		],
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Test: {{input}}"
				}
			}
		]
	}`

	setupTestPromptFromJSON(t, server, "test-error-prompt", testPromptContent)

	tests := []struct {
		name              string
		params            models.MCPPromptsGetParams
		expectedErrorCode int
		errorMsgContains  string
	}{
		{
			name:              "missing prompt name",
			params:            models.MCPPromptsGetParams{},
			expectedErrorCode: -32602,
			errorMsgContains:  "",
		},
		{
			name: "prompt not found",
			params: models.MCPPromptsGetParams{
				Name: "nonexistent-prompt",
			},
			expectedErrorCode: -32602,
			errorMsgContains:  "not found",
		},
		{
			name: "missing required argument",
			params: models.MCPPromptsGetParams{
				Name:      "test-error-prompt",
				Arguments: map[string]interface{}{},
			},
			expectedErrorCode: -32602,
			errorMsgContains:  "",
		},
		{
			name: "argument too long",
			params: models.MCPPromptsGetParams{
				Name: "test-error-prompt",
				Arguments: map[string]interface{}{
					"input": strings.Repeat("a", 200),
				},
			},
			expectedErrorCode: -32602,
			errorMsgContains:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-" + tt.name,
				Method:  "prompts/get",
				Params:  tt.params,
			}

			response := server.handlePromptsGet(message)

			if response.Error == nil {
				t.Errorf("Expected error for %s", tt.name)
				return
			}

			if response.Error.Code != tt.expectedErrorCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedErrorCode, response.Error.Code)
			}

			if tt.errorMsgContains != "" && !strings.Contains(response.Error.Message, tt.errorMsgContains) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsgContains, response.Error.Message)
			}
		})
	}
}

func TestPromptsIntegrationFlow(t *testing.T) {
	server := NewMCPServer()

	testPromptContent := `{
		"name": "test-integration-prompt",
		"description": "A test prompt for integration testing",
		"arguments": [
			{
				"name": "input",
				"description": "Test input",
				"required": true
			}
		],
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Integration test: {{input}}"
				}
			}
		]
	}`

	setupTestPromptFromJSON(t, server, "test-integration-prompt", testPromptContent)

	// Step 1: List available prompts
	listJSON := `{"jsonrpc":"2.0","id":"list-1","method":"prompts/list"}`
	reader := strings.NewReader(listJSON)
	writer := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := server.processMessages(ctx, reader, writer)
	if err != nil {
		t.Errorf("Expected nil error (EOF), got %v", err)
	}

	// Verify list response
	var listResponse models.MCPMessage
	if err := json.Unmarshal(writer.Bytes(), &listResponse); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	if listResponse.Error != nil {
		t.Errorf("List prompts failed with error: %v", listResponse.Error)
	}

	// Step 2: Get a specific prompt
	getJSON := `{"jsonrpc":"2.0","id":"get-1","method":"prompts/get","params":{"name":"test-integration-prompt","arguments":{"input":"hello world"}}}`
	reader2 := strings.NewReader(getJSON)
	writer2 := &bytes.Buffer{}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	err2 := server.processMessages(ctx2, reader2, writer2)
	if err2 != nil {
		t.Errorf("Expected nil error (EOF), got %v", err2)
	}

	// Verify get response
	var getResponse models.MCPMessage
	if err := json.Unmarshal(writer2.Bytes(), &getResponse); err != nil {
		t.Fatalf("Failed to parse get response: %v", err)
	}

	if getResponse.Error != nil {
		t.Errorf("Get prompt failed with error: %v", getResponse.Error)
	}

	// Verify the response contains rendered content
	resultMap, ok := getResponse.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	messages, ok := resultMap["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Error("Expected messages array in result")
	}
}
