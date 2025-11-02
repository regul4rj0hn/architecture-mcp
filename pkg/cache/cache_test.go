package cache

import (
	"fmt"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
)

func TestNewDocumentCache(t *testing.T) {
	cache := NewDocumentCache()

	if cache == nil {
		t.Fatal("NewDocumentCache returned nil")
	}

	if cache.Size() != 0 {
		t.Errorf("Expected empty cache, got size %d", cache.Size())
	}

	if !cache.IsEmpty() {
		t.Error("Expected cache to be empty")
	}
}

func TestDocumentCache_SetAndGet(t *testing.T) {
	cache := NewDocumentCache()

	// Create test document
	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Test Document",
			Category:     "guideline",
			Path:         "/docs/guidelines/test.md",
			LastModified: time.Now(),
			Size:         100,
			Checksum:     "abc123",
		},
		Content: models.DocumentContent{
			RawContent: "# Test Document\nThis is a test.",
		},
	}

	// Test Set
	cache.Set(doc.Metadata.Path, doc)

	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}

	// Test Get
	retrieved, err := cache.Get(doc.Metadata.Path)
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}

	if retrieved.Metadata.Title != doc.Metadata.Title {
		t.Errorf("Expected title %s, got %s", doc.Metadata.Title, retrieved.Metadata.Title)
	}

	if retrieved.Metadata.Category != doc.Metadata.Category {
		t.Errorf("Expected category %s, got %s", doc.Metadata.Category, retrieved.Metadata.Category)
	}
}

func TestDocumentCache_GetNonExistent(t *testing.T) {
	cache := NewDocumentCache()

	_, err := cache.Get("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent document")
	}

	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestDocumentCache_GetByCategory(t *testing.T) {
	cache := NewDocumentCache()

	// Create test documents with different categories
	docs := []*models.Document{
		{
			Metadata: models.DocumentMetadata{
				Title:    "Guideline 1",
				Category: "guideline",
				Path:     "/docs/guidelines/guide1.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Guideline 2",
				Category: "guideline",
				Path:     "/docs/guidelines/guide2.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Pattern 1",
				Category: "pattern",
				Path:     "/docs/patterns/pattern1.md",
			},
		},
	}

	// Add documents to cache
	for _, doc := range docs {
		cache.Set(doc.Metadata.Path, doc)
	}

	// Test GetByCategory
	guidelines := cache.GetByCategory("guideline")
	if len(guidelines) != 2 {
		t.Errorf("Expected 2 guidelines, got %d", len(guidelines))
	}

	patterns := cache.GetByCategory("pattern")
	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}

	nonexistent := cache.GetByCategory("nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("Expected 0 documents for nonexistent category, got %d", len(nonexistent))
	}
}

func TestDocumentCache_Invalidate(t *testing.T) {
	cache := NewDocumentCache()

	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:    "Test Document",
			Category: "guideline",
			Path:     "/docs/guidelines/test.md",
		},
	}

	cache.Set(doc.Metadata.Path, doc)

	// Verify document exists
	_, err := cache.Get(doc.Metadata.Path)
	if err != nil {
		t.Fatalf("Document should exist: %v", err)
	}

	// Invalidate document
	cache.Invalidate(doc.Metadata.Path)

	// Verify document is gone
	_, err = cache.Get(doc.Metadata.Path)
	if err == nil {
		t.Error("Document should be invalidated")
	}

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after invalidation, got %d", cache.Size())
	}
}

func TestDocumentCache_InvalidateByCategory(t *testing.T) {
	cache := NewDocumentCache()

	// Add documents with different categories
	docs := []*models.Document{
		{
			Metadata: models.DocumentMetadata{
				Title:    "Guideline 1",
				Category: "guideline",
				Path:     "/docs/guidelines/guide1.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Guideline 2",
				Category: "guideline",
				Path:     "/docs/guidelines/guide2.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Pattern 1",
				Category: "pattern",
				Path:     "/docs/patterns/pattern1.md",
			},
		},
	}

	for _, doc := range docs {
		cache.Set(doc.Metadata.Path, doc)
	}

	// Invalidate all guidelines
	count := cache.InvalidateByCategory("guideline")
	if count != 2 {
		t.Errorf("Expected to invalidate 2 documents, got %d", count)
	}

	// Verify guidelines are gone but pattern remains
	guidelines := cache.GetByCategory("guideline")
	if len(guidelines) != 0 {
		t.Errorf("Expected 0 guidelines after invalidation, got %d", len(guidelines))
	}

	patterns := cache.GetByCategory("pattern")
	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern to remain, got %d", len(patterns))
	}
}

func TestDocumentCache_InvalidateByPaths(t *testing.T) {
	cache := NewDocumentCache()

	// Add test documents
	paths := []string{
		"/docs/guidelines/guide1.md",
		"/docs/guidelines/guide2.md",
		"/docs/patterns/pattern1.md",
	}

	for _, path := range paths {
		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Path:     path,
				Category: "guideline",
			},
		}
		cache.Set(path, doc)
	}

	// Invalidate specific paths
	pathsToInvalidate := []string{
		"/docs/guidelines/guide1.md",
		"/docs/patterns/pattern1.md",
		"/nonexistent/path.md", // Should not cause error
	}

	count := cache.InvalidateByPaths(pathsToInvalidate)
	if count != 2 {
		t.Errorf("Expected to invalidate 2 documents, got %d", count)
	}

	// Verify correct documents were invalidated
	_, err := cache.Get("/docs/guidelines/guide1.md")
	if err == nil {
		t.Error("guide1.md should be invalidated")
	}

	_, err = cache.Get("/docs/patterns/pattern1.md")
	if err == nil {
		t.Error("pattern1.md should be invalidated")
	}

	_, err = cache.Get("/docs/guidelines/guide2.md")
	if err != nil {
		t.Error("guide2.md should still exist")
	}
}

func TestDocumentCache_Clear(t *testing.T) {
	cache := NewDocumentCache()

	// Add test documents
	for i := 0; i < 3; i++ {
		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Path:     fmt.Sprintf("/docs/test%d.md", i),
				Category: "guideline",
			},
		}
		cache.Set(doc.Metadata.Path, doc)
	}

	if cache.Size() != 3 {
		t.Errorf("Expected cache size 3, got %d", cache.Size())
	}

	// Clear cache
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}

	if !cache.IsEmpty() {
		t.Error("Cache should be empty after clear")
	}
}

func TestDocumentCache_Stats(t *testing.T) {
	cache := NewDocumentCache()

	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Path:     "/docs/test.md",
			Category: "guideline",
		},
		Content: models.DocumentContent{
			RawContent: "Test content",
		},
	}

	cache.Set(doc.Metadata.Path, doc)

	// Generate some hits and misses
	cache.Get(doc.Metadata.Path) // Hit
	cache.Get(doc.Metadata.Path) // Hit
	cache.Get("/nonexistent")    // Miss

	stats := cache.GetStats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}

	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	hitRatio := cache.GetCacheHitRatio()
	expectedRatio := float64(2) / float64(3) * 100.0
	if hitRatio != expectedRatio {
		t.Errorf("Expected hit ratio %.2f, got %.2f", expectedRatio, hitRatio)
	}
}

func TestDocumentCache_GetCategories(t *testing.T) {
	cache := NewDocumentCache()

	// Add documents with different categories
	categories := []string{"guideline", "pattern", "adr"}
	for i, category := range categories {
		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Path:     fmt.Sprintf("/docs/%s/test%d.md", category, i),
				Category: category,
			},
		}
		cache.Set(doc.Metadata.Path, doc)
	}

	retrievedCategories := cache.GetCategories()
	if len(retrievedCategories) != 3 {
		t.Errorf("Expected 3 categories, got %d", len(retrievedCategories))
	}

	// Convert to map for easier checking
	categoryMap := make(map[string]bool)
	for _, cat := range retrievedCategories {
		categoryMap[cat] = true
	}

	for _, expectedCat := range categories {
		if !categoryMap[expectedCat] {
			t.Errorf("Expected category %s not found", expectedCat)
		}
	}
}

func TestDocumentCache_Cleanup(t *testing.T) {
	cache := NewDocumentCache()

	// Add a document
	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Path:     "/docs/test.md",
			Category: "guideline",
		},
	}
	cache.Set(doc.Metadata.Path, doc)

	initialStats := cache.GetStats()

	// Perform cleanup
	cache.Cleanup()

	newStats := cache.GetStats()

	// Verify cleanup timestamp was updated
	if !newStats.LastCleanup.After(initialStats.LastCleanup) {
		t.Error("Cleanup timestamp should be updated")
	}

	// Document should still exist after cleanup
	_, err := cache.Get(doc.Metadata.Path)
	if err != nil {
		t.Error("Document should still exist after cleanup")
	}
}
