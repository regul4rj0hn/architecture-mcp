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
