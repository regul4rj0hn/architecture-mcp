package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/monitor"
	"mcp-architecture-service/pkg/scanner"
)

// MCPServer represents the main MCP server
type MCPServer struct {
	serverInfo   models.MCPServerInfo
	capabilities models.MCPCapabilities
	initialized  bool

	// Documentation system components
	cache   *cache.DocumentCache
	scanner *scanner.DocumentationScanner
	monitor *monitor.FileSystemMonitor

	// Coordination channels
	refreshChan  chan models.FileEvent
	shutdownChan chan struct{}

	// Synchronization
	mu sync.RWMutex
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer() *MCPServer {
	// Initialize documentation system components
	docCache := cache.NewDocumentCache()
	docScanner := scanner.NewDocumentationScanner(".")

	fileMonitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		log.Printf("Warning: Failed to create file system monitor: %v", err)
		fileMonitor = nil
	}

	return &MCPServer{
		serverInfo: models.MCPServerInfo{
			Name:    "mcp-architecture-service",
			Version: "1.0.0",
		},
		capabilities: models.MCPCapabilities{
			Resources: &models.MCPResourceCapabilities{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		initialized: false,

		// Documentation system
		cache:   docCache,
		scanner: docScanner,
		monitor: fileMonitor,

		// Coordination channels
		refreshChan:  make(chan models.FileEvent, 100), // Buffered channel for file events
		shutdownChan: make(chan struct{}),
	}
}

// Start begins the MCP server operation
func (s *MCPServer) Start(ctx context.Context) error {
	log.Println("Starting MCP Architecture Service...")

	// Initialize documentation system
	if err := s.initializeDocumentationSystem(ctx); err != nil {
		log.Printf("Warning: Failed to initialize documentation system: %v", err)
	}

	// Start cache refresh coordinator
	go s.cacheRefreshCoordinator(ctx)

	// Start JSON-RPC message processing loop
	return s.processMessages(ctx, os.Stdin, os.Stdout)
}

// Shutdown gracefully shuts down the MCP server
func (s *MCPServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down MCP Architecture Service...")

	// Signal shutdown to background goroutines
	close(s.shutdownChan)

	// Stop file system monitoring
	if s.monitor != nil {
		if err := s.monitor.StopWatching(); err != nil {
			log.Printf("Error stopping file monitor: %v", err)
		}
	}

	// Clear cache
	s.cache.Clear()

	return nil
}

// processMessages handles the JSON-RPC message processing loop
func (s *MCPServer) processMessages(ctx context.Context, reader io.Reader, writer io.Writer) error {
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(writer)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var message models.MCPMessage
			if err := decoder.Decode(&message); err != nil {
				if err == io.EOF {
					return nil
				}
				log.Printf("Error decoding message: %v", err)
				continue
			}

			response := s.handleMessage(&message)
			if response != nil {
				if err := encoder.Encode(response); err != nil {
					log.Printf("Error encoding response: %v", err)
				}
			}
		}
	}
}

// handleMessage processes individual MCP messages
func (s *MCPServer) handleMessage(message *models.MCPMessage) *models.MCPMessage {
	switch message.Method {
	case "initialize":
		return s.handleInitialize(message)
	case "notifications/initialized":
		return s.handleInitialized(message)
	case "resources/list":
		return s.handleResourcesList(message)
	case "resources/read":
		return s.handleResourcesRead(message)
	default:
		return s.createErrorResponse(message.ID, -32601, "Method not found")
	}
}

// handleInitialize handles the MCP initialize method
func (s *MCPServer) handleInitialize(message *models.MCPMessage) *models.MCPMessage {
	result := models.MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    s.capabilities,
		ServerInfo:      s.serverInfo,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleInitialized handles the notifications/initialized method
func (s *MCPServer) handleInitialized(message *models.MCPMessage) *models.MCPMessage {
	s.initialized = true
	log.Println("MCP server initialized successfully")
	return nil // No response for notifications
}

// handleResourcesList handles the resources/list method
func (s *MCPServer) handleResourcesList(message *models.MCPMessage) *models.MCPMessage {
	// Return empty list for now - will be implemented in later tasks
	result := models.MCPResourcesListResult{
		Resources: []models.MCPResource{},
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleResourcesRead handles the resources/read method
func (s *MCPServer) handleResourcesRead(message *models.MCPMessage) *models.MCPMessage {
	// Return error for now - will be implemented in later tasks
	return s.createErrorResponse(message.ID, -32602, "Resource reading not yet implemented")
}

// createErrorResponse creates an MCP error response
func (s *MCPServer) createErrorResponse(id interface{}, code int, message string) *models.MCPMessage {
	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &models.MCPError{
			Code:    code,
			Message: message,
		},
	}
}

// initializeDocumentationSystem sets up the documentation scanning and monitoring
func (s *MCPServer) initializeDocumentationSystem(ctx context.Context) error {
	// Define documentation directories to scan
	docDirs := []string{
		"docs/guidelines",
		"docs/patterns",
		"docs/adr",
	}

	// Populate initial cache using scanner
	log.Println("Scanning documentation directories for initial cache population...")

	indexes, err := s.scanner.BuildIndex(docDirs)
	if err != nil {
		log.Printf("Warning: Failed to build initial index: %v", err)
	}

	// Store indexes in cache
	for category, index := range indexes {
		s.cache.SetIndex(category, index)
		log.Printf("Cached %d documents for category '%s'", index.Count, category)

		// Load individual documents into cache
		for _, docMeta := range index.Documents {
			if err := s.loadDocumentIntoCache(docMeta); err != nil {
				log.Printf("Warning: Failed to load document %s: %v", docMeta.Path, err)
			}
		}
	}

	// Set up file system monitoring if available
	if s.monitor != nil {
		for _, dir := range docDirs {
			if _, err := os.Stat(dir); err == nil {
				err := s.monitor.WatchDirectory(dir, s.handleFileEvent)
				if err != nil {
					log.Printf("Warning: Failed to watch directory %s: %v", dir, err)
				}
			}
		}
	}

	log.Printf("Documentation system initialized with %d total documents", s.cache.Size())
	return nil
}

// loadDocumentIntoCache loads a document's full content into the cache
func (s *MCPServer) loadDocumentIntoCache(metadata models.DocumentMetadata) error {
	// Read the file content
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		return err
	}

	// Create document with content
	doc := &models.Document{
		Metadata: metadata,
		Content: models.DocumentContent{
			RawContent: string(content),
			Sections:   []models.DocumentSection{}, // Will be populated by parser if needed
		},
	}

	// Store in cache
	s.cache.Set(metadata.Path, doc)
	return nil
}

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
		log.Printf("Warning: Refresh channel full, dropping file event for %s", event.Path)
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

		log.Printf("Processing %d pending file events for cache refresh", len(pendingEvents))

		for path, event := range pendingEvents {
			s.processFileEventForCache(event)
			delete(pendingEvents, path)
		}

		// Log cache statistics after refresh
		log.Printf("Cache refresh complete - Documents: %d, Hit ratio: %.1f%%",
			s.cache.Size(), s.cache.GetCacheHitRatio())
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

	switch event.Type {
	case "create", "modify":
		// Parse the file and update cache
		metadata, err := s.scanner.ParseMarkdownFile(event.Path)
		if err != nil {
			log.Printf("Error parsing updated file %s: %v", event.Path, err)
			return
		}

		// Load document content into cache
		if err := s.loadDocumentIntoCache(*metadata); err != nil {
			log.Printf("Error loading updated document %s: %v", event.Path, err)
			return
		}

		// Update category index
		s.updateCategoryIndex(metadata.Category)

		log.Printf("Updated cache for %s file: %s", event.Type, event.Path)

	case "delete":
		// Remove from cache
		s.cache.Invalidate(event.Path)

		// Update category indexes - we need to determine category from path
		category := s.getCategoryFromPath(event.Path)
		s.updateCategoryIndex(category)

		log.Printf("Removed deleted file from cache: %s", event.Path)
	}
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
