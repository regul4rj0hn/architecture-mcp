package server

import (
	"testing"

	"mcp-architecture-service/internal/models"
)

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

func TestHandleCompletionCompleteRouting(t *testing.T) {
	server := NewMCPServer()

	// Test that completion/complete method is routed correctly
	completionMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-completion",
		Method:  "completion/complete",
		Params: map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "test-prompt",
			},
			"argument": map[string]interface{}{
				"name":  "test_arg",
				"value": "test",
			},
		},
	}

	response := server.handleMessage(completionMessage)

	if response == nil {
		t.Fatal("handleMessage() returned nil for completion/complete method")
	}

	// We expect either a valid response or an error (e.g., prompt not found)
	// but NOT a "Method not found" error
	if response.Error != nil && response.Error.Code == -32601 {
		t.Errorf("completion/complete method not routed correctly, got 'Method not found' error")
	}
}

func TestCompletionCapabilityAdvertised(t *testing.T) {
	server := NewMCPServer()

	// Verify that completion capability is set in server capabilities
	if server.capabilities.Completion == nil {
		t.Fatal("Completion capability is nil, should be initialized")
	}

	if !server.capabilities.Completion.ArgumentCompletions {
		t.Error("ArgumentCompletions should be true")
	}
}
