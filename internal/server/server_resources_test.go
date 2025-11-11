package server

import (
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
)

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
