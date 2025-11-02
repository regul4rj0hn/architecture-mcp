package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/errors"
	"mcp-architecture-service/pkg/logging"
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

	// Error handling and degradation
	circuitBreakerManager *errors.CircuitBreakerManager
	degradationManager    *errors.GracefulDegradationManager

	// Logging
	loggingManager *logging.LoggingManager
	logger         *logging.StructuredLogger

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

	// Initialize logging system
	loggingManager := logging.NewLoggingManager()
	loggingManager.SetGlobalContext("service", "mcp-architecture-service")
	loggingManager.SetGlobalContext("version", "1.0.0")
	logger := loggingManager.GetLogger("server")

	// Initialize error handling components
	circuitBreakerManager := errors.NewCircuitBreakerManager()
	degradationManager := errors.NewGracefulDegradationManager()

	// Register default degradation rules
	for _, rule := range errors.CreateDefaultRules() {
		degradationManager.RegisterComponent(rule)
	}

	fileMonitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		loggingManager.LogError("server", err, "Failed to create file system monitor", map[string]interface{}{
			"component": "file_monitor",
		})
		fileMonitor = nil
		// Record error for degradation management
		degradationManager.RecordError(errors.ComponentFileSystemMonitoring, err)
	}

	server := &MCPServer{
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

		// Error handling
		circuitBreakerManager: circuitBreakerManager,
		degradationManager:    degradationManager,

		// Logging
		loggingManager: loggingManager,
		logger:         logger,

		// Coordination channels
		refreshChan:  make(chan models.FileEvent, 100), // Buffered channel for file events
		shutdownChan: make(chan struct{}),
	}

	// Set up degradation state change callback
	degradationManager.SetStateChangeCallback(server.onDegradationStateChange)

	// Set up circuit breaker callbacks
	server.setupCircuitBreakerCallbacks()

	return server
}

// Start begins the MCP server operation
func (s *MCPServer) Start(ctx context.Context) error {
	startTime := time.Now()

	s.loggingManager.LogStartupSequence("server_start", map[string]interface{}{
		"phase": "initialization",
	}, 0, true)

	// Initialize documentation system
	docInitStart := time.Now()
	if err := s.initializeDocumentationSystem(ctx); err != nil {
		s.loggingManager.LogStartupSequence("documentation_init", map[string]interface{}{
			"error": err.Error(),
		}, time.Since(docInitStart), false)
		s.logger.WithError(err).Warn("Failed to initialize documentation system")
	} else {
		s.loggingManager.LogStartupSequence("documentation_init", map[string]interface{}{},
			time.Since(docInitStart), true)
	}

	// Start cache refresh coordinator
	go s.cacheRefreshCoordinator(ctx)

	s.loggingManager.LogStartupSequence("server_ready", map[string]interface{}{
		"total_startup_time_ms": time.Since(startTime).Milliseconds(),
	}, time.Since(startTime), true)

	s.logger.Info("MCP Architecture Service started successfully")

	// Start JSON-RPC message processing loop
	return s.processMessages(ctx, os.Stdin, os.Stdout)
}

// Shutdown gracefully shuts down the MCP server
func (s *MCPServer) Shutdown(ctx context.Context) error {
	shutdownStart := time.Now()

	s.loggingManager.LogShutdownSequence("shutdown_start", map[string]interface{}{}, 0, true)

	// Signal shutdown to background goroutines
	close(s.shutdownChan)

	// Stop file system monitoring
	monitorShutdownStart := time.Now()
	if s.monitor != nil {
		if err := s.monitor.StopWatching(); err != nil {
			s.loggingManager.LogShutdownSequence("monitor_stop", map[string]interface{}{
				"error": err.Error(),
			}, time.Since(monitorShutdownStart), false)
			s.logger.WithError(err).Error("Error stopping file monitor")
		} else {
			s.loggingManager.LogShutdownSequence("monitor_stop", map[string]interface{}{},
				time.Since(monitorShutdownStart), true)
		}
	}

	// Clear cache
	cacheShutdownStart := time.Now()
	s.cache.Clear()
	s.loggingManager.LogShutdownSequence("cache_clear", map[string]interface{}{},
		time.Since(cacheShutdownStart), true)

	s.loggingManager.LogShutdownSequence("shutdown_complete", map[string]interface{}{
		"total_shutdown_time_ms": time.Since(shutdownStart).Milliseconds(),
	}, time.Since(shutdownStart), true)

	s.logger.Info("MCP Architecture Service shutdown completed")

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

// HandleMessage processes individual MCP messages (exported for testing)
func (s *MCPServer) HandleMessage(message *models.MCPMessage) *models.MCPMessage {
	return s.handleMessage(message)
}

// handleMessage processes individual MCP messages
func (s *MCPServer) handleMessage(message *models.MCPMessage) *models.MCPMessage {
	startTime := time.Now()
	var response *models.MCPMessage
	var success bool = true
	var errorMsg string

	defer func() {
		duration := time.Since(startTime)
		s.loggingManager.LogMCPRequest(message.Method, message.ID, duration, success, errorMsg)
	}()

	switch message.Method {
	case "initialize":
		response = s.handleInitialize(message)
	case "notifications/initialized":
		response = s.handleInitialized(message)
	case "resources/list":
		response = s.handleResourcesList(message)
	case "resources/read":
		response = s.handleResourcesRead(message)
	default:
		success = false
		errorMsg = "Method not found"
		response = s.createErrorResponse(message.ID, -32601, "Method not found")
	}

	// Check if response contains an error
	if response != nil && response.Error != nil {
		success = false
		errorMsg = response.Error.Message
	}

	return response
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
	s.logger.WithContext("request_id", message.ID).Info("MCP server initialized successfully")
	return nil // No response for notifications
}

// handleResourcesList handles the resources/list method
func (s *MCPServer) handleResourcesList(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var resources []models.MCPResource

	// Get all cached documents and convert them to MCP resources
	allDocuments := s.cache.GetAllDocuments()

	for _, doc := range allDocuments {
		resource := s.createMCPResourceFromDocument(doc)
		resources = append(resources, resource)
	}

	result := models.MCPResourcesListResult{
		Resources: resources,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleResourcesRead handles the resources/read method
func (s *MCPServer) handleResourcesRead(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse request parameters
	var params models.MCPResourcesReadParams
	if message.Params != nil {
		paramsBytes, err := json.Marshal(message.Params)
		if err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters")
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.createErrorResponse(message.ID, -32602, "Invalid parameters format")
		}
	}

	// Validate URI parameter
	if params.URI == "" {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Missing required parameter: uri", nil)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Parse the MCP resource URI
	category, path, err := s.parseResourceURI(params.URI)
	if err != nil {
		structuredErr := errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid resource URI", err).WithContext("uri", params.URI)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Find the document in cache with circuit breaker protection
	circuitBreaker := s.circuitBreakerManager.GetOrCreate("resource_read",
		errors.DefaultCircuitBreakerConfig("resource_read"))

	var document *models.Document
	err = circuitBreaker.Execute(func() error {
		var findErr error
		document, findErr = s.findDocumentByResourcePath(category, path)
		return findErr
	})

	if err != nil {
		// Check if it's a circuit breaker error or actual resource error
		if structuredErr, ok := err.(*errors.StructuredError); ok {
			return s.createStructuredErrorResponse(message.ID, structuredErr)
		}

		structuredErr := errors.NewMCPError(errors.ErrCodeResourceNotFound,
			"Resource not found", err).
			WithContext("uri", params.URI).
			WithContext("category", category).
			WithContext("path", path)
		return s.createStructuredErrorResponse(message.ID, structuredErr)
	}

	// Create resource content response
	content := models.MCPResourceContent{
		URI:      params.URI,
		MimeType: "text/markdown",
		Text:     document.Content.RawContent,
	}

	result := models.MCPResourcesReadResult{
		Contents: []models.MCPResourceContent{content},
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
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

// createStructuredErrorResponse creates an MCP error response from a structured error
func (s *MCPServer) createStructuredErrorResponse(id interface{}, structuredErr *errors.StructuredError) *models.MCPMessage {
	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   structuredErr.ToMCPError(),
	}
}

// setupCircuitBreakerCallbacks sets up callbacks for circuit breaker state changes
func (s *MCPServer) setupCircuitBreakerCallbacks() {
	// This would be called when creating circuit breakers, but since we create them
	// on-demand, we'll set up the callback in the circuit breaker creation
}

// onDegradationStateChange handles degradation state changes
func (s *MCPServer) onDegradationStateChange(component errors.ServiceComponent, oldLevel, newLevel errors.DegradationLevel) {
	s.loggingManager.LogDegradationStateChange(component, oldLevel, newLevel)

	// Take specific actions based on component and degradation level
	switch component {
	case errors.ComponentFileSystemMonitoring:
		if newLevel != errors.DegradationNone {
			s.logger.WithContext("action", "switch_to_periodic_scanning").
				Warn("File system monitoring degraded - switching to periodic scanning")
		} else {
			s.logger.WithContext("action", "resume_realtime_monitoring").
				Info("File system monitoring recovered - resuming real-time monitoring")
		}
	case errors.ComponentCacheRefresh:
		if newLevel != errors.DegradationNone {
			s.logger.WithContext("action", "disable_automatic_updates").
				Warn("Cache refresh degraded - disabling automatic updates")
		}
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
	scanStart := time.Now()
	s.logger.Info("Scanning documentation directories for initial cache population")

	indexes, err := s.scanner.BuildIndex(docDirs)
	scanDuration := time.Since(scanStart)

	var totalDocs int
	var scanErrors []string

	if err != nil {
		scanErrors = append(scanErrors, err.Error())
		s.logger.WithError(err).Warn("Failed to build initial index")
	}

	// Store indexes in cache
	for category, index := range indexes {
		s.cache.SetIndex(category, index)
		totalDocs += index.Count

		s.logger.WithContext("category", category).
			WithContext("document_count", index.Count).
			Info("Cached documents for category")

		// Load individual documents into cache
		for _, docMeta := range index.Documents {
			if err := s.loadDocumentIntoCache(docMeta); err != nil {
				scanErrors = append(scanErrors, fmt.Sprintf("Failed to load %s: %v", docMeta.Path, err))
				s.logger.WithError(err).
					WithContext("document_path", docMeta.Path).
					Warn("Failed to load document into cache")
			}
		}
	}

	// Log overall scan results
	s.loggingManager.LogDocumentScan("initial_scan", totalDocs, scanErrors, scanDuration)

	// Set up file system monitoring if available
	if s.monitor != nil {
		for _, dir := range docDirs {
			if _, err := os.Stat(dir); err == nil {
				err := s.monitor.WatchDirectory(dir, s.handleFileEvent)
				if err != nil {
					s.logger.WithError(err).
						WithContext("directory", dir).
						Warn("Failed to watch directory")
					s.degradationManager.RecordError(errors.ComponentFileSystemMonitoring, err)
				} else {
					s.logger.WithContext("directory", dir).
						Info("Started monitoring directory")
				}
			}
		}
	}

	s.logger.WithContext("total_documents", s.cache.Size()).
		Info("Documentation system initialized successfully")
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

// createMCPResourceFromDocument converts a Document to an MCPResource
func (s *MCPServer) createMCPResourceFromDocument(doc *models.Document) models.MCPResource {
	// Generate MCP resource URI based on category
	uri := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)

	// Create description from title and category
	description := fmt.Sprintf("%s document", strings.Title(doc.Metadata.Category))
	if doc.Metadata.Title != "" {
		description = fmt.Sprintf("%s: %s", strings.Title(doc.Metadata.Category), doc.Metadata.Title)
	}

	// Create annotations with metadata
	annotations := map[string]string{
		"category":     doc.Metadata.Category,
		"path":         doc.Metadata.Path,
		"lastModified": doc.Metadata.LastModified.Format(time.RFC3339),
		"size":         fmt.Sprintf("%d", doc.Metadata.Size),
		"checksum":     doc.Metadata.Checksum,
	}

	return models.MCPResource{
		URI:         uri,
		Name:        doc.Metadata.Title,
		Description: description,
		MimeType:    "text/markdown",
		Annotations: annotations,
	}
}

// generateResourceURI creates an MCP resource URI based on category and path
func (s *MCPServer) generateResourceURI(category, path string) string {
	// Remove file extension and normalize path
	cleanPath := strings.TrimSuffix(path, ".md")
	cleanPath = filepath.ToSlash(cleanPath)

	// Remove category prefix from path if present
	switch category {
	case "guideline":
		cleanPath = strings.TrimPrefix(cleanPath, "docs/guidelines/")
		return fmt.Sprintf("architecture://guidelines/%s", cleanPath)
	case "pattern":
		cleanPath = strings.TrimPrefix(cleanPath, "docs/patterns/")
		return fmt.Sprintf("architecture://patterns/%s", cleanPath)
	case "adr":
		cleanPath = strings.TrimPrefix(cleanPath, "docs/adr/")
		// For ADRs, extract ADR ID from filename if possible
		adrId := s.extractADRId(cleanPath)
		return fmt.Sprintf("architecture://adr/%s", adrId)
	default:
		return fmt.Sprintf("architecture://unknown/%s", cleanPath)
	}
}

// extractADRId extracts ADR ID from filename or path
func (s *MCPServer) extractADRId(path string) string {
	// Get the base filename
	filename := filepath.Base(path)

	// Try to extract ADR number from common patterns like "001-api-design" or "adr-001"
	patterns := []string{
		`^(\d+)-`,    // "001-api-design"
		`^adr-(\d+)`, // "adr-001"
		`^ADR-(\d+)`, // "ADR-001"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(filename); len(matches) > 1 {
			return matches[1]
		}
	}

	// If no pattern matches, use the filename without extension
	return filename
}

// parseResourceURI parses an MCP resource URI and returns category and path
func (s *MCPServer) parseResourceURI(uri string) (category, path string, err error) {
	// Expected URI patterns:
	// architecture://guidelines/{path}
	// architecture://patterns/{path}
	// architecture://adr/{adr_id}

	if !strings.HasPrefix(uri, "architecture://") {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid URI scheme, expected 'architecture://'", nil).
			WithContext("uri", uri)
	}

	// Remove the scheme prefix
	remainder := strings.TrimPrefix(uri, "architecture://")

	// Split into category and path
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) < 2 {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid URI format, expected 'architecture://{category}/{path}'", nil).
			WithContext("uri", uri)
	}

	category = parts[0]
	path = parts[1]

	// Validate path is not empty and doesn't start with slash
	if path == "" || strings.HasPrefix(path, "/") {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid URI format, path cannot be empty or start with '/'", nil).
			WithContext("uri", uri).
			WithContext("path", path)
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return "", "", errors.NewValidationError(errors.ErrCodePathTraversal,
			"Path traversal attempt detected", nil).
			WithContext("uri", uri).
			WithContext("path", path)
	}

	// Validate category
	switch category {
	case "guidelines":
		return "guideline", path, nil
	case "patterns":
		return "pattern", path, nil
	case "adr":
		return "adr", path, nil
	default:
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidCategory,
			"Unsupported resource category", nil).
			WithContext("uri", uri).
			WithContext("category", category)
	}
}

// findDocumentByResourcePath finds a document in the cache by category and resource path
func (s *MCPServer) findDocumentByResourcePath(category, resourcePath string) (*models.Document, error) {
	// Record this operation for degradation management
	err := s.degradationManager.ExecuteWithDegradation(
		errors.ComponentResourceDiscovery,
		func() error {
			// Get all documents for the category
			documents := s.cache.GetByCategory(category)

			if len(documents) == 0 {
				return errors.NewCacheError(errors.ErrCodeCacheMiss,
					"No documents found for category", nil).
					WithContext("category", category)
			}
			return nil
		},
		func(level errors.DegradationLevel) error {
			// Degraded behavior - return empty result
			return errors.NewSystemError("SERVICE_DEGRADED",
				"Resource discovery is degraded", nil).
				WithContext("degradation_level", level.String())
		},
	)

	if err != nil {
		return nil, err
	}

	documents := s.cache.GetByCategory(category)

	// For each document, generate its resource URI and compare with the requested path
	for _, doc := range documents {
		docResourceURI := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)

		// Extract the path part from the generated URI for comparison
		_, docResourcePath, err := s.parseResourceURI(docResourceURI)
		if err != nil {
			continue // Skip malformed URIs
		}

		// Compare paths (case-insensitive)
		if strings.EqualFold(docResourcePath, resourcePath) {
			return doc, nil
		}
	}

	// If no exact match found, try direct path lookup in cache
	// This handles cases where the resource path might be a direct file path
	possiblePaths := s.generatePossibleFilePaths(category, resourcePath)

	for _, possiblePath := range possiblePaths {
		if doc, err := s.cache.Get(possiblePath); err == nil {
			return doc, nil
		}
	}

	return nil, errors.NewMCPError(errors.ErrCodeResourceNotFound,
		"Document not found for path", nil).
		WithContext("category", category).
		WithContext("resourcePath", resourcePath)
}

// generatePossibleFilePaths generates possible file paths for a given category and resource path
func (s *MCPServer) generatePossibleFilePaths(category, resourcePath string) []string {
	var paths []string

	// Add .md extension if not present
	if !strings.HasSuffix(resourcePath, ".md") {
		resourcePath += ".md"
	}

	switch category {
	case "guideline":
		paths = append(paths, filepath.Join("docs/guidelines", resourcePath))
		paths = append(paths, filepath.Join("docs", "guidelines", resourcePath))
	case "pattern":
		paths = append(paths, filepath.Join("docs/patterns", resourcePath))
		paths = append(paths, filepath.Join("docs", "patterns", resourcePath))
	case "adr":
		// For ADRs, try different naming patterns
		adrId := strings.TrimSuffix(resourcePath, ".md")

		// Try various ADR naming patterns
		patterns := []string{
			fmt.Sprintf("%s.md", adrId),
			fmt.Sprintf("adr-%s.md", adrId),
			fmt.Sprintf("ADR-%s.md", adrId),
			fmt.Sprintf("%03s.md", adrId), // Zero-padded numbers
		}

		for _, pattern := range patterns {
			paths = append(paths, filepath.Join("docs/adr", pattern))
			paths = append(paths, filepath.Join("docs", "adr", pattern))
		}

		// Also try to find by ADR ID in existing documents
		allDocs := s.cache.GetByCategory("adr")
		for _, doc := range allDocs {
			docURI := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)
			if strings.Contains(docURI, adrId) {
				paths = append(paths, doc.Metadata.Path)
			}
		}
	}

	return paths
}
