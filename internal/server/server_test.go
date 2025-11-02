package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
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
			Category:     "guideline",
			Path:         "docs/guidelines/api-design.md",
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
			Category:     "pattern",
			Path:         "docs/patterns/repository.md",
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
			Category:     "guideline",
			Path:         "docs/guidelines/api-design.md",
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
