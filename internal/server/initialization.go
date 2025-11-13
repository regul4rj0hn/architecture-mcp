package server

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
	"mcp-architecture-service/pkg/logging"
	"mcp-architecture-service/pkg/prompts"
	"mcp-architecture-service/pkg/tools"
)

// initializeDocumentationSystem sets up the documentation scanning and monitoring with concurrent processing
func (s *MCPServer) initializeDocumentationSystem(ctx context.Context) error {
	// Define documentation directories to scan
	docDirs := []string{
		config.GuidelinesPath,
		config.PatternsPath,
		config.ADRPath,
	}

	// Populate initial cache using concurrent scanner
	scanStart := time.Now()
	s.logger.Info("Scanning documentation directories for initial cache population (concurrent mode)")

	// Use concurrent initialization
	return s.initializeDocumentationSystemConcurrent(ctx, docDirs, scanStart)
}

// initializeDocumentationSystemConcurrent performs concurrent initialization for better startup performance
func (s *MCPServer) initializeDocumentationSystemConcurrent(ctx context.Context, docDirs []string, scanStart time.Time) error {
	// Channel for coordinating concurrent operations
	type initResult struct {
		operation string
		err       error
		data      interface{}
	}

	resultChan := make(chan initResult, 2) // Buffer for scanning and monitoring setup

	// Start concurrent scanning
	go func() {
		indexes, err := s.scanner.BuildIndex(docDirs)
		resultChan <- initResult{
			operation: "scanning",
			err:       err,
			data:      indexes,
		}
	}()

	// Start concurrent monitoring setup
	go func() {
		err := s.setupFileSystemMonitoring(docDirs)
		resultChan <- initResult{
			operation: "monitoring",
			err:       err,
			data:      nil,
		}
	}()

	// Collect results from concurrent operations
	var scanIndexes map[string]*models.DocumentIndex
	var scanErrors []string
	var monitoringErr error

	for i := 0; i < 2; i++ {
		select {
		case result := <-resultChan:
			switch result.operation {
			case "scanning":
				if result.err != nil {
					scanErrors = append(scanErrors, result.err.Error())
					s.logger.WithError(result.err).Warn("Failed to build initial index")
				} else {
					scanIndexes = result.data.(map[string]*models.DocumentIndex)
				}
			case "monitoring":
				monitoringErr = result.err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	scanDuration := time.Since(scanStart)

	// Process scan results and load documents concurrently
	if scanIndexes != nil {
		totalDocs := s.processScanResultsConcurrent(scanIndexes, &scanErrors)

		// Log overall scan results
		scanLogger := s.loggingManager.GetLogger("scanner").
			WithContext("scan_directory", "initial_scan").
			WithContext("documents_found", totalDocs).
			WithContext("error_count", len(scanErrors)).
			WithContext("duration_ms", scanDuration.Milliseconds())

		if len(scanErrors) > 0 {
			errorSample := scanErrors
			if len(scanErrors) > 5 {
				errorSample = scanErrors[:5]
			}
			scanLogger.WithContext("sample_errors", errorSample).Warn("Document scan completed with errors")
		} else {
			scanLogger.Info("Document scan completed successfully")
		}
	}

	// Handle monitoring setup result
	if monitoringErr != nil {
		s.logger.WithError(monitoringErr).Warn("File system monitoring setup had issues")
	}

	s.logger.WithContext("total_documents", s.cache.Size()).
		WithContext("startup_time_ms", scanDuration.Milliseconds()).
		Info("Documentation system initialized successfully with concurrent processing")
	return nil
}

// initializePromptsSystem loads prompts and sets up monitoring
func (s *MCPServer) initializePromptsSystem() error {
	s.logger.Info("Initializing prompts system")

	// Load prompt definitions
	if err := s.promptManager.LoadPrompts(); err != nil {
		s.logger.WithError(err).Warn("Failed to load prompts")
		return err
	}

	// Get loaded prompts count
	loadedPrompts := s.promptManager.ListPrompts()
	s.logger.WithContext("prompt_count", len(loadedPrompts)).
		Info("Prompts loaded successfully")

	// Set up file system monitoring for prompts directory
	if s.monitor != nil {
		if err := s.promptManager.StartWatching(); err != nil {
			s.logger.WithError(err).Warn("Failed to start prompts directory monitoring")
			// Don't return error - prompts can still work without hot-reload
		}
	}

	s.logger.Info("Prompts system initialized successfully")
	return nil
}

// processScanResultsConcurrent processes scan results and loads documents concurrently
func (s *MCPServer) processScanResultsConcurrent(indexes map[string]*models.DocumentIndex, scanErrors *[]string) int {
	var totalDocs int

	// Collect all documents to load
	var allDocuments []models.DocumentMetadata
	for category, index := range indexes {
		s.cache.SetIndex(category, index)
		totalDocs += index.Count

		s.logger.WithContext("category", category).
			WithContext("document_count", index.Count).
			Info("Cached documents for category")

		allDocuments = append(allDocuments, index.Documents...)
	}

	// Load documents concurrently
	if len(allDocuments) > 0 {
		s.loadDocumentsConcurrent(allDocuments, scanErrors)
	}

	return totalDocs
}

// loadDocumentsConcurrent loads multiple documents into cache concurrently
// Worker pool size is capped to balance parallelism with resource usage
func (s *MCPServer) loadDocumentsConcurrent(documents []models.DocumentMetadata, scanErrors *[]string) {
	numWorkers := min(runtime.NumCPU(), len(documents))
	if numWorkers > 4 {
		numWorkers = 4
	}

	docChan := make(chan models.DocumentMetadata, len(documents))
	errorChan := make(chan error, len(documents))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for doc := range docChan {
				if err := s.loadDocumentIntoCache(doc); err != nil {
					errorChan <- fmt.Errorf("failed to load %s: %v", doc.Path, err)
				}
			}
		}()
	}

	for _, doc := range documents {
		docChan <- doc
	}
	close(docChan)

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	for err := range errorChan {
		*scanErrors = append(*scanErrors, err.Error())
		s.logger.WithError(err).Warn("Failed to load document into cache")
	}
}

// setupFileSystemMonitoring sets up file system monitoring for all directories
func (s *MCPServer) setupFileSystemMonitoring(docDirs []string) error {
	if s.monitor == nil {
		return fmt.Errorf("file system monitor not available")
	}

	var setupErrors []error

	for _, dir := range docDirs {
		if _, err := os.Stat(dir); err == nil {
			err := s.monitor.WatchDirectory(dir, s.handleFileEvent)
			if err != nil {
				setupErrors = append(setupErrors, fmt.Errorf("failed to watch %s: %v", dir, err))
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

	if len(setupErrors) > 0 {
		return fmt.Errorf("monitoring setup had %d errors", len(setupErrors))
	}

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

// initializeToolsSystem sets up the tools system with registered tools
func (s *MCPServer) initializeToolsSystem() error {
	s.logger.Info("Initializing tools system")

	// Create tool manager with logger
	loggingManager := logging.NewLoggingManager()
	toolLogger := loggingManager.GetLogger("tools")
	s.toolManager = tools.NewToolManager(toolLogger)

	// Register built-in tools
	var registrationErrors []error

	// Register ValidatePatternTool
	validateTool := tools.NewValidatePatternTool(s.cache, toolLogger)
	if err := s.toolManager.RegisterTool(validateTool); err != nil {
		s.logger.WithError(err).
			WithContext("tool", validateTool.Name()).
			Error("Failed to register ValidatePatternTool")
		registrationErrors = append(registrationErrors, fmt.Errorf("ValidatePatternTool: %w", err))
	} else {
		s.logger.WithContext("tool", validateTool.Name()).
			Info("Registered tool successfully")
	}

	// Register SearchArchitectureTool
	searchTool := tools.NewSearchArchitectureTool(s.cache, toolLogger)
	if err := s.toolManager.RegisterTool(searchTool); err != nil {
		s.logger.WithError(err).
			WithContext("tool", searchTool.Name()).
			Error("Failed to register SearchArchitectureTool")
		registrationErrors = append(registrationErrors, fmt.Errorf("SearchArchitectureTool: %w", err))
	} else {
		s.logger.WithContext("tool", searchTool.Name()).
			Info("Registered tool successfully")
	}

	// Register CheckADRAlignmentTool
	adrTool := tools.NewCheckADRAlignmentTool(s.cache, toolLogger)
	if err := s.toolManager.RegisterTool(adrTool); err != nil {
		s.logger.WithError(err).
			WithContext("tool", adrTool.Name()).
			Error("Failed to register CheckADRAlignmentTool")
		registrationErrors = append(registrationErrors, fmt.Errorf("CheckADRAlignmentTool: %w", err))
	} else {
		s.logger.WithContext("tool", adrTool.Name()).
			Info("Registered tool successfully")
	}

	// Validate all registered tools
	registeredTools := s.toolManager.ListTools()
	s.logger.WithContext("tool_count", len(registeredTools)).
		Info("Tools validation complete")

	// Log any registration errors but don't fail initialization
	if len(registrationErrors) > 0 {
		s.logger.WithContext("error_count", len(registrationErrors)).
			Warn("Some tools failed to register")
		// Tools system can still function with partial registration
	}

	// Inject tool manager into prompt manager for prompt-tool integration
	if s.promptManager != nil {
		// Create adapter to bridge tools.ToolManager and prompts.ToolManagerInterface
		adapter := &toolManagerAdapter{tm: s.toolManager}
		s.promptManager.SetToolManager(adapter)
		s.logger.Info("Tool manager injected into prompt manager for tool reference expansion")
	}

	s.logger.Info("Tools system initialized successfully")
	return nil
}

// toolManagerAdapter adapts tools.ToolManager to prompts.ToolManagerInterface
type toolManagerAdapter struct {
	tm *tools.ToolManager
}

func (a *toolManagerAdapter) GetTool(name string) (prompts.ToolInterface, error) {
	return a.tm.GetTool(name)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
