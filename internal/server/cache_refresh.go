package server

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
)

// handleFileEvent processes file system events and queues them for cache refresh
func (s *MCPServer) handleFileEvent(event models.FileEvent) {
	// Only process markdown files
	if !strings.HasSuffix(strings.ToLower(event.Path), ".md") {
		return
	}

	// Send event to refresh coordinator via channel
	select {
	case s.refreshChan <- event:
		// Event queued successfully
	default:
		// Channel is full, log warning but don't block
		s.logger.WithContext("event_path", event.Path).
			WithContext("event_type", event.Type).
			Warn("Refresh channel full, dropping file event")
	}
}

// cacheRefreshCoordinator coordinates cache updates from file system events
func (s *MCPServer) cacheRefreshCoordinator(ctx context.Context) {
	// Debounce timer to batch multiple rapid changes
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

		// Log cache refresh operation
		s.loggingManager.LogCacheRefresh("batch_refresh", affectedFiles, refreshDuration, true)

		// Log cache statistics after refresh
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
			// Add event to pending batch
			pendingEvents[event.Path] = event

			// Reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			debounceTimer = time.AfterFunc(500*time.Millisecond, processPendingEvents)

		case <-time.After(5 * time.Second):
			// Periodic processing to ensure events don't get stuck
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

	switch event.Type {
	case "create", "modify":
		// Parse the file and update cache
		metadata, err := s.scanner.ParseMarkdownFile(event.Path)
		if err != nil {
			s.logger.WithError(err).
				WithContext("file_path", event.Path).
				WithContext("event_type", event.Type).
				Error("Error parsing updated file")
			s.degradationManager.RecordError(errors.ComponentDocumentParsing, err)
			return
		}

		// Load document content into cache
		if err := s.loadDocumentIntoCache(*metadata); err != nil {
			s.logger.WithError(err).
				WithContext("file_path", event.Path).
				WithContext("event_type", event.Type).
				Error("Error loading updated document")
			s.degradationManager.RecordError(errors.ComponentCacheRefresh, err)
			return
		}

		// Update category index
		s.updateCategoryIndex(metadata.Category)

		s.logger.WithContext("file_path", event.Path).
			WithContext("event_type", event.Type).
			WithContext("category", metadata.Category).
			Info("Updated cache for file")

	case "delete":
		// Remove from cache
		s.cache.Invalidate(event.Path)

		// Update category indexes - we need to determine category from path
		category := s.getCategoryFromPath(event.Path)
		s.updateCategoryIndex(category)

		s.logger.WithContext("file_path", event.Path).
			WithContext("category", category).
			Info("Removed deleted file from cache")
	}

	// Log file system event processing time
	s.loggingManager.LogFileSystemEvent(event.Type, event.Path, time.Since(eventStart))
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

	if strings.Contains(normalizedPath, "guidelines") {
		return "guideline"
	}
	if strings.Contains(normalizedPath, "patterns") {
		return "pattern"
	}
	if strings.Contains(normalizedPath, "adr") {
		return "adr"
	}
	return "unknown"
}
