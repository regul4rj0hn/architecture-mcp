package cache

import (
	"fmt"
	"sync"

	"mcp-architecture-service/internal/models"
)

// DocumentCache provides in-memory caching for documentation
type DocumentCache struct {
	documents map[string]*models.Document
	indexes   map[string]*models.DocumentIndex
	mutex     sync.RWMutex
}

// NewDocumentCache creates a new document cache
func NewDocumentCache() *DocumentCache {
	return &DocumentCache{
		documents: make(map[string]*models.Document),
		indexes:   make(map[string]*models.DocumentIndex),
	}
}

// Get retrieves a document from the cache by key (path)
func (dc *DocumentCache) Get(key string) (*models.Document, error) {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	document, exists := dc.documents[key]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", key)
	}

	return document, nil
}

// Set stores a document in the cache
func (dc *DocumentCache) Set(key string, document *models.Document) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.documents[key] = document
}

// Invalidate removes a document from the cache
func (dc *DocumentCache) Invalidate(key string) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	delete(dc.documents, key)
}

// Clear removes all documents from the cache
func (dc *DocumentCache) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.documents = make(map[string]*models.Document)
	dc.indexes = make(map[string]*models.DocumentIndex)
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
