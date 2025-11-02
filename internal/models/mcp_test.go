package models

import (
	"encoding/json"
	"testing"
)

func TestMCPMessageSerialization(t *testing.T) {
	// Test request message
	request := MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-123",
		Method:  "initialize",
		Params: MCPInitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{"test": true},
			ClientInfo: MCPClientInfo{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Deserialize back
	var parsed MCPMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if parsed.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", parsed.JSONRPC)
	}

	if parsed.ID != "test-123" {
		t.Errorf("Expected ID 'test-123', got '%v'", parsed.ID)
	}

	if parsed.Method != "initialize" {
		t.Errorf("Expected method 'initialize', got '%s'", parsed.Method)
	}
}

func TestMCPResponseSerialization(t *testing.T) {
	// Test response message
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-456",
		Result: MCPInitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: MCPCapabilities{
				Resources: &MCPResourceCapabilities{
					Subscribe:   false,
					ListChanged: false,
				},
			},
			ServerInfo: MCPServerInfo{
				Name:    "test-server",
				Version: "1.0.0",
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Deserialize back
	var parsed MCPMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if parsed.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", parsed.JSONRPC)
	}

	if parsed.ID != "test-456" {
		t.Errorf("Expected ID 'test-456', got '%v'", parsed.ID)
	}

	if parsed.Result == nil {
		t.Error("Expected result to be set")
	}
}

func TestMCPErrorSerialization(t *testing.T) {
	// Test error message
	errorMsg := MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-error",
		Error: &MCPError{
			Code:    -32601,
			Message: "Method not found",
			Data:    map[string]string{"method": "unknown"},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(errorMsg)
	if err != nil {
		t.Fatalf("Failed to marshal error: %v", err)
	}

	// Deserialize back
	var parsed MCPMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal error: %v", err)
	}

	if parsed.Error == nil {
		t.Fatal("Expected error to be set")
	}

	if parsed.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", parsed.Error.Code)
	}

	if parsed.Error.Message != "Method not found" {
		t.Errorf("Expected error message 'Method not found', got '%s'", parsed.Error.Message)
	}
}

func TestMCPResourceSerialization(t *testing.T) {
	resource := MCPResource{
		URI:         "architecture://guidelines/api-design",
		Name:        "API Design Guidelines",
		Description: "Guidelines for designing REST APIs",
		MimeType:    "text/markdown",
		Annotations: map[string]string{
			"category": "guideline",
			"version":  "1.0",
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("Failed to marshal resource: %v", err)
	}

	// Deserialize back
	var parsed MCPResource
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal resource: %v", err)
	}

	if parsed.URI != "architecture://guidelines/api-design" {
		t.Errorf("Expected URI 'architecture://guidelines/api-design', got '%s'", parsed.URI)
	}

	if parsed.Name != "API Design Guidelines" {
		t.Errorf("Expected name 'API Design Guidelines', got '%s'", parsed.Name)
	}

	if parsed.MimeType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", parsed.MimeType)
	}

	if len(parsed.Annotations) != 2 {
		t.Errorf("Expected 2 annotations, got %d", len(parsed.Annotations))
	}
}

func TestMCPResourceContentSerialization(t *testing.T) {
	content := MCPResourceContent{
		URI:      "architecture://guidelines/api-design",
		MimeType: "text/markdown",
		Text:     "# API Design Guidelines\n\nThis document contains...",
	}

	// Serialize to JSON
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal resource content: %v", err)
	}

	// Deserialize back
	var parsed MCPResourceContent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal resource content: %v", err)
	}

	if parsed.URI != "architecture://guidelines/api-design" {
		t.Errorf("Expected URI 'architecture://guidelines/api-design', got '%s'", parsed.URI)
	}

	if parsed.MimeType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", parsed.MimeType)
	}

	if parsed.Text != "# API Design Guidelines\n\nThis document contains..." {
		t.Errorf("Expected specific text content, got '%s'", parsed.Text)
	}
}

func TestMCPResourcesListResultSerialization(t *testing.T) {
	result := MCPResourcesListResult{
		Resources: []MCPResource{
			{
				URI:      "architecture://guidelines/api-design",
				Name:     "API Design Guidelines",
				MimeType: "text/markdown",
			},
			{
				URI:      "architecture://patterns/microservices",
				Name:     "Microservices Pattern",
				MimeType: "text/markdown",
			},
		},
		NextCursor: "cursor-123",
	}

	// Serialize to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal resources list result: %v", err)
	}

	// Deserialize back
	var parsed MCPResourcesListResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal resources list result: %v", err)
	}

	if len(parsed.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(parsed.Resources))
	}

	if parsed.NextCursor != "cursor-123" {
		t.Errorf("Expected cursor 'cursor-123', got '%s'", parsed.NextCursor)
	}

	if parsed.Resources[0].URI != "architecture://guidelines/api-design" {
		t.Errorf("Expected first resource URI 'architecture://guidelines/api-design', got '%s'", parsed.Resources[0].URI)
	}
}

func TestMCPResourcesReadResultSerialization(t *testing.T) {
	result := MCPResourcesReadResult{
		Contents: []MCPResourceContent{
			{
				URI:      "architecture://guidelines/api-design",
				MimeType: "text/markdown",
				Text:     "# API Design Guidelines\n\nContent here...",
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal resources read result: %v", err)
	}

	// Deserialize back
	var parsed MCPResourcesReadResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal resources read result: %v", err)
	}

	if len(parsed.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(parsed.Contents))
	}

	if parsed.Contents[0].URI != "architecture://guidelines/api-design" {
		t.Errorf("Expected URI 'architecture://guidelines/api-design', got '%s'", parsed.Contents[0].URI)
	}

	if parsed.Contents[0].MimeType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", parsed.Contents[0].MimeType)
	}
}

func TestJSONRPCProtocolCompliance(t *testing.T) {
	// Test that our messages comply with JSON-RPC 2.0 specification

	// Valid request
	requestJSON := `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`

	var request MCPMessage
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		t.Fatalf("Failed to parse valid JSON-RPC request: %v", err)
	}

	if request.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", request.JSONRPC)
	}

	// Valid response
	responseJSON := `{"jsonrpc":"2.0","id":"1","result":{"protocolVersion":"2024-11-05","capabilities":{"resources":{}},"serverInfo":{"name":"test","version":"1.0"}}}`

	var response MCPMessage
	if err := json.Unmarshal([]byte(responseJSON), &response); err != nil {
		t.Fatalf("Failed to parse valid JSON-RPC response: %v", err)
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	// Valid error response
	errorJSON := `{"jsonrpc":"2.0","id":"1","error":{"code":-32601,"message":"Method not found"}}`

	var errorResponse MCPMessage
	if err := json.Unmarshal([]byte(errorJSON), &errorResponse); err != nil {
		t.Fatalf("Failed to parse valid JSON-RPC error: %v", err)
	}

	if errorResponse.Error == nil {
		t.Error("Expected error to be set")
	}

	if errorResponse.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", errorResponse.Error.Code)
	}
}
