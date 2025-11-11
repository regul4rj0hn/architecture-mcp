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
