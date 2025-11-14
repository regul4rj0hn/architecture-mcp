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

// Test fixtures and setup helpers

type testEnv struct {
	tempDir       string
	server        *MCPServer
	ctx           context.Context
	cancel        context.CancelFunc
	guidelinesDir string
	patternsDir   string
	adrDir        string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "mcp_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	env := &testEnv{
		tempDir:       tempDir,
		guidelinesDir: filepath.Join(tempDir, "mcp", "resources", "guidelines"),
		patternsDir:   filepath.Join(tempDir, "mcp", "resources", "patterns"),
		adrDir:        filepath.Join(tempDir, "mcp", "resources", "adr"),
	}

	for _, dir := range []string{env.guidelinesDir, env.patternsDir, env.adrDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	return env
}

func (e *testEnv) cleanup(t *testing.T, originalDir string) {
	t.Helper()
	if e.cancel != nil {
		e.cancel()
	}
	os.Chdir(originalDir)
	os.RemoveAll(e.tempDir)
}

func (e *testEnv) initServer(t *testing.T) {
	t.Helper()

	originalDir, _ := os.Getwd()
	os.Chdir(e.tempDir)
	t.Cleanup(func() { e.cleanup(t, originalDir) })

	e.server = NewMCPServer()
	e.ctx, e.cancel = context.WithTimeout(context.Background(), 10*time.Second)

	if err := e.server.initializeDocumentationSystem(e.ctx); err != nil {
		t.Fatalf("Failed to initialize documentation system: %v", err)
	}
}

func (e *testEnv) writeTestDocs(t *testing.T, docs map[string]string) {
	t.Helper()
	for path, content := range docs {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", path, err)
		}
	}
}

// Standard test documents
func standardTestDocs(env *testEnv) map[string]string {
	return map[string]string{
		filepath.Join(env.guidelinesDir, "api-design.md"): `# API Design Guidelines

This document outlines the API design principles and best practices.

## REST Principles
- Use HTTP methods appropriately
- Design resource-oriented URLs
- Return appropriate status codes

## Authentication
- Use OAuth 2.0 for API authentication
- Implement proper session management`,

		filepath.Join(env.patternsDir, "repository-pattern.md"): `# Repository Pattern

The repository pattern encapsulates data access logic.

## Implementation
- Define repository interfaces
- Implement concrete repositories
- Use dependency injection`,

		filepath.Join(env.adrDir, "001-microservices-architecture.md"): `# ADR-001: Use Microservices Architecture

## Status
Accepted

## Context
We need to choose an architecture pattern for our system.

## Decision
We will use a microservices architecture.

## Consequences
- Better scalability
- Independent deployment`,
	}
}

// Validation helpers

func validateMCPResponse(t *testing.T, response *models.MCPMessage, expectError bool) {
	t.Helper()
	if response == nil {
		t.Fatal("Response is nil")
	}
	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}
	if expectError && response.Error == nil {
		t.Error("Expected error but got none")
	}
	if !expectError && response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
	}
}

func validateResourceList(t *testing.T, result models.MCPResourcesListResult, expectedCount int) {
	t.Helper()
	if len(result.Resources) != expectedCount {
		t.Errorf("Expected %d resources, got %d", expectedCount, len(result.Resources))
	}
	for _, resource := range result.Resources {
		if resource.URI == "" || resource.Name == "" || resource.MimeType == "" {
			t.Error("Resource missing required fields")
		}
	}
}

func validateResourceContent(t *testing.T, content models.MCPResourceContent, expectedURI string, expectedTexts []string) {
	t.Helper()
	if content.URI != expectedURI {
		t.Errorf("Expected URI '%s', got '%s'", expectedURI, content.URI)
	}
	if content.MimeType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", content.MimeType)
	}
	for _, text := range expectedTexts {
		if !strings.Contains(content.Text, text) {
			t.Errorf("Content should contain '%s'", text)
		}
	}
}

// Test: Documentation System Integration
func TestDocumentationSystemIntegration(t *testing.T) {
	env := setupTestEnv(t)
	env.writeTestDocs(t, standardTestDocs(env))
	env.initServer(t)

	// Verify initial cache population
	if env.server.cache.Size() != 3 {
		t.Errorf("Expected 3 documents in cache, got %d", env.server.cache.Size())
	}

	// Verify categorization
	categories := map[string]int{
		"guideline": len(env.server.cache.GetByCategory("guideline")),
		"pattern":   len(env.server.cache.GetByCategory("pattern")),
		"adr":       len(env.server.cache.GetByCategory("adr")),
	}

	for cat, count := range categories {
		if count != 1 {
			t.Errorf("Expected 1 %s document, got %d", cat, count)
		}
	}
}

// Test: Resource List Method
func TestResourcesListMethod(t *testing.T) {
	env := setupTestEnv(t)
	env.writeTestDocs(t, standardTestDocs(env))
	env.initServer(t)

	msg := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-list",
		Method:  "resources/list",
	}

	response := env.server.handleResourcesList(msg)
	validateMCPResponse(t, response, false)

	result := response.Result.(models.MCPResourcesListResult)
	validateResourceList(t, result, 3)
}

// Test: Resource Read Method - Table Driven
func TestResourcesReadMethod(t *testing.T) {
	env := setupTestEnv(t)
	env.writeTestDocs(t, standardTestDocs(env))
	env.initServer(t)

	tests := []struct {
		name          string
		uri           string
		expectedTexts []string
	}{
		{
			name:          "Guideline",
			uri:           "architecture://guidelines/api-design",
			expectedTexts: []string{"API Design Guidelines", "REST Principles"},
		},
		{
			name:          "Pattern",
			uri:           "architecture://patterns/repository-pattern",
			expectedTexts: []string{"Repository Pattern", "Implementation"},
		},
		{
			name:          "ADR",
			uri:           "architecture://adr/001",
			expectedTexts: []string{"ADR-001", "Status", "Decision"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-read",
				Method:  "resources/read",
				Params:  models.MCPResourcesReadParams{URI: tt.uri},
			}

			response := env.server.handleResourcesRead(msg)
			validateMCPResponse(t, response, false)

			result := response.Result.(models.MCPResourcesReadResult)
			if len(result.Contents) != 1 {
				t.Fatalf("Expected 1 content item, got %d", len(result.Contents))
			}

			validateResourceContent(t, result.Contents[0], tt.uri, tt.expectedTexts)
		})
	}
}

// Test: Resource Error Scenarios - Table Driven
func TestResourceErrorScenarios(t *testing.T) {
	server := NewMCPServer()

	tests := []struct {
		name         string
		uri          string
		expectedCode int
		errorText    string
	}{
		{
			name:         "InvalidScheme",
			uri:          "invalid://guidelines/test",
			expectedCode: -32602,
			errorText:    "Invalid resource URI",
		},
		{
			name:         "UnsupportedCategory",
			uri:          "architecture://unsupported/test",
			expectedCode: -32602,
			errorText:    "unsupported resource category",
		},
		{
			name:         "MissingURI",
			uri:          "",
			expectedCode: -32602,
			errorText:    "Missing required parameter: uri",
		},
		{
			name:         "ResourceNotFound",
			uri:          "architecture://guidelines/nonexistent",
			expectedCode: -32603,
			errorText:    "Resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-error",
				Method:  "resources/read",
				Params:  models.MCPResourcesReadParams{URI: tt.uri},
			}

			response := server.handleResourcesRead(msg)
			validateMCPResponse(t, response, true)

			if response.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, response.Error.Code)
			}
			if !strings.Contains(response.Error.Message, tt.errorText) {
				t.Errorf("Expected error message containing '%s', got '%s'", tt.errorText, response.Error.Message)
			}
		})
	}
}

// Test: URI Parsing - Table Driven
func TestResourceURIParsing(t *testing.T) {
	server := NewMCPServer()

	tests := []struct {
		name        string
		uri         string
		wantCat     string
		wantPath    string
		expectError bool
	}{
		{"ValidGuideline", "architecture://guidelines/api-design", "guideline", "api-design", false},
		{"ValidPattern", "architecture://patterns/repository", "pattern", "repository", false},
		{"ValidADR", "architecture://adr/001", "adr", "001", false},
		{"InvalidScheme", "invalid://guidelines/test", "", "", true},
		{"InvalidCategory", "architecture://invalid/test", "", "", true},
		{"MissingPath", "architecture://guidelines", "", "", true},
		{"EmptyURI", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, path, err := server.parseResourceURI(tt.uri)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if cat != tt.wantCat {
				t.Errorf("Expected category '%s', got '%s'", tt.wantCat, cat)
			}
			if path != tt.wantPath {
				t.Errorf("Expected path '%s', got '%s'", tt.wantPath, path)
			}
		})
	}
}

// Test: Protocol Compliance
func TestJSONRPCCompliance(t *testing.T) {
	server := NewMCPServer()

	msg := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "compliance-test",
		Method:  "resources/list",
	}

	response := server.handleResourcesList(msg)
	validateMCPResponse(t, response, false)

	if response.ID != "compliance-test" {
		t.Errorf("Expected ID 'compliance-test', got '%v'", response.ID)
	}
	if response.Result == nil {
		t.Error("Response should have result")
	}
}

// Test: Tools System Integration
func TestToolsSystemIntegration(t *testing.T) {
	env := setupTestEnv(t)
	env.writeTestDocs(t, standardTestDocs(env))
	env.initServer(t)

	if err := env.server.initializeToolsSystem(); err != nil {
		t.Fatalf("Failed to initialize tools system: %v", err)
	}

	// Test tools/list
	listMsg := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "test-tools-list",
		Method:  "tools/list",
	}

	response := env.server.handleToolsList(listMsg)
	validateMCPResponse(t, response, false)

	result := response.Result.(models.MCPToolsListResult)
	if len(result.Tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(result.Tools))
	}
}

// Test: Tools Call Method - Table Driven
func TestToolsCallMethod(t *testing.T) {
	env := setupTestEnv(t)
	env.writeTestDocs(t, standardTestDocs(env))
	env.initServer(t)

	if err := env.server.initializeToolsSystem(); err != nil {
		t.Fatalf("Failed to initialize tools system: %v", err)
	}

	tests := []struct {
		name      string
		toolName  string
		args      map[string]interface{}
		expectErr bool
	}{
		{
			name:     "ValidatePattern",
			toolName: "validate-against-pattern",
			args: map[string]interface{}{
				"code":         "type Repository interface {}",
				"pattern_name": "repository-pattern",
				"language":     "go",
			},
			expectErr: false,
		},
		{
			name:     "SearchArchitecture",
			toolName: "search-architecture",
			args: map[string]interface{}{
				"query":         "API",
				"resource_type": "all",
				"max_results":   5,
			},
			expectErr: false,
		},
		{
			name:     "CheckADRAlignment",
			toolName: "check-adr-alignment",
			args: map[string]interface{}{
				"decision_description": "Use microservices",
			},
			expectErr: false,
		},
		{
			name:      "InvalidTool",
			toolName:  "nonexistent-tool",
			args:      map[string]interface{}{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-tool-call",
				Method:  "tools/call",
				Params: models.MCPToolsCallParams{
					Name:      tt.toolName,
					Arguments: tt.args,
				},
			}

			response := env.server.handleToolsCall(msg)
			validateMCPResponse(t, response, tt.expectErr)

			if !tt.expectErr {
				result := response.Result.(models.MCPToolsCallResult)
				if len(result.Content) == 0 {
					t.Error("Expected at least one content item")
				}
			}
		})
	}
}

// Test: Cache Refresh Coordinator
func TestCacheRefreshCoordinator(t *testing.T) {
	server := NewMCPServer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go server.cacheRefreshCoordinator(ctx)

	testEvent := models.FileEvent{Type: "delete", Path: config.PatternsPath + "/test.md"}

	select {
	case server.refreshChan <- testEvent:
		// Event sent successfully
	case <-time.After(1 * time.Second):
		t.Error("Failed to send event to refresh channel")
	}
}

// Test: Category From Path - Table Driven
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
		{strings.ToUpper(config.GuidelinesPath) + "/API.MD", "guideline"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := server.getCategoryFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Test: Concurrent Tool Invocations
func TestConcurrentToolInvocations(t *testing.T) {
	env := setupTestEnv(t)
	env.writeTestDocs(t, standardTestDocs(env))
	env.initServer(t)

	if err := env.server.initializeToolsSystem(); err != nil {
		t.Fatalf("Failed to initialize tools system: %v", err)
	}

	const numConcurrent = 10
	done := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(index int) {
			msg := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      fmt.Sprintf("concurrent-%d", index),
				Method:  "tools/call",
				Params: models.MCPToolsCallParams{
					Name: "search-architecture",
					Arguments: map[string]interface{}{
						"query":         "API",
						"resource_type": "all",
						"max_results":   5,
					},
				},
			}

			response := env.server.handleToolsCall(msg)
			if response.Error != nil {
				done <- fmt.Errorf("concurrent call %d failed: %v", index, response.Error)
			} else {
				done <- nil
			}
		}(i)
	}

	for i := 0; i < numConcurrent; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent tool invocations timed out")
		}
	}
}
