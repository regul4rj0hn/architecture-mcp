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
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/prompts"
)

func TestNewMCPServer(t *testing.T) {
	server := NewMCPServer()

	if server == nil {
		t.Fatal("NewMCPServer() returned nil")
	}

	if server.serverInfo.Name != "mcp-architecture-service" {
		t.Errorf("Expected server name 'mcp-architecture-service', got '%s'", server.serverInfo.Name)
	}

	if server.serverInfo.Version != "1.0.0" {
		t.Errorf("Expected server version '1.0.0', got '%s'", server.serverInfo.Version)
	}

	if server.initialized {
		t.Error("Expected server to be uninitialized")
	}

	if server.capabilities.Resources == nil {
		t.Error("Expected resources capabilities to be set")
	}
}

func TestHandleInitialize(t *testing.T) {
	server := NewMCPServer()

	initMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-init",
		Method:  "initialize",
		Params: models.MCPInitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: models.MCPClientInfo{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	}

	response := server.handleInitialize(initMessage)

	if response == nil {
		t.Fatal("handleInitialize() returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != "test-init" {
		t.Errorf("Expected ID 'test-init', got '%v'", response.ID)
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}

	// Verify result structure
	result, ok := response.Result.(models.MCPInitializeResult)
	if !ok {
		t.Fatal("Expected result to be MCPInitializeResult")
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version '2024-11-05', got '%s'", result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "mcp-architecture-service" {
		t.Errorf("Expected server name 'mcp-architecture-service', got '%s'", result.ServerInfo.Name)
	}
}

func TestHandleInitialized(t *testing.T) {
	server := NewMCPServer()

	if server.initialized {
		t.Error("Expected server to be uninitialized initially")
	}

	initMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	response := server.handleInitialized(initMessage)

	// Notifications should not return a response
	if response != nil {
		t.Error("Expected no response for notification")
	}

	if !server.initialized {
		t.Error("Expected server to be initialized after handling initialized notification")
	}
}

func TestHandleResourcesList(t *testing.T) {
	server := NewMCPServer()

	// Add some test documents to the cache
	testDoc1 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "API Design Guidelines",
			Category:     config.CategoryGuideline,
			Path:         config.GuidelinesPath + "/api-design.md",
			LastModified: time.Now(),
			Size:         1024,
			Checksum:     "abc123",
		},
		Content: models.DocumentContent{
			RawContent: "# API Design Guidelines\nThis is a test document.",
		},
	}

	testDoc2 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Repository Pattern",
			Category:     config.CategoryPattern,
			Path:         config.PatternsPath + "/repository.md",
			LastModified: time.Now(),
			Size:         512,
			Checksum:     "def456",
		},
		Content: models.DocumentContent{
			RawContent: "# Repository Pattern\nThis is a test pattern.",
		},
	}

	server.cache.Set(testDoc1.Metadata.Path, testDoc1)
	server.cache.Set(testDoc2.Metadata.Path, testDoc2)

	listMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-list",
		Method:  "resources/list",
	}

	response := server.handleResourcesList(listMessage)

	if response == nil {
		t.Fatal("handleResourcesList() returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != "test-list" {
		t.Errorf("Expected ID 'test-list', got '%v'", response.ID)
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}

	// Verify result structure
	result, ok := response.Result.(models.MCPResourcesListResult)
	if !ok {
		t.Fatal("Expected result to be MCPResourcesListResult")
	}

	// Should return the test documents
	if len(result.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d resources", len(result.Resources))
	}

	// Verify resource properties
	for _, resource := range result.Resources {
		if resource.URI == "" {
			t.Error("Resource URI should not be empty")
		}
		if resource.Name == "" {
			t.Error("Resource name should not be empty")
		}
		if resource.MimeType != "text/markdown" {
			t.Errorf("Expected mimeType 'text/markdown', got '%s'", resource.MimeType)
		}
		if resource.Annotations == nil {
			t.Error("Resource annotations should not be nil")
		}
		if resource.Annotations["category"] == "" {
			t.Error("Resource should have category annotation")
		}
	}

	// Verify specific URIs are generated correctly
	foundGuideline := false
	foundPattern := false
	for _, resource := range result.Resources {
		if strings.Contains(resource.URI, "architecture://guidelines/") {
			foundGuideline = true
		}
		if strings.Contains(resource.URI, "architecture://patterns/") {
			foundPattern = true
		}
	}

	if !foundGuideline {
		t.Error("Expected to find guideline resource with correct URI")
	}
	if !foundPattern {
		t.Error("Expected to find pattern resource with correct URI")
	}
}

func TestHandleResourcesRead(t *testing.T) {
	server := NewMCPServer()

	// Add a test document to the cache
	testDoc := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "API Design Guidelines",
			Category:     config.CategoryGuideline,
			Path:         config.GuidelinesPath + "/api-design.md",
			LastModified: time.Now(),
			Size:         1024,
			Checksum:     "abc123",
		},
		Content: models.DocumentContent{
			RawContent: "# API Design Guidelines\nThis is a test document with guidelines for API design.",
		},
	}

	server.cache.Set(testDoc.Metadata.Path, testDoc)

	// Test successful resource read
	readMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-read",
		Method:  "resources/read",
		Params: models.MCPResourcesReadParams{
			URI: "architecture://guidelines/api-design",
		},
	}

	response := server.handleResourcesRead(readMessage)

	if response == nil {
		t.Fatal("handleResourcesRead() returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != "test-read" {
		t.Errorf("Expected ID 'test-read', got '%v'", response.ID)
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}

	// Verify result structure
	result, ok := response.Result.(models.MCPResourcesReadResult)
	if !ok {
		t.Fatal("Expected result to be MCPResourcesReadResult")
	}

	if len(result.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Contents))
	}

	content := result.Contents[0]
	if content.URI != "architecture://guidelines/api-design" {
		t.Errorf("Expected URI 'architecture://guidelines/api-design', got '%s'", content.URI)
	}

	if content.MimeType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", content.MimeType)
	}

	if content.Text != testDoc.Content.RawContent {
		t.Errorf("Expected content to match document raw content")
	}
}

func TestHandleResourcesReadErrors(t *testing.T) {
	server := NewMCPServer()

	// Test missing URI parameter
	readMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-read-no-uri",
		Method:  "resources/read",
		Params:  models.MCPResourcesReadParams{},
	}

	response := server.handleResourcesRead(readMessage)
	if response.Error == nil {
		t.Error("Expected error for missing URI parameter")
	}
	if response.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response.Error.Code)
	}

	// Test invalid URI scheme
	readMessage2 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-read-invalid-scheme",
		Method:  "resources/read",
		Params: models.MCPResourcesReadParams{
			URI: "invalid://guidelines/test",
		},
	}

	response2 := server.handleResourcesRead(readMessage2)
	if response2.Error == nil {
		t.Error("Expected error for invalid URI scheme")
	}
	if response2.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response2.Error.Code)
	}

	// Test unsupported category
	readMessage3 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-read-invalid-category",
		Method:  "resources/read",
		Params: models.MCPResourcesReadParams{
			URI: "architecture://invalid/test",
		},
	}

	response3 := server.handleResourcesRead(readMessage3)
	if response3.Error == nil {
		t.Error("Expected error for unsupported category")
	}
	if response3.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response3.Error.Code)
	}

	// Test resource not found
	readMessage4 := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-read-not-found",
		Method:  "resources/read",
		Params: models.MCPResourcesReadParams{
			URI: "architecture://guidelines/nonexistent",
		},
	}

	response4 := server.handleResourcesRead(readMessage4)
	if response4.Error == nil {
		t.Error("Expected error for resource not found")
	}
	if response4.Error.Code != -32603 {
		t.Errorf("Expected error code -32603, got %d", response4.Error.Code)
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	server := NewMCPServer()

	unknownMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-unknown",
		Method:  "unknown/method",
	}

	response := server.handleMessage(unknownMessage)

	if response == nil {
		t.Fatal("handleMessage() returned nil for unknown method")
	}

	if response.Error == nil {
		t.Error("Expected error for unknown method")
	}

	if response.Error.Code != -32601 {
		t.Errorf("Expected error code -32601 (Method not found), got %d", response.Error.Code)
	}

	if response.Error.Message != "Method not found" {
		t.Errorf("Expected error message 'Method not found', got '%s'", response.Error.Message)
	}
}

func TestJSONRPCMessageParsing(t *testing.T) {
	server := NewMCPServer()

	// Test valid JSON-RPC message
	validJSON := `{"jsonrpc":"2.0","id":"test","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`

	reader := strings.NewReader(validJSON)
	writer := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should process one message and then return nil when EOF is reached
	err := server.processMessages(ctx, reader, writer)
	if err != nil {
		t.Errorf("Expected nil error (EOF), got %v", err)
	}

	// Check that a response was written
	if writer.Len() == 0 {
		t.Error("Expected response to be written")
	}

	// Parse the response
	var response models.MCPMessage
	if err := json.Unmarshal(writer.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != "test" {
		t.Errorf("Expected ID 'test', got '%v'", response.ID)
	}
}

func TestServerStartupAndShutdown(t *testing.T) {
	server := NewMCPServer()

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		// Use empty reader to simulate no input
		reader := strings.NewReader("")
		writer := &bytes.Buffer{}
		errChan <- server.processMessages(ctx, reader, writer)
	}()

	// Give server a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for server to stop
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error during shutdown: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Server did not shut down within timeout")
	}

	// Test explicit shutdown
	shutdownErr := server.Shutdown(context.Background())
	if shutdownErr != nil {
		t.Errorf("Unexpected error during explicit shutdown: %v", shutdownErr)
	}
}

func TestMCPInitializationFlow(t *testing.T) {
	server := NewMCPServer()

	// Step 1: Send initialize request
	initJSON := `{"jsonrpc":"2.0","id":"init-1","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

	reader := strings.NewReader(initJSON)
	writer := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Process initialize message
	err := server.processMessages(ctx, reader, writer)
	if err != nil {
		t.Errorf("Expected nil error (EOF), got %v", err)
	}

	// Verify initialize response
	var initResponse models.MCPMessage
	if err := json.Unmarshal(writer.Bytes(), &initResponse); err != nil {
		t.Fatalf("Failed to parse initialize response: %v", err)
	}

	if initResponse.Error != nil {
		t.Errorf("Initialize failed with error: %v", initResponse.Error)
	}

	result, ok := initResponse.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected initialize result to be a map")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("Expected protocol version '2024-11-05', got '%v'", result["protocolVersion"])
	}

	// Step 2: Send initialized notification
	if server.initialized {
		t.Error("Server should not be initialized before notification")
	}

	initializedJSON := `{"jsonrpc":"2.0","method":"notifications/initialized"}`

	reader2 := strings.NewReader(initializedJSON)
	writer2 := &bytes.Buffer{}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	// Process initialized notification
	err2 := server.processMessages(ctx2, reader2, writer2)
	if err2 != nil {
		t.Errorf("Expected nil error (EOF), got %v", err2)
	}

	// Verify server is now initialized
	if !server.initialized {
		t.Error("Server should be initialized after notification")
	}

	// No response should be written for notification
	if writer2.Len() > 0 {
		t.Error("No response should be written for notification")
	}
}

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

func TestPromptsCapabilityInInitialize(t *testing.T) {
	server := NewMCPServer()

	initMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-init-prompts",
		Method:  "initialize",
		Params: models.MCPInitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: models.MCPClientInfo{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	}

	response := server.handleInitialize(initMessage)

	if response == nil {
		t.Fatal("handleInitialize() returned nil")
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}

	// Verify result structure
	result, ok := response.Result.(models.MCPInitializeResult)
	if !ok {
		t.Fatal("Expected result to be MCPInitializeResult")
	}

	// Verify prompts capability is present
	if result.Capabilities.Prompts == nil {
		t.Error("Expected prompts capability to be present in initialization response")
	}

	// Verify resources capability is also present (existing functionality)
	if result.Capabilities.Resources == nil {
		t.Error("Expected resources capability to be present in initialization response")
	}
}
