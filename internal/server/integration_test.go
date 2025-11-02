package server

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
)

func TestDocumentationSystemIntegration(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := ioutil.TempDir("", "mcp_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create docs subdirectories
	guidelinesDir := filepath.Join(tempDir, "docs", "guidelines")
	patternsDir := filepath.Join(tempDir, "docs", "patterns")
	adrDir := filepath.Join(tempDir, "docs", "adr")

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
		if err := ioutil.WriteFile(path, []byte(content), 0644); err != nil {
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

		err = ioutil.WriteFile(apiDesignPath, []byte(newContent), 0644)
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
	testEvent := models.FileEvent{Type: "delete", Path: "docs/patterns/test2.md"}

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
		{"docs/guidelines/api.md", "guideline"},
		{"docs/patterns/repository.md", "pattern"},
		{"docs/adr/adr-001.md", "adr"},
		{"some/other/path.md", "unknown"},
		{"DOCS/GUIDELINES/API.MD", "guideline"}, // Test case insensitive
	}

	for _, test := range tests {
		result := server.getCategoryFromPath(test.path)
		if result != test.expected {
			t.Errorf("getCategoryFromPath(%s) = %s, expected %s", test.path, result, test.expected)
		}
	}
}
