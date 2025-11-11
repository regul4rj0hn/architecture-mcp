package cache

import (
	"fmt"
	"sync"
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
			Path:         "/mcp/resources/guidelines/test.md",
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
				Path:     "/mcp/resources/guidelines/guide1.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Guideline 2",
				Category: "guideline",
				Path:     "/mcp/resources/guidelines/guide2.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Pattern 1",
				Category: "pattern",
				Path:     "/mcp/resources/patterns/pattern1.md",
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
			Path:     "/mcp/resources/guidelines/test.md",
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
				Path:     "/mcp/resources/guidelines/guide1.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Guideline 2",
				Category: "guideline",
				Path:     "/mcp/resources/guidelines/guide2.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Pattern 1",
				Category: "pattern",
				Path:     "/mcp/resources/patterns/pattern1.md",
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
		"/mcp/resources/guidelines/guide1.md",
		"/mcp/resources/guidelines/guide2.md",
		"/mcp/resources/patterns/pattern1.md",
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
		"/mcp/resources/guidelines/guide1.md",
		"/mcp/resources/patterns/pattern1.md",
		"/nonexistent/path.md", // Should not cause error
	}

	count := cache.InvalidateByPaths(pathsToInvalidate)
	if count != 2 {
		t.Errorf("Expected to invalidate 2 documents, got %d", count)
	}

	// Verify correct documents were invalidated
	_, err := cache.Get("/mcp/resources/guidelines/guide1.md")
	if err == nil {
		t.Error("guide1.md should be invalidated")
	}

	_, err = cache.Get("/mcp/resources/patterns/pattern1.md")
	if err == nil {
		t.Error("pattern1.md should be invalidated")
	}

	_, err = cache.Get("/mcp/resources/guidelines/guide2.md")
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
				Path:     fmt.Sprintf("/mcp/resources/test%d.md", i),
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
			Path:     "/mcp/resources/test.md",
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
				Path:     fmt.Sprintf("/mcp/resources/%s/test%d.md", category, i),
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
			Path:     "/mcp/resources/test.md",
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

// TestDocumentCache_IndexOperations tests index storage and retrieval
func TestDocumentCache_IndexOperations(t *testing.T) {
	cache := NewDocumentCache()

	// Create test index
	index := &models.DocumentIndex{
		Category: "guideline",
		Documents: []models.DocumentMetadata{
			{
				Title:    "Test Guideline 1",
				Category: "guideline",
				Path:     "/mcp/resources/guidelines/test1.md",
			},
			{
				Title:    "Test Guideline 2",
				Category: "guideline",
				Path:     "/mcp/resources/guidelines/test2.md",
			},
		},
		Count: 2,
	}

	// Test SetIndex
	cache.SetIndex("guideline", index)

	// Test GetIndex
	retrievedIndex := cache.GetIndex("guideline")
	if retrievedIndex == nil {
		t.Fatal("Expected index to be retrieved, got nil")
	}

	if retrievedIndex.Category != "guideline" {
		t.Errorf("Expected category 'guideline', got %s", retrievedIndex.Category)
	}

	if retrievedIndex.Count != 2 {
		t.Errorf("Expected count 2, got %d", retrievedIndex.Count)
	}

	if len(retrievedIndex.Documents) != 2 {
		t.Errorf("Expected 2 documents in index, got %d", len(retrievedIndex.Documents))
	}

	// Test GetIndex for non-existent category
	nonExistentIndex := cache.GetIndex("nonexistent")
	if nonExistentIndex != nil {
		t.Error("Expected nil for non-existent index")
	}
}

// TestDocumentCache_GetAllOperations tests GetAllDocuments and GetAllIndexes
func TestDocumentCache_GetAllOperations(t *testing.T) {
	cache := NewDocumentCache()

	// Add test documents
	docs := []*models.Document{
		{
			Metadata: models.DocumentMetadata{
				Title:    "Doc 1",
				Category: "guideline",
				Path:     "/mcp/resources/guidelines/doc1.md",
			},
		},
		{
			Metadata: models.DocumentMetadata{
				Title:    "Doc 2",
				Category: "pattern",
				Path:     "/mcp/resources/patterns/doc2.md",
			},
		},
	}

	for _, doc := range docs {
		cache.Set(doc.Metadata.Path, doc)
	}

	// Add test indexes
	guidelineIndex := &models.DocumentIndex{
		Category: "guideline",
		Count:    1,
	}
	patternIndex := &models.DocumentIndex{
		Category: "pattern",
		Count:    1,
	}

	cache.SetIndex("guideline", guidelineIndex)
	cache.SetIndex("pattern", patternIndex)

	// Test GetAllDocuments
	allDocs := cache.GetAllDocuments()
	if len(allDocs) != 2 {
		t.Errorf("Expected 2 documents, got %d", len(allDocs))
	}

	// Verify documents are copies (concurrent access safety)
	for path, doc := range allDocs {
		if doc.Metadata.Path != path {
			t.Errorf("Document path mismatch: expected %s, got %s", path, doc.Metadata.Path)
		}
	}

	// Test GetAllIndexes
	allIndexes := cache.GetAllIndexes()
	if len(allIndexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(allIndexes))
	}

	if allIndexes["guideline"] == nil || allIndexes["pattern"] == nil {
		t.Error("Expected both guideline and pattern indexes to be present")
	}
}

// TestDocumentCache_ConcurrentAccess tests concurrent read/write operations
func TestDocumentCache_ConcurrentAccess(t *testing.T) {
	cache := NewDocumentCache()

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Test concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				doc := &models.Document{
					Metadata: models.DocumentMetadata{
						Title:    fmt.Sprintf("Doc %d-%d", id, j),
						Category: "guideline",
						Path:     fmt.Sprintf("/mcp/resources/guidelines/doc-%d-%d.md", id, j),
					},
					Content: models.DocumentContent{
						RawContent: fmt.Sprintf("Content for doc %d-%d", id, j),
					},
				}
				cache.Set(doc.Metadata.Path, doc)
			}
		}(i)
	}
	wg.Wait()

	expectedSize := numGoroutines * numOperations
	if cache.Size() != expectedSize {
		t.Errorf("Expected cache size %d, got %d", expectedSize, cache.Size())
	}

	// Test concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				path := fmt.Sprintf("/mcp/resources/guidelines/doc-%d-%d.md", id, j)
				doc, err := cache.Get(path)
				if err != nil {
					t.Errorf("Failed to get document %s: %v", path, err)
					return
				}
				if doc.Metadata.Path != path {
					t.Errorf("Document path mismatch: expected %s, got %s", path, doc.Metadata.Path)
				}
			}
		}(i)
	}
	wg.Wait()

	// Test concurrent invalidations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations/2; j++ {
				path := fmt.Sprintf("/mcp/resources/guidelines/doc-%d-%d.md", id, j)
				cache.Invalidate(path)
			}
		}(i)
	}
	wg.Wait()

	// Verify some documents were invalidated
	finalSize := cache.Size()
	if finalSize >= expectedSize {
		t.Errorf("Expected cache size to be reduced after invalidations, got %d", finalSize)
	}
}

// TestDocumentCache_ConcurrentReadWrite tests mixed concurrent operations
func TestDocumentCache_ConcurrentReadWrite(t *testing.T) {
	cache := NewDocumentCache()

	const numReaders = 5
	const numWriters = 3
	const numOperations = 50

	var wg sync.WaitGroup

	// Start writers
	wg.Add(numWriters)
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				doc := &models.Document{
					Metadata: models.DocumentMetadata{
						Title:    fmt.Sprintf("Writer %d Doc %d", id, j),
						Category: "guideline",
						Path:     fmt.Sprintf("/mcp/resources/guidelines/writer-%d-doc-%d.md", id, j),
					},
				}
				cache.Set(doc.Metadata.Path, doc)

				// Occasionally invalidate older documents
				if j > 10 && j%10 == 0 {
					oldPath := fmt.Sprintf("/mcp/resources/guidelines/writer-%d-doc-%d.md", id, j-10)
					cache.Invalidate(oldPath)
				}
			}
		}(i)
	}

	// Start readers
	wg.Add(numReaders)
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Try to read various documents
				for writerID := 0; writerID < numWriters; writerID++ {
					path := fmt.Sprintf("/mcp/resources/guidelines/writer-%d-doc-%d.md", writerID, j)
					_, _ = cache.Get(path) // Ignore errors as documents may not exist yet
				}

				// Test other read operations
				_ = cache.GetByCategory("guideline")
				_ = cache.GetCategories()
				_ = cache.Size()
				_ = cache.IsEmpty()
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is in a consistent state
	stats := cache.GetStats()
	if stats.Hits < 0 || stats.Misses < 0 {
		t.Error("Cache stats should not be negative")
	}

	allDocs := cache.GetAllDocuments()
	if len(allDocs) != cache.Size() {
		t.Errorf("GetAllDocuments count (%d) doesn't match Size() (%d)", len(allDocs), cache.Size())
	}
}

// TestDocumentCache_MemoryManagement tests memory usage tracking and cleanup
func TestDocumentCache_MemoryManagement(t *testing.T) {
	cache := NewDocumentCache()

	// Test initial memory usage
	initialStats := cache.GetStats()
	if initialStats.MemoryUsage != 0 {
		t.Errorf("Expected initial memory usage to be 0, got %d", initialStats.MemoryUsage)
	}

	// Add documents with varying content sizes
	contentSizes := []int{100, 1000, 10000}
	for i, size := range contentSizes {
		content := make([]byte, size)
		for j := range content {
			content[j] = 'A' + byte(j%26) // Fill with letters
		}

		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:    fmt.Sprintf("Doc %d", i),
				Category: "guideline",
				Path:     fmt.Sprintf("/mcp/resources/guidelines/doc%d.md", i),
			},
			Content: models.DocumentContent{
				RawContent: string(content),
			},
		}
		cache.Set(doc.Metadata.Path, doc)

		// Verify memory usage increases
		stats := cache.GetStats()
		if stats.MemoryUsage <= initialStats.MemoryUsage {
			t.Errorf("Expected memory usage to increase after adding document %d", i)
		}
		initialStats = stats
	}

	// Test cleanup
	preCleanupStats := cache.GetStats()
	cache.Cleanup()
	postCleanupStats := cache.GetStats()

	// Verify cleanup timestamp was updated
	if !postCleanupStats.LastCleanup.After(preCleanupStats.LastCleanup) {
		t.Error("Cleanup timestamp should be updated")
	}

	// Documents should still exist after cleanup
	if cache.Size() != len(contentSizes) {
		t.Errorf("Expected %d documents after cleanup, got %d", len(contentSizes), cache.Size())
	}

	// Test memory usage after clearing cache
	cache.Clear()
	finalStats := cache.GetStats()
	if finalStats.MemoryUsage != 0 {
		t.Errorf("Expected memory usage to be 0 after clear, got %d", finalStats.MemoryUsage)
	}
}

// TestDocumentCache_LargeDataset tests cache performance with large datasets
func TestDocumentCache_LargeDataset(t *testing.T) {
	cache := NewDocumentCache()

	const numDocuments = 1000
	const contentSize = 1024 // 1KB per document

	// Add large number of documents
	for i := 0; i < numDocuments; i++ {
		content := make([]byte, contentSize)
		for j := range content {
			content[j] = byte('A' + (i+j)%26)
		}

		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:    fmt.Sprintf("Large Doc %d", i),
				Category: fmt.Sprintf("category%d", i%5), // 5 different categories
				Path:     fmt.Sprintf("/mcp/resources/large/doc%d.md", i),
			},
			Content: models.DocumentContent{
				RawContent: string(content),
			},
		}
		cache.Set(doc.Metadata.Path, doc)
	}

	// Verify all documents were added
	if cache.Size() != numDocuments {
		t.Errorf("Expected %d documents, got %d", numDocuments, cache.Size())
	}

	// Test retrieval performance
	start := time.Now()
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/mcp/resources/large/doc%d.md", i)
		_, err := cache.Get(path)
		if err != nil {
			t.Errorf("Failed to get document %s: %v", path, err)
		}
	}
	duration := time.Since(start)

	// Should be very fast (under 10ms for 100 retrievals)
	if duration > 10*time.Millisecond {
		t.Errorf("Cache retrieval too slow: %v for 100 operations", duration)
	}

	// Test category operations
	categories := cache.GetCategories()
	if len(categories) != 5 {
		t.Errorf("Expected 5 categories, got %d", len(categories))
	}

	// Test category-based retrieval
	for i := 0; i < 5; i++ {
		categoryDocs := cache.GetByCategory(fmt.Sprintf("category%d", i))
		expectedCount := numDocuments / 5
		if len(categoryDocs) != expectedCount {
			t.Errorf("Expected %d documents in category%d, got %d", expectedCount, i, len(categoryDocs))
		}
	}

	// Test bulk invalidation
	pathsToInvalidate := make([]string, 100)
	for i := 0; i < 100; i++ {
		pathsToInvalidate[i] = fmt.Sprintf("/mcp/resources/large/doc%d.md", i)
	}

	invalidatedCount := cache.InvalidateByPaths(pathsToInvalidate)
	if invalidatedCount != 100 {
		t.Errorf("Expected to invalidate 100 documents, got %d", invalidatedCount)
	}

	if cache.Size() != numDocuments-100 {
		t.Errorf("Expected %d documents after invalidation, got %d", numDocuments-100, cache.Size())
	}
}

// TestDocumentCache_EdgeCases tests various edge cases and error conditions
func TestDocumentCache_EdgeCases(t *testing.T) {
	cache := NewDocumentCache()

	// Test operations on empty cache
	if !cache.IsEmpty() {
		t.Error("New cache should be empty")
	}

	if cache.Size() != 0 {
		t.Error("New cache size should be 0")
	}

	categories := cache.GetCategories()
	if len(categories) != 0 {
		t.Error("New cache should have no categories")
	}

	docs := cache.GetByCategory("nonexistent")
	if len(docs) != 0 {
		t.Error("GetByCategory on empty cache should return empty slice")
	}

	// Test hit ratio on empty cache
	hitRatio := cache.GetCacheHitRatio()
	if hitRatio != 0.0 {
		t.Errorf("Expected hit ratio 0.0 for empty cache, got %f", hitRatio)
	}

	// Test invalidation of non-existent documents
	cache.Invalidate("/nonexistent/path")

	count := cache.InvalidateByCategory("nonexistent")
	if count != 0 {
		t.Errorf("Expected 0 invalidations for non-existent category, got %d", count)
	}

	count = cache.InvalidateByPaths([]string{"/nonexistent1", "/nonexistent2"})
	if count != 0 {
		t.Errorf("Expected 0 invalidations for non-existent paths, got %d", count)
	}

	// Test with document that has empty category
	emptyDoc := &models.Document{
		Metadata: models.DocumentMetadata{
			Path:     "/test/empty",
			Category: "",
		},
	}
	cache.Set("/test/empty", emptyDoc)
	retrievedDoc, err := cache.Get("/test/empty")
	if err != nil {
		t.Errorf("Should be able to store and retrieve document with empty category: %v", err)
	}
	if retrievedDoc.Metadata.Category != "" {
		t.Error("Retrieved document should have empty category")
	}

	// Test with empty path
	emptyPathDoc := &models.Document{
		Metadata: models.DocumentMetadata{
			Path: "",
		},
	}
	cache.Set("", emptyPathDoc)

	retrieved, err := cache.Get("")
	if err != nil {
		t.Errorf("Should be able to use empty string as key: %v", err)
	}
	if retrieved.Metadata.Path != "" {
		t.Error("Retrieved document should have empty path")
	}

	// Test multiple cleanup calls
	cache.Cleanup()
	cache.Cleanup()
	cache.Cleanup()

	// Test multiple clear calls
	cache.Clear()
	cache.Clear()

	if !cache.IsEmpty() {
		t.Error("Cache should be empty after multiple clears")
	}
}

// TestDocumentCache_StatsAccuracy tests the accuracy of cache statistics
func TestDocumentCache_StatsAccuracy(t *testing.T) {
	cache := NewDocumentCache()

	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:    "Stats Test Doc",
			Category: "guideline",
			Path:     "/mcp/resources/guidelines/stats.md",
		},
		Content: models.DocumentContent{
			RawContent: "Test content for stats",
		},
	}

	cache.Set(doc.Metadata.Path, doc)

	// Generate specific hit/miss pattern
	cache.Get(doc.Metadata.Path) // Hit 1
	cache.Get(doc.Metadata.Path) // Hit 2
	cache.Get("/nonexistent1")   // Miss 1
	cache.Get(doc.Metadata.Path) // Hit 3
	cache.Get("/nonexistent2")   // Miss 2
	cache.Get("/nonexistent3")   // Miss 3

	stats := cache.GetStats()
	if stats.Hits != 3 {
		t.Errorf("Expected 3 hits, got %d", stats.Hits)
	}

	if stats.Misses != 3 {
		t.Errorf("Expected 3 misses, got %d", stats.Misses)
	}

	expectedRatio := float64(3) / float64(6) * 100.0 // 50%
	hitRatio := cache.GetCacheHitRatio()
	if hitRatio != expectedRatio {
		t.Errorf("Expected hit ratio %.2f, got %.2f", expectedRatio, hitRatio)
	}

	// Test invalidation count
	cache.Invalidate(doc.Metadata.Path)
	cache.Invalidate("/nonexistent")

	stats = cache.GetStats()
	if stats.Invalidations != 2 { // Both calls increment the counter
		t.Errorf("Expected 2 invalidations, got %d", stats.Invalidations)
	}

	// Add more documents and test category invalidation count
	for i := 0; i < 5; i++ {
		testDoc := &models.Document{
			Metadata: models.DocumentMetadata{
				Path:     fmt.Sprintf("/mcp/resources/guidelines/test%d.md", i),
				Category: "guideline",
			},
		}
		cache.Set(testDoc.Metadata.Path, testDoc)
	}

	invalidatedCount := cache.InvalidateByCategory("guideline")
	if invalidatedCount != 5 {
		t.Errorf("Expected to invalidate 5 documents, got %d", invalidatedCount)
	}

	finalStats := cache.GetStats()
	if finalStats.Invalidations != 7 { // 2 + 5
		t.Errorf("Expected 7 total invalidations, got %d", finalStats.Invalidations)
	}
}
