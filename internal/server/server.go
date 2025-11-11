package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
	"mcp-architecture-service/pkg/logging"
	"mcp-architecture-service/pkg/monitor"
	"mcp-architecture-service/pkg/prompts"
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

	// Prompts system
	promptManager *prompts.PromptManager

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
	return newMCPServerWithOptions(true)
}

// newMCPServerWithOptions creates a new MCP server with optional components
// enableMonitor controls whether file system monitoring is enabled (disabled for benchmarks)
func newMCPServerWithOptions(enableMonitor bool) *MCPServer {
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

	var fileMonitor *monitor.FileSystemMonitor
	if enableMonitor {
		var err error
		fileMonitor, err = monitor.NewFileSystemMonitor()
		if err != nil {
			loggingManager.LogError("server", err, "Failed to create file system monitor", map[string]interface{}{
				"component": "file_monitor",
			})
			fileMonitor = nil
			// Record error for degradation management
			degradationManager.RecordError(errors.ComponentFileSystemMonitoring, err)
		}
	}

	// Initialize prompt manager
	promptManager := prompts.NewPromptManager(config.PromptsBasePath, docCache, fileMonitor)

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
			Prompts: &models.MCPPromptCapabilities{
				ListChanged: false,
			},
		},
		initialized: false,

		// Documentation system
		cache:   docCache,
		scanner: docScanner,
		monitor: fileMonitor,

		// Prompts system
		promptManager: promptManager,

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

	// Initialize prompts system
	promptInitStart := time.Now()
	if err := s.initializePromptsSystem(); err != nil {
		s.loggingManager.LogStartupSequence("prompts_init", map[string]interface{}{
			"error": err.Error(),
		}, time.Since(promptInitStart), false)
		s.logger.WithError(err).Warn("Failed to initialize prompts system")
	} else {
		s.loggingManager.LogStartupSequence("prompts_init", map[string]interface{}{},
			time.Since(promptInitStart), true)
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

	// Clear cache and stop cleanup goroutines
	cacheShutdownStart := time.Now()
	s.cache.Close() // Stop cleanup goroutines
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
	case "prompts/list":
		response = s.handlePromptsList(message)
	case "prompts/get":
		response = s.handlePromptsGet(message)
	case "server/performance":
		response = s.handlePerformanceMetrics(message)
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
