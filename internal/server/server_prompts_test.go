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
	"mcp-architecture-service/pkg/prompts"
)

func TestHandlePromptsList(t *testing.T) {
	server := NewMCPServer()

	// Create temporary prompts directory for testing
	tmpDir := t.TempDir()
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

	if err := os.WriteFile(filepath.Join(tmpDir, "test-list-prompt.json"), []byte(testPromptContent), 0644); err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Update prompt manager to use test directory
	server.promptManager = prompts.NewPromptManager(tmpDir, server.cache, server.monitor)

	// Load prompts from the test directory
	if err := server.promptManager.LoadPrompts(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

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

	// Create temporary prompts directory for testing
	tmpDir := t.TempDir()
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

	if err := os.WriteFile(filepath.Join(tmpDir, "test-get-prompt.json"), []byte(testPromptContent), 0644); err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Update prompt manager to use test directory
	server.promptManager = prompts.NewPromptManager(tmpDir, server.cache, server.monitor)

	// Load prompts from the test directory
	if err := server.promptManager.LoadPrompts(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

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

	if response == nil {
		t.Fatal("handlePromptsGet() returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != "test-prompts-get" {
		t.Errorf("Expected ID 'test-prompts-get', got '%v'", response.ID)
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

	// Verify message structure
	for _, msg := range result.Messages {
		if msg.Role == "" {
			t.Error("Message role should not be empty")
		}
		if msg.Content.Type == "" {
			t.Error("Message content type should not be empty")
		}
		if msg.Content.Text == "" {
			t.Error("Message content text should not be empty")
		}
		// Verify argument substitution occurred
		if strings.Contains(msg.Content.Text, "{{input}}") {
			t.Error("Template variable {{input}} was not substituted")
		}
		if !strings.Contains(msg.Content.Text, "test value") {
			t.Error("Expected rendered text to contain 'test value'")
		}
	}
}

func TestHandlePromptsGetErrors(t *testing.T) {
	server := NewMCPServer()

	// Create temporary prompts directory for testing
	tmpDir := t.TempDir()
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

	if err := os.WriteFile(filepath.Join(tmpDir, "test-error-prompt.json"), []byte(testPromptContent), 0644); err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Update prompt manager to use test directory
	server.promptManager = prompts.NewPromptManager(tmpDir, server.cache, server.monitor)

	// Load prompts
	if err := server.promptManager.LoadPrompts(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	// Test missing prompt name parameter
	getMessage1 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-get-no-name",
		Method:  "prompts/get",
		Params:  models.MCPPromptsGetParams{},
	}

	response1 := server.handlePromptsGet(getMessage1)
	if response1.Error == nil {
		t.Error("Expected error for missing prompt name parameter")
	}
	if response1.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response1.Error.Code)
	}

	// Test prompt not found
	getMessage2 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-get-not-found",
		Method:  "prompts/get",
		Params: models.MCPPromptsGetParams{
			Name: "nonexistent-prompt",
		},
	}

	response2 := server.handlePromptsGet(getMessage2)
	if response2.Error == nil {
		t.Error("Expected error for prompt not found")
	}
	if response2.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response2.Error.Code)
	}
	if !strings.Contains(response2.Error.Message, "not found") {
		t.Errorf("Expected error message to contain 'not found', got '%s'", response2.Error.Message)
	}

	// Test missing required argument
	getMessage3 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-get-missing-arg",
		Method:  "prompts/get",
		Params: models.MCPPromptsGetParams{
			Name:      "test-error-prompt",
			Arguments: map[string]interface{}{},
		},
	}

	response3 := server.handlePromptsGet(getMessage3)
	if response3.Error == nil {
		t.Error("Expected error for missing required argument")
	}
	if response3.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response3.Error.Code)
	}

	// Test argument too long
	longInput := strings.Repeat("a", 200) // Exceeds maxLength of 100
	getMessage4 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-get-arg-too-long",
		Method:  "prompts/get",
		Params: models.MCPPromptsGetParams{
			Name: "test-error-prompt",
			Arguments: map[string]interface{}{
				"input": longInput,
			},
		},
	}

	response4 := server.handlePromptsGet(getMessage4)
	if response4.Error == nil {
		t.Error("Expected error for argument too long")
	}
	if response4.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response4.Error.Code)
	}
}

func TestPromptsIntegrationFlow(t *testing.T) {
	server := NewMCPServer()

	// Create temporary prompts directory for testing
	tmpDir := t.TempDir()
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

	if err := os.WriteFile(filepath.Join(tmpDir, "test-integration-prompt.json"), []byte(testPromptContent), 0644); err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Update prompt manager to use test directory
	server.promptManager = prompts.NewPromptManager(tmpDir, server.cache, server.monitor)

	// Load prompts
	if err := server.promptManager.LoadPrompts(); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

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
