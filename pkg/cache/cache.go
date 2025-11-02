package cache

import (
	"runtime"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
)

// DocumentCache provides in-memory caching for documentation
type DocumentCache struct {
	documents      map[string]*models.Document
	indexes        map[string]*models.DocumentIndex
	pathToCategory map[string]string // Maps document paths to their categories for fast category lookup
	mutex          sync.RWMutex
	stats          CacheStats
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits          int64     `json:"hits"`
	Misses        int64     `json:"misses"`
	Invalidations int64     `json:"invalidations"`
	LastCleanup   time.Time `json:"lastCleanup"`
	MemoryUsage   int64     `json:"memoryUsage"` // Approximate memory usage in bytes
}

// NewDocumentCache creates a new document cache
func NewDocumentCache() *DocumentCache {
	return &DocumentCache{
		documents:      make(map[string]*models.Document),
		indexes:        make(map[string]*models.DocumentIndex),
		pathToCategory: make(map[string]string),
		stats:          CacheStats{LastCleanup: time.Now()},
	}
}

// Get retrieves a document from the cache by key (path)
func (dc *DocumentCache) Get(key string) (*models.Document, error) {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	document, exists := dc.documents[key]
	if !exists {
		dc.stats.Misses++
		return nil, errors.NewCacheError(errors.ErrCodeCacheMiss,
			"Document not found in cache", nil).
			WithContext("key", key)
	}

	dc.stats.Hits++
	return document, nil
}

// Set stores a document in the cache
func (dc *DocumentCache) Set(key string, document *models.Document) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.documents[key] = document
	dc.pathToCategory[key] = document.Metadata.Category
	dc.updateMemoryUsage()
}

// Invalidate removes a document from the cache
func (dc *DocumentCache) Invalidate(key string) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	delete(dc.documents, key)
	delete(dc.pathToCategory, key)
	dc.stats.Invalidations++
	dc.updateMemoryUsage()
}

// Clear removes all documents from the cache
func (dc *DocumentCache) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.documents = make(map[string]*models.Document)
	dc.indexes = make(map[string]*models.DocumentIndex)
	dc.pathToCategory = make(map[string]string)
	dc.stats.LastCleanup = time.Now()
	dc.updateMemoryUsage()
}

// GetIndex retrieves a document index by category
func (dc *DocumentCache) GetIndex(category string) *models.DocumentIndex {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return dc.indexes[category]
}

// SetIndex stores a document index for a category
func (dc *DocumentCache) SetIndex(category string, index *models.DocumentIndex) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.indexes[category] = index
}

// GetAllDocuments returns all cached documents
func (dc *DocumentCache) GetAllDocuments() map[string]*models.Document {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	// Create a copy to avoid concurrent access issues
	result := make(map[string]*models.Document)
	for key, doc := range dc.documents {
		result[key] = doc
	}

	return result
}

// GetAllIndexes returns all cached indexes
func (dc *DocumentCache) GetAllIndexes() map[string]*models.DocumentIndex {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	// Create a copy to avoid concurrent access issues
	result := make(map[string]*models.DocumentIndex)
	for key, index := range dc.indexes {
		result[key] = index
	}

	return result
}

// Size returns the number of cached documents
func (dc *DocumentCache) Size() int {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return len(dc.documents)
}

// GetByCategory retrieves all documents for a specific category
func (dc *DocumentCache) GetByCategory(category string) []*models.Document {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	var documents []*models.Document
	for path, docCategory := range dc.pathToCategory {
		if docCategory == category {
			if doc, exists := dc.documents[path]; exists {
				documents = append(documents, doc)
			}
		}
	}

	return documents
}

// InvalidateByCategory removes all documents of a specific category from the cache
func (dc *DocumentCache) InvalidateByCategory(category string) int {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	var invalidatedCount int
	var pathsToDelete []string

	// Collect paths to delete to avoid modifying map while iterating
	for path, docCategory := range dc.pathToCategory {
		if docCategory == category {
			pathsToDelete = append(pathsToDelete, path)
		}
	}

	// Delete collected paths
	for _, path := range pathsToDelete {
		delete(dc.documents, path)
		delete(dc.pathToCategory, path)
		invalidatedCount++
	}

	dc.stats.Invalidations += int64(invalidatedCount)
	dc.updateMemoryUsage()

	return invalidatedCount
}

// InvalidateByPaths removes multiple documents from the cache by their paths
func (dc *DocumentCache) InvalidateByPaths(paths []string) int {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	var invalidatedCount int
	for _, path := range paths {
		if _, exists := dc.documents[path]; exists {
			delete(dc.documents, path)
			delete(dc.pathToCategory, path)
			invalidatedCount++
		}
	}

	dc.stats.Invalidations += int64(invalidatedCount)
	dc.updateMemoryUsage()

	return invalidatedCount
}

// GetStats returns cache performance statistics
func (dc *DocumentCache) GetStats() CacheStats {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return dc.stats
}

// Cleanup performs memory cleanup and garbage collection
func (dc *DocumentCache) Cleanup() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Force garbage collection to free up memory
	runtime.GC()

	dc.stats.LastCleanup = time.Now()
	dc.updateMemoryUsage()
}

// updateMemoryUsage estimates the memory usage of the cache (must be called with lock held)
func (dc *DocumentCache) updateMemoryUsage() {
	var memUsage int64

	// Estimate memory usage based on document count and average size
	// This is an approximation since exact memory measurement is complex in Go
	for _, doc := range dc.documents {
		// Estimate: metadata (~200 bytes) + content size + overhead
		memUsage += 200 + int64(len(doc.Content.RawContent)) + 100
	}

	// Add overhead for maps and indexes
	memUsage += int64(len(dc.documents) * 50)       // Map overhead
	memUsage += int64(len(dc.pathToCategory) * 100) // Path mapping overhead

	dc.stats.MemoryUsage = memUsage
}

// GetCacheHitRatio returns the cache hit ratio as a percentage
func (dc *DocumentCache) GetCacheHitRatio() float64 {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	total := dc.stats.Hits + dc.stats.Misses
	if total == 0 {
		return 0.0
	}

	return float64(dc.stats.Hits) / float64(total) * 100.0
}

// IsEmpty returns true if the cache contains no documents
func (dc *DocumentCache) IsEmpty() bool {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return len(dc.documents) == 0
}

// GetCategories returns all unique categories in the cache
func (dc *DocumentCache) GetCategories() []string {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	categorySet := make(map[string]bool)
	for _, category := range dc.pathToCategory {
		categorySet[category] = true
	}

	var categories []string
	for category := range categorySet {
		categories = append(categories, category)
	}

	return categories
}
