package server

import (
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
)

// setupTestCacheDocuments prepares test documents and adds them to the server cache
func setupTestCacheDocuments(t *testing.T, server *MCPServer) {
	t.Helper()

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
}

// validateResourceListBasics validates basic response structure and properties
func validateResourceListBasics(t *testing.T, response *models.MCPMessage, expectedID string) models.MCPResourcesListResult {
	t.Helper()

	if response == nil {
		t.Fatal("handleResourcesList() returned nil")
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

	result, ok := response.Result.(models.MCPResourcesListResult)
	if !ok {
		t.Fatal("Expected result to be MCPResourcesListResult")
	}

	if len(result.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d resources", len(result.Resources))
	}

	return result
}

// validateResourceProperties validates individual resource properties
func validateResourceProperties(t *testing.T, resources []models.MCPResource) {
	t.Helper()

	for _, resource := range resources {
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
}

// validateResourceURIs validates URI patterns for different resource categories
func validateResourceURIs(t *testing.T, resources []models.MCPResource) {
	t.Helper()

	foundGuideline := false
	foundPattern := false
	for _, resource := range resources {
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

func TestHandleResourcesList(t *testing.T) {
	server := NewMCPServer()
	setupTestCacheDocuments(t, server)

	listMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-list",
		Method:  "resources/list",
	}

	response := server.handleResourcesList(listMessage)

	result := validateResourceListBasics(t, response, "test-list")
	validateResourceProperties(t, result.Resources)
	validateResourceURIs(t, result.Resources)
}

// validateResourceReadResponse validates a successful resource read response
func validateResourceReadResponse(t *testing.T, response *models.MCPMessage, expectedID, expectedURI, expectedContent string) {
	t.Helper()

	if response == nil {
		t.Fatal("handleResourcesRead() returned nil")
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

	result, ok := response.Result.(models.MCPResourcesReadResult)
	if !ok {
		t.Fatal("Expected result to be MCPResourcesReadResult")
	}

	if len(result.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Contents))
	}

	content := result.Contents[0]
	if content.URI != expectedURI {
		t.Errorf("Expected URI '%s', got '%s'", expectedURI, content.URI)
	}

	if content.MimeType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", content.MimeType)
	}

	if content.Text != expectedContent {
		t.Errorf("Expected content to match document raw content")
	}
}

func TestHandleResourcesRead(t *testing.T) {
	server := NewMCPServer()

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

	readMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-read",
		Method:  "resources/read",
		Params: models.MCPResourcesReadParams{
			URI: "architecture://guidelines/api-design",
		},
	}

	response := server.handleResourcesRead(readMessage)
	validateResourceReadResponse(t, response, "test-read", "architecture://guidelines/api-design", testDoc.Content.RawContent)
}

func TestHandleResourcesReadErrors(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		expectedCode int
	}{
		{
			name:         "missing URI parameter",
			uri:          "",
			expectedCode: -32602,
		},
		{
			name:         "invalid URI scheme",
			uri:          "invalid://guidelines/test",
			expectedCode: -32602,
		},
		{
			name:         "unsupported category",
			uri:          "architecture://invalid/test",
			expectedCode: -32602,
		},
		{
			name:         "resource not found",
			uri:          "architecture://guidelines/nonexistent",
			expectedCode: -32603,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMCPServer()

			readMessage := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-read-error",
				Method:  "resources/read",
				Params: models.MCPResourcesReadParams{
					URI: tt.uri,
				},
			}

			response := server.handleResourcesRead(readMessage)
			if response.Error == nil {
				t.Errorf("Expected error for %s", tt.name)
			}
			if response.Error != nil && response.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, response.Error.Code)
			}
		})
	}
}
