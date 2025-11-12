package cache

import (
	"runtime"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
	"mcp-architecture-service/pkg/logging"
)

// DocumentCache provides in-memory caching for documentation with optimized memory usage
type DocumentCache struct {
	documents      map[string]*models.Document
	indexes        map[string]*models.DocumentIndex
	pathToCategory map[string]string // Maps document paths to their categories for fast category lookup
	mutex          sync.RWMutex
	stats          CacheStats

	// Memory optimization features
	memoryPool     *sync.Pool // Pool for reusing document objects
	maxMemoryUsage int64      // Maximum memory usage before cleanup (in bytes)
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
	logger         *logging.StructuredLogger
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits          int64     `json:"hits"`
	Misses        int64     `json:"misses"`
	Invalidations int64     `json:"invalidations"`
	LastCleanup   time.Time `json:"lastCleanup"`
	MemoryUsage   int64     `json:"memoryUsage"` // Approximate memory usage in bytes
}

// NewDocumentCache creates a new document cache with memory optimizations
func NewDocumentCache() *DocumentCache {
	// Create a default logger for the cache
	loggingManager := logging.NewLoggingManager()
	logger := loggingManager.GetLogger("cache")

	cache := &DocumentCache{
		documents:      make(map[string]*models.Document),
		indexes:        make(map[string]*models.DocumentIndex),
		pathToCategory: make(map[string]string),
		stats:          CacheStats{LastCleanup: time.Now()},
		maxMemoryUsage: 256 * 1024 * 1024, // 256MB default limit
		stopCleanup:    make(chan struct{}),
		logger:         logger,
	}

	// Initialize memory pool for document reuse
	cache.memoryPool = &sync.Pool{
		New: func() interface{} {
			return &models.Document{}
		},
	}

	// Start periodic cleanup goroutine
	cache.cleanupTicker = time.NewTicker(5 * time.Minute)
	go cache.periodicCleanup()

	return cache
}

// periodicCleanup runs periodic memory cleanup operations
func (dc *DocumentCache) periodicCleanup() {
	for {
		select {
		case <-dc.cleanupTicker.C:
			dc.performMemoryCleanup()
		case <-dc.stopCleanup:
			dc.cleanupTicker.Stop()
			return
		}
	}
}

// performMemoryCleanup performs memory optimization when usage is high
func (dc *DocumentCache) performMemoryCleanup() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Check if memory usage exceeds threshold
	if dc.stats.MemoryUsage > dc.maxMemoryUsage {
		// Force garbage collection
		runtime.GC()

		// Update memory usage after GC
		dc.updateMemoryUsage()

		dc.logger.WithContext("memory_usage_bytes", dc.stats.MemoryUsage).
			Debug("Cache cleanup performed")
		dc.stats.LastCleanup = time.Now()
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

// Set stores a document in the cache with memory optimization
func (dc *DocumentCache) Set(key string, document *models.Document) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Check if we need to perform cleanup before adding new document
	if dc.stats.MemoryUsage > dc.maxMemoryUsage*80/100 { // 80% threshold
		dc.performLRUCleanup()
	}

	dc.documents[key] = document
	dc.pathToCategory[key] = document.Metadata.Category
	dc.updateMemoryUsage()
}

// performLRUCleanup removes least recently used documents to free memory
func (dc *DocumentCache) performLRUCleanup() {
	// Simple cleanup strategy: remove 10% of documents
	// In a production system, you might implement proper LRU tracking
	targetSize := len(dc.documents) * 90 / 100

	if targetSize >= len(dc.documents) {
		return
	}

	// Remove documents until we reach target size
	count := 0
	for key := range dc.documents {
		if count >= len(dc.documents)-targetSize {
			break
		}
		delete(dc.documents, key)
		delete(dc.pathToCategory, key)
		count++
	}

	dc.stats.Invalidations += int64(count)
	dc.logger.WithContext("documents_removed", count).
		Debug("LRU cleanup removed documents")
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

// Clear removes all documents from the cache and stops cleanup goroutine
func (dc *DocumentCache) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.documents = make(map[string]*models.Document)
	dc.indexes = make(map[string]*models.DocumentIndex)
	dc.pathToCategory = make(map[string]string)
	dc.stats.LastCleanup = time.Now()
	dc.updateMemoryUsage()
}

// Close stops the cache cleanup goroutine and releases resources
func (dc *DocumentCache) Close() {
	close(dc.stopCleanup)
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

// GetPerformanceMetrics returns detailed performance metrics for monitoring
func (dc *DocumentCache) GetPerformanceMetrics() map[string]interface{} {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return map[string]interface{}{
		"total_documents":    len(dc.documents),
		"total_categories":   len(dc.indexes),
		"memory_usage_bytes": dc.stats.MemoryUsage,
		"memory_limit_bytes": dc.maxMemoryUsage,
		"memory_usage_pct":   float64(dc.stats.MemoryUsage) / float64(dc.maxMemoryUsage) * 100.0,
		"cache_hits":         dc.stats.Hits,
		"cache_misses":       dc.stats.Misses,
		"cache_hit_ratio":    dc.GetCacheHitRatio(),
		"invalidations":      dc.stats.Invalidations,
		"last_cleanup":       dc.stats.LastCleanup,
	}
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
