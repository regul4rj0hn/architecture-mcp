package server

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

// handleFileEvent processes file system events and queues them for cache refresh
func (s *MCPServer) handleFileEvent(event models.FileEvent) {
	if !strings.HasSuffix(strings.ToLower(event.Path), config.MarkdownExtension) {
		return
	}

	select {
	case s.refreshChan <- event:
	default:
		// Non-blocking send prevents file monitor from blocking on full channel
		s.logger.WithContext("event_path", event.Path).
			WithContext("event_type", event.Type).
			Warn("Refresh channel full, dropping file event")
	}
}

// cacheRefreshCoordinator coordinates cache updates from file system events
// Uses debouncing to batch rapid file changes and reduce cache thrashing
func (s *MCPServer) cacheRefreshCoordinator(ctx context.Context) {
	var debounceTimer *time.Timer
	pendingEvents := make(map[string]models.FileEvent)

	processPendingEvents := func() {
		if len(pendingEvents) == 0 {
			return
		}

		refreshStart := time.Now()
		affectedFiles := make([]string, 0, len(pendingEvents))

		s.logger.WithContext("pending_events", len(pendingEvents)).
			Info("Processing pending file events for cache refresh")

		for path, event := range pendingEvents {
			s.processFileEventForCache(event)
			affectedFiles = append(affectedFiles, path)
			delete(pendingEvents, path)
		}

		refreshDuration := time.Since(refreshStart)

		s.loggingManager.LogCacheRefresh("batch_refresh", affectedFiles, refreshDuration, true)

		s.logger.WithContext("total_documents", s.cache.Size()).
			WithContext("cache_hit_ratio", s.cache.GetCacheHitRatio()).
			Info("Cache refresh completed")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownChan:
			return
		case event := <-s.refreshChan:
			pendingEvents[event.Path] = event

			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			// 500ms debounce window balances responsiveness with batching efficiency
			debounceTimer = time.AfterFunc(500*time.Millisecond, processPendingEvents)

		case <-time.After(5 * time.Second):
			// Safety mechanism: process events even if debounce timer fails
			if len(pendingEvents) > 0 {
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				processPendingEvents()
			}
		}
	}
}

// processFileEventForCache handles individual file events for cache updates
func (s *MCPServer) processFileEventForCache(event models.FileEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eventStart := time.Now()
	defer func() {
		s.loggingManager.LogFileSystemEvent(event.Type, event.Path, time.Since(eventStart))
	}()

	switch event.Type {
	case "create", "modify":
		s.handleCreateOrModifyEvent(event)
	case "delete":
		s.handleDeleteEvent(event)
	}
}

// handleCreateOrModifyEvent processes file creation or modification events
func (s *MCPServer) handleCreateOrModifyEvent(event models.FileEvent) {
	metadata, err := s.scanner.ParseMarkdownFile(event.Path)
	if err != nil {
		s.logger.WithError(err).
			WithContext("file_path", event.Path).
			WithContext("event_type", event.Type).
			Error("Error parsing updated file")
		s.degradationManager.RecordError(errors.ComponentDocumentParsing, err)
		return
	}

	if err := s.loadDocumentIntoCache(*metadata); err != nil {
		s.logger.WithError(err).
			WithContext("file_path", event.Path).
			WithContext("event_type", event.Type).
			Error("Error loading updated document")
		s.degradationManager.RecordError(errors.ComponentCacheRefresh, err)
		return
	}

	s.updateCategoryIndex(metadata.Category)

	s.logger.WithContext("file_path", event.Path).
		WithContext("event_type", event.Type).
		WithContext("category", metadata.Category).
		Info("Updated cache for file")
}

// handleDeleteEvent processes file deletion events
func (s *MCPServer) handleDeleteEvent(event models.FileEvent) {
	s.cache.Invalidate(event.Path)

	category := s.getCategoryFromPath(event.Path)
	s.updateCategoryIndex(category)

	s.logger.WithContext("file_path", event.Path).
		WithContext("category", category).
		Info("Removed deleted file from cache")
}

// updateCategoryIndex rebuilds the index for a specific category
func (s *MCPServer) updateCategoryIndex(category string) {
	// Get all documents for this category from cache
	documents := s.cache.GetByCategory(category)

	// Build new index
	var docMetadata []models.DocumentMetadata
	for _, doc := range documents {
		docMetadata = append(docMetadata, doc.Metadata)
	}

	newIndex := &models.DocumentIndex{
		Category:  category,
		Documents: docMetadata,
		Count:     len(docMetadata),
	}

	// Update cache with new index
	s.cache.SetIndex(category, newIndex)
}

// getCategoryFromPath determines category from file path
func (s *MCPServer) getCategoryFromPath(path string) string {
	normalizedPath := filepath.ToSlash(strings.ToLower(path))

	if strings.Contains(normalizedPath, config.URIGuidelines) {
		return config.CategoryGuideline
	}
	if strings.Contains(normalizedPath, config.URIPatterns) {
		return config.CategoryPattern
	}
	if strings.Contains(normalizedPath, config.URIADR) {
		return config.CategoryADR
	}
	return config.CategoryUnknown
}
