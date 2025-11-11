package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
)

func TestDocumentationSystemIntegration(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "mcp_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mcp/resources subdirectories
	guidelinesDir := filepath.Join(tempDir, "mcp", "resources", "guidelines")
	patternsDir := filepath.Join(tempDir, "mcp", "resources", "patterns")
	adrDir := filepath.Join(tempDir, "mcp", "resources", "adr")

	for _, dir := range []string{guidelinesDir, patternsDir, adrDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create test documents
	testDocs := map[string]string{
		filepath.Join(guidelinesDir, "api-design.md"): "# API Design Guidelines\n\nThis is a guideline document.",
		filepath.Join(patternsDir, "repository.md"):   "# Repository Pattern\n\nThis is a pattern document.",
		filepath.Join(adrDir, "adr-001.md"):           "# ADR-001: Use Go for Backend\n\nThis is an ADR document.",
	}

	for path, content := range testDocs {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", path, err)
		}
	}

	// Change to temp directory for testing
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create MCP server
	server := NewMCPServer()

	// Initialize documentation system
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = server.initializeDocumentationSystem(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize documentation system: %v", err)
	}

	// Verify initial cache population
	if server.cache.Size() != 3 {
		t.Errorf("Expected 3 documents in cache, got %d", server.cache.Size())
	}

	// Verify documents are categorized correctly
	guidelines := server.cache.GetByCategory("guideline")
	patterns := server.cache.GetByCategory("pattern")
	adrs := server.cache.GetByCategory("adr")

	if len(guidelines) != 1 {
		t.Errorf("Expected 1 guideline document, got %d", len(guidelines))
	}
	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern document, got %d", len(patterns))
	}
	if len(adrs) != 1 {
		t.Errorf("Expected 1 ADR document, got %d", len(adrs))
	}

	// Test file modification through monitor integration
	if server.monitor != nil {
		// Start cache refresh coordinator
		go server.cacheRefreshCoordinator(ctx)

		// Modify a file
		newContent := "# Updated API Design Guidelines\n\nThis is updated content."
		apiDesignPath := filepath.Join(guidelinesDir, "api-design.md")

		err = os.WriteFile(apiDesignPath, []byte(newContent), 0644)
		if err != nil {
			t.Fatalf("Failed to update test file: %v", err)
		}

		// Wait for file system event processing (debounced)
		time.Sleep(1500 * time.Millisecond) // Wait longer than debounce delay

		// Verify cache was updated
		relPath, _ := filepath.Rel(tempDir, apiDesignPath)
		doc, err := server.cache.Get(relPath)
		if err != nil {
			t.Errorf("Failed to get updated document from cache: %v", err)
		} else if doc.Content.RawContent != newContent {
			t.Errorf("Document content was not updated in cache. Expected: %s, Got: %s", newContent, doc.Content.RawContent)
		}

		// Test file deletion
		deleteTestPath := filepath.Join(patternsDir, "repository.md")
		err = os.Remove(deleteTestPath)
		if err != nil {
			t.Fatalf("Failed to delete test file: %v", err)
		}

		// Wait for file system event processing (debounced)
		time.Sleep(1500 * time.Millisecond) // Wait longer than debounce delay

		// Verify document was removed from cache
		relDeletePath, _ := filepath.Rel(tempDir, deleteTestPath)
		_, err = server.cache.Get(relDeletePath)
		if err == nil {
			t.Errorf("Expected document to be removed from cache, but it still exists")
		}

		// Verify cache size decreased
		if server.cache.Size() != 2 {
			t.Errorf("Expected 2 documents in cache after deletion, got %d", server.cache.Size())
		}
	}

	// Test graceful shutdown
	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}

func TestCacheRefreshCoordinator(t *testing.T) {
	server := NewMCPServer()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start coordinator
	go server.cacheRefreshCoordinator(ctx)

	// Test that coordinator can receive events
	testEvent := models.FileEvent{Type: "delete", Path: config.PatternsPath + "/test2.md"}

	select {
	case server.refreshChan <- testEvent:
		// Event sent successfully
	case <-time.After(1 * time.Second):
		t.Errorf("Failed to send event to refresh channel")
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Test coordinator shutdown
	cancel()
	time.Sleep(100 * time.Millisecond) // Give coordinator time to exit
}

func TestGetCategoryFromPath(t *testing.T) {
	server := NewMCPServer()

	tests := []struct {
		path     string
		expected string
	}{
		{config.GuidelinesPath + "/api.md", "guideline"},
		{config.PatternsPath + "/repository.md", "pattern"},
		{config.ADRPath + "/adr-001.md", "adr"},
		{"some/other/path.md", "unknown"},
		{strings.ToUpper(config.GuidelinesPath) + "/API.MD", "guideline"}, // Test case insensitive
	}

	for _, test := range tests {
		result := server.getCategoryFromPath(test.path)
		if result != test.expected {
			t.Errorf("getCategoryFromPath(%s) = %s, expected %s", test.path, result, test.expected)
		}
	}
}

// TestMCPResourceMethodsIntegration tests the complete MCP resource functionality
func TestMCPResourceMethodsIntegration(t *testing.T) {
	// Create temporary directory structure with test documents
	tempDir, err := os.MkdirTemp("", "mcp_resource_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mcp/resources subdirectories
	guidelinesDir := filepath.Join(tempDir, "mcp", "resources", "guidelines")
	patternsDir := filepath.Join(tempDir, "mcp", "resources", "patterns")
	adrDir := filepath.Join(tempDir, "mcp", "resources", "adr")

	for _, dir := range []string{guidelinesDir, patternsDir, adrDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create comprehensive test documents
	testDocs := map[string]string{
		filepath.Join(guidelinesDir, "api-design.md"): `# API Design Guidelines

This document outlines the API design principles and best practices.

## REST Principles
- Use HTTP methods appropriately
- Design resource-oriented URLs
- Return appropriate status codes`,

		filepath.Join(guidelinesDir, "security.md"): `# Security Guidelines

Security considerations for all applications.

## Authentication
- Use OAuth 2.0 for API authentication
- Implement proper session management`,

		filepath.Join(patternsDir, "repository.md"): `# Repository Pattern

The repository pattern encapsulates data access logic.

## Implementation
- Define repository interfaces
- Implement concrete repositories`,

		filepath.Join(patternsDir, "factory.md"): `# Factory Pattern

The factory pattern creates objects without specifying exact classes.

## Benefits
- Loose coupling
- Easier testing`,

		filepath.Join(adrDir, "adr-001.md"): `# ADR-001: Use Go for Backend Services

## Status
Accepted

## Context
We need to choose a backend language for our microservices.

## Decision
We will use Go for all new backend services.

## Consequences
- Better performance
- Strong typing`,

		filepath.Join(adrDir, "adr-002.md"): `# ADR-002: Use PostgreSQL for Primary Database

## Status
Accepted

## Context
We need a reliable database for our application data.

## Decision
We will use PostgreSQL as our primary database.`,
	}

	for path, content := range testDocs {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", path, err)
		}
	}

	// Change to temp directory for testing
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create and initialize MCP server
	server := NewMCPServer()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = server.initializeDocumentationSystem(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize documentation system: %v", err)
	}

	// Test 1: resources/list method integration
	t.Run("ResourcesListIntegration", func(t *testing.T) {
		listMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-list-integration",
			Method:  "resources/list",
		}

		response := server.handleResourcesList(listMessage)

		// Verify response structure
		if response == nil {
			t.Fatal("handleResourcesList() returned nil")
		}

		if response.Error != nil {
			t.Fatalf("Expected no error, got %v", response.Error)
		}

		result, ok := response.Result.(models.MCPResourcesListResult)
		if !ok {
			t.Fatal("Expected result to be MCPResourcesListResult")
		}

		// Should return all 6 test documents
		if len(result.Resources) != 6 {
			t.Errorf("Expected 6 resources, got %d", len(result.Resources))
		}

		// Verify resource categories and URIs
		categoryCount := make(map[string]int)
		uriPatterns := make(map[string]bool)

		for _, resource := range result.Resources {
			// Verify required fields
			if resource.URI == "" {
				t.Error("Resource URI should not be empty")
			}
			if resource.Name == "" {
				t.Error("Resource name should not be empty")
			}
			if resource.MimeType != "text/markdown" {
				t.Errorf("Expected mimeType 'text/markdown', got '%s'", resource.MimeType)
			}

			// Count categories
			if category, exists := resource.Annotations["category"]; exists {
				categoryCount[category]++
			}

			// Track URI patterns
			if strings.HasPrefix(resource.URI, "architecture://guidelines/") {
				uriPatterns["guidelines"] = true
			} else if strings.HasPrefix(resource.URI, "architecture://patterns/") {
				uriPatterns["patterns"] = true
			} else if strings.HasPrefix(resource.URI, "architecture://adr/") {
				uriPatterns["adr"] = true
			}
		}

		// Verify category distribution
		if categoryCount["guideline"] != 2 {
			t.Errorf("Expected 2 guideline resources, got %d", categoryCount["guideline"])
		}
		if categoryCount["pattern"] != 2 {
			t.Errorf("Expected 2 pattern resources, got %d", categoryCount["pattern"])
		}
		if categoryCount["adr"] != 2 {
			t.Errorf("Expected 2 ADR resources, got %d", categoryCount["adr"])
		}

		// Verify all URI patterns are present
		expectedPatterns := []string{"guidelines", "patterns", "adr"}
		for _, pattern := range expectedPatterns {
			if !uriPatterns[pattern] {
				t.Errorf("Expected to find %s URI pattern", pattern)
			}
		}
	})

	// Test 2: resources/read method integration for guidelines
	t.Run("ResourcesReadGuidelinesIntegration", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-read-guideline",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://guidelines/api-design",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response == nil {
			t.Fatal("handleResourcesRead() returned nil")
		}

		if response.Error != nil {
			t.Fatalf("Expected no error, got %v", response.Error)
		}

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

		// Verify content contains expected text
		if !strings.Contains(content.Text, "API Design Guidelines") {
			t.Error("Content should contain 'API Design Guidelines'")
		}
		if !strings.Contains(content.Text, "REST Principles") {
			t.Error("Content should contain 'REST Principles'")
		}
	})

	// Test 3: resources/read method integration for patterns
	t.Run("ResourcesReadPatternsIntegration", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-read-pattern",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://patterns/repository",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response == nil {
			t.Fatal("handleResourcesRead() returned nil")
		}

		if response.Error != nil {
			t.Fatalf("Expected no error, got %v", response.Error)
		}

		result, ok := response.Result.(models.MCPResourcesReadResult)
		if !ok {
			t.Fatal("Expected result to be MCPResourcesReadResult")
		}

		content := result.Contents[0]
		if content.URI != "architecture://patterns/repository" {
			t.Errorf("Expected URI 'architecture://patterns/repository', got '%s'", content.URI)
		}

		// Verify content contains expected text
		if !strings.Contains(content.Text, "Repository Pattern") {
			t.Error("Content should contain 'Repository Pattern'")
		}
		if !strings.Contains(content.Text, "Implementation") {
			t.Error("Content should contain 'Implementation'")
		}
	})

	// Test 4: resources/read method integration for ADRs
	t.Run("ResourcesReadADRIntegration", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-read-adr",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://adr/001",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response == nil {
			t.Fatal("handleResourcesRead() returned nil")
		}

		if response.Error != nil {
			t.Fatalf("Expected no error, got %v", response.Error)
		}

		result, ok := response.Result.(models.MCPResourcesReadResult)
		if !ok {
			t.Fatal("Expected result to be MCPResourcesReadResult")
		}

		content := result.Contents[0]
		if content.URI != "architecture://adr/001" {
			t.Errorf("Expected URI 'architecture://adr/001', got '%s'", content.URI)
		}

		// Verify content contains expected ADR structure
		if !strings.Contains(content.Text, "ADR-001") {
			t.Error("Content should contain 'ADR-001'")
		}
		if !strings.Contains(content.Text, "Status") {
			t.Error("Content should contain 'Status'")
		}
		if !strings.Contains(content.Text, "Context") {
			t.Error("Content should contain 'Context'")
		}
		if !strings.Contains(content.Text, "Decision") {
			t.Error("Content should contain 'Decision'")
		}
	})
}

// TestMCPResourceErrorScenariosIntegration tests error handling in resource methods
func TestMCPResourceErrorScenariosIntegration(t *testing.T) {
	server := NewMCPServer()

	// Test 1: Invalid URI schemes
	t.Run("InvalidURIScheme", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-invalid-scheme",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "invalid://guidelines/test",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error == nil {
			t.Error("Expected error for invalid URI scheme")
		}
		if response.Error.Code != -32602 {
			t.Errorf("Expected error code -32602, got %d", response.Error.Code)
		}
		if !strings.Contains(response.Error.Message, "Invalid resource URI") {
			t.Errorf("Expected error message about invalid URI, got '%s'", response.Error.Message)
		}
	})

	// Test 2: Unsupported categories
	t.Run("UnsupportedCategory", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-unsupported-category",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://unsupported/test",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error == nil {
			t.Error("Expected error for unsupported category")
		}
		if response.Error.Code != -32602 {
			t.Errorf("Expected error code -32602, got %d", response.Error.Code)
		}
		if !strings.Contains(response.Error.Message, "unsupported resource category") {
			t.Errorf("Expected error message about unsupported category, got '%s'", response.Error.Message)
		}
	})

	// Test 3: Missing URI parameter
	t.Run("MissingURIParameter", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-missing-uri",
			Method:  "resources/read",
			Params:  models.MCPResourcesReadParams{}, // Empty params
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error == nil {
			t.Error("Expected error for missing URI parameter")
		}
		if response.Error.Code != -32602 {
			t.Errorf("Expected error code -32602, got %d", response.Error.Code)
		}
		if !strings.Contains(response.Error.Message, "Missing required parameter: uri") {
			t.Errorf("Expected error message about missing URI, got '%s'", response.Error.Message)
		}
	})

	// Test 4: Resource not found
	t.Run("ResourceNotFound", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "test-not-found",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://guidelines/nonexistent-document",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error == nil {
			t.Error("Expected error for resource not found")
		}
		if response.Error.Code != -32603 {
			t.Errorf("Expected error code -32603, got %d", response.Error.Code)
		}
		if !strings.Contains(response.Error.Message, "Resource not found") {
			t.Errorf("Expected error message about resource not found, got '%s'", response.Error.Message)
		}
	})

	// Test 5: Malformed URI format
	t.Run("MalformedURIFormat", func(t *testing.T) {
		testCases := []string{
			"architecture://",                 // Missing category and path
			"architecture://guidelines",       // Missing path
			"architecture:///guidelines/test", // Extra slash
			"architecture://guidelines//test", // Double slash in path
		}

		for i, uri := range testCases {
			t.Run(fmt.Sprintf("MalformedURI_%d", i), func(t *testing.T) {
				readMessage := &models.MCPMessage{
					JSONRPC: "2.0",
					ID:      fmt.Sprintf("test-malformed-%d", i),
					Method:  "resources/read",
					Params: models.MCPResourcesReadParams{
						URI: uri,
					},
				}

				response := server.handleResourcesRead(readMessage)

				if response.Error == nil {
					t.Errorf("Expected error for malformed URI: %s", uri)
				}
				if response.Error.Code != -32602 {
					t.Errorf("Expected error code -32602, got %d", response.Error.Code)
				}
			})
		}
	})
}

// TestMCPProtocolComplianceIntegration tests MCP protocol compliance
func TestMCPProtocolComplianceIntegration(t *testing.T) {
	server := NewMCPServer()

	// Test 1: JSON-RPC 2.0 compliance
	t.Run("JSONRPCCompliance", func(t *testing.T) {
		listMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "compliance-test",
			Method:  "resources/list",
		}

		response := server.handleResourcesList(listMessage)

		// Verify JSON-RPC 2.0 compliance
		if response.JSONRPC != "2.0" {
			t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
		}

		if response.ID != "compliance-test" {
			t.Errorf("Expected ID 'compliance-test', got '%v'", response.ID)
		}

		// Should have either result or error, but not both
		if response.Result != nil && response.Error != nil {
			t.Error("Response should not have both result and error")
		}

		if response.Result == nil && response.Error == nil {
			t.Error("Response should have either result or error")
		}
	})

	// Test 2: MCP resource structure compliance
	t.Run("MCPResourceStructureCompliance", func(t *testing.T) {
		// Add a test document to cache
		testDoc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:        "Test Document",
				Category:     "guideline",
				Path:         config.GuidelinesPath + "/test.md",
				LastModified: time.Now(),
				Size:         100,
				Checksum:     "test123",
			},
			Content: models.DocumentContent{
				RawContent: "# Test Document\nThis is a test.",
			},
		}
		server.cache.Set(testDoc.Metadata.Path, testDoc)

		listMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "structure-test",
			Method:  "resources/list",
		}

		response := server.handleResourcesList(listMessage)

		result, ok := response.Result.(models.MCPResourcesListResult)
		if !ok {
			t.Fatal("Expected result to be MCPResourcesListResult")
		}

		if len(result.Resources) == 0 {
			t.Fatal("Expected at least one resource")
		}

		resource := result.Resources[0]

		// Verify required MCP resource fields
		if resource.URI == "" {
			t.Error("Resource URI is required")
		}
		if resource.Name == "" {
			t.Error("Resource name is required")
		}
		if resource.MimeType == "" {
			t.Error("Resource mimeType is required")
		}

		// Verify URI format compliance
		if !strings.HasPrefix(resource.URI, "architecture://") {
			t.Errorf("Resource URI should start with 'architecture://', got '%s'", resource.URI)
		}

		// Verify annotations structure
		if resource.Annotations == nil {
			t.Error("Resource annotations should not be nil")
		}

		requiredAnnotations := []string{"category", "path", "lastModified", "size", "checksum"}
		for _, annotation := range requiredAnnotations {
			if _, exists := resource.Annotations[annotation]; !exists {
				t.Errorf("Resource should have '%s' annotation", annotation)
			}
		}
	})

	// Test 3: MCP resource content structure compliance
	t.Run("MCPResourceContentCompliance", func(t *testing.T) {
		// Add a test document to cache
		testDoc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:        "Content Test",
				Category:     "pattern",
				Path:         config.PatternsPath + "/content-test.md",
				LastModified: time.Now(),
				Size:         200,
				Checksum:     "content123",
			},
			Content: models.DocumentContent{
				RawContent: "# Content Test\nThis is content for testing.",
			},
		}
		server.cache.Set(testDoc.Metadata.Path, testDoc)

		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "content-compliance-test",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://patterns/content-test",
			},
		}

		response := server.handleResourcesRead(readMessage)

		result, ok := response.Result.(models.MCPResourcesReadResult)
		if !ok {
			t.Fatal("Expected result to be MCPResourcesReadResult")
		}

		if len(result.Contents) != 1 {
			t.Errorf("Expected 1 content item, got %d", len(result.Contents))
		}

		content := result.Contents[0]

		// Verify required MCP resource content fields
		if content.URI == "" {
			t.Error("Resource content URI is required")
		}
		if content.MimeType == "" {
			t.Error("Resource content mimeType is required")
		}

		// Should have either text or blob, but not both
		if content.Text != "" && content.Blob != "" {
			t.Error("Resource content should not have both text and blob")
		}
		if content.Text == "" && content.Blob == "" {
			t.Error("Resource content should have either text or blob")
		}

		// For markdown content, should use text field
		if content.MimeType == "text/markdown" && content.Text == "" {
			t.Error("Markdown content should use text field")
		}
	})

	// Test 4: Error response compliance
	t.Run("ErrorResponseCompliance", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "error-compliance-test",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "invalid://test/path",
			},
		}

		response := server.handleResourcesRead(readMessage)

		// Verify error response structure
		if response.Error == nil {
			t.Fatal("Expected error response")
		}

		// Verify required error fields
		if response.Error.Code == 0 {
			t.Error("Error code is required")
		}
		if response.Error.Message == "" {
			t.Error("Error message is required")
		}

		// Verify JSON-RPC 2.0 compliance for errors
		if response.JSONRPC != "2.0" {
			t.Errorf("Expected JSONRPC '2.0' in error response, got '%s'", response.JSONRPC)
		}
		if response.ID != "error-compliance-test" {
			t.Errorf("Expected ID 'error-compliance-test' in error response, got '%v'", response.ID)
		}

		// Should not have result field in error response
		if response.Result != nil {
			t.Error("Error response should not have result field")
		}
	})
}

// TestResourceURIParsingIntegration tests URI parsing functionality
func TestResourceURIParsingIntegration(t *testing.T) {
	server := NewMCPServer()

	testCases := []struct {
		name             string
		uri              string
		expectedCategory string
		expectedPath     string
		expectError      bool
	}{
		{
			name:             "ValidGuidelineURI",
			uri:              "architecture://guidelines/api-design",
			expectedCategory: "guideline",
			expectedPath:     "api-design",
			expectError:      false,
		},
		{
			name:             "ValidPatternURI",
			uri:              "architecture://patterns/repository",
			expectedCategory: "pattern",
			expectedPath:     "repository",
			expectError:      false,
		},
		{
			name:             "ValidADRURI",
			uri:              "architecture://adr/001",
			expectedCategory: "adr",
			expectedPath:     "001",
			expectError:      false,
		},
		{
			name:             "ValidNestedPath",
			uri:              "architecture://guidelines/security/authentication",
			expectedCategory: "guideline",
			expectedPath:     "security/authentication",
			expectError:      false,
		},
		{
			name:        "InvalidScheme",
			uri:         "invalid://guidelines/test",
			expectError: true,
		},
		{
			name:        "InvalidCategory",
			uri:         "architecture://invalid/test",
			expectError: true,
		},
		{
			name:        "MissingPath",
			uri:         "architecture://guidelines",
			expectError: true,
		},
		{
			name:        "EmptyURI",
			uri:         "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			category, path, err := server.parseResourceURI(tc.uri)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for URI '%s', but got none", tc.uri)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for URI '%s': %v", tc.uri, err)
				}

				if category != tc.expectedCategory {
					t.Errorf("Expected category '%s', got '%s'", tc.expectedCategory, category)
				}

				if path != tc.expectedPath {
					t.Errorf("Expected path '%s', got '%s'", tc.expectedPath, path)
				}
			}
		})
	}
}

// TestResourceContentRetrievalIntegration tests content retrieval with various scenarios
func TestResourceContentRetrievalIntegration(t *testing.T) {
	// Create temporary directory with test documents
	tempDir, err := os.MkdirTemp("", "mcp_content_retrieval_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test documents with different content types
	testDocs := map[string]string{
		filepath.Join(tempDir, "mcp", "resources", "guidelines", "simple.md"): `# Simple Document
This is a simple document with basic content.`,

		filepath.Join(tempDir, "mcp", "resources", "patterns", "complex.md"): `# Complex Pattern Document

## Overview
This is a more complex document with multiple sections.

### Code Examples
` + "```go\nfunc example() {\n    return \"test\"\n}\n```" + `

### Lists
- Item 1
- Item 2
- Item 3

### Tables
| Column 1 | Column 2 |
|----------|----------|
| Value 1  | Value 2  |`,

		filepath.Join(tempDir, "mcp", "resources", "adr", "adr-special-chars.md"): `# ADR with Special Characters

This document contains special characters: áéíóú, ñ, ç, ü

## Symbols
- © Copyright
- ® Registered
- ™ Trademark
- € Euro
- £ Pound`,
	}

	// Create directories and files
	for path, content := range testDocs {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", path, err)
		}
	}

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Initialize server
	server := NewMCPServer()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = server.initializeDocumentationSystem(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize documentation system: %v", err)
	}

	// Test 1: Simple content retrieval
	t.Run("SimpleContentRetrieval", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "simple-content",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://guidelines/simple",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error != nil {
			t.Fatalf("Unexpected error: %v", response.Error)
		}

		result := response.Result.(models.MCPResourcesReadResult)
		content := result.Contents[0]

		if !strings.Contains(content.Text, "Simple Document") {
			t.Error("Content should contain 'Simple Document'")
		}
		if !strings.Contains(content.Text, "basic content") {
			t.Error("Content should contain 'basic content'")
		}
	})

	// Test 2: Complex content with code blocks and formatting
	t.Run("ComplexContentRetrieval", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "complex-content",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://patterns/complex",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error != nil {
			t.Fatalf("Unexpected error: %v", response.Error)
		}

		result := response.Result.(models.MCPResourcesReadResult)
		content := result.Contents[0]

		// Verify complex content elements are preserved
		expectedElements := []string{
			"Complex Pattern Document",
			"Overview",
			"Code Examples",
			"```go",
			"func example()",
			"Lists",
			"- Item 1",
			"Tables",
			"| Column 1 | Column 2 |",
		}

		for _, element := range expectedElements {
			if !strings.Contains(content.Text, element) {
				t.Errorf("Content should contain '%s'", element)
			}
		}
	})

	// Test 3: Content with special characters
	t.Run("SpecialCharactersContent", func(t *testing.T) {
		readMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "special-chars",
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: "architecture://adr/special-chars",
			},
		}

		response := server.handleResourcesRead(readMessage)

		if response.Error != nil {
			t.Fatalf("Unexpected error: %v", response.Error)
		}

		result := response.Result.(models.MCPResourcesReadResult)
		content := result.Contents[0]

		// Verify special characters are preserved
		specialChars := []string{"áéíóú", "ñ", "ç", "ü", "©", "®", "™", "€", "£"}
		for _, char := range specialChars {
			if !strings.Contains(content.Text, char) {
				t.Errorf("Content should contain special character '%s'", char)
			}
		}
	})

	// Test 4: Content length and integrity
	t.Run("ContentIntegrity", func(t *testing.T) {
		// Get all resources and verify their content integrity
		listMessage := &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      "integrity-list",
			Method:  "resources/list",
		}

		listResponse := server.handleResourcesList(listMessage)
		listResult := listResponse.Result.(models.MCPResourcesListResult)

		for _, resource := range listResult.Resources {
			readMessage := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "integrity-read",
				Method:  "resources/read",
				Params: models.MCPResourcesReadParams{
					URI: resource.URI,
				},
			}

			readResponse := server.handleResourcesRead(readMessage)

			if readResponse.Error != nil {
				t.Errorf("Failed to read resource %s: %v", resource.URI, readResponse.Error)
				continue
			}

			readResult := readResponse.Result.(models.MCPResourcesReadResult)
			content := readResult.Contents[0]

			// Verify content is not empty
			if content.Text == "" {
				t.Errorf("Content for resource %s should not be empty", resource.URI)
			}

			// Verify content matches URI
			if content.URI != resource.URI {
				t.Errorf("Content URI %s does not match resource URI %s", content.URI, resource.URI)
			}

			// Verify content has proper structure (starts with #)
			if !strings.HasPrefix(strings.TrimSpace(content.Text), "#") {
				t.Errorf("Markdown content for %s should start with header", resource.URI)
			}
		}
	})
}
