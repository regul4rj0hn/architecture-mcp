package server

import (
	"context"
	"encoding/json"
	"io"
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
	"mcp-architecture-service/pkg/tools"
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

	// Tools system
	toolManager *tools.ToolManager

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

// NewMCPServerWithLogLevel creates a new MCP server with specified log level
func NewMCPServerWithLogLevel(logLevel string) *MCPServer {
	server := newMCPServerWithOptions(true)
	server.loggingManager.SetLogLevel(logLevel)
	return server
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
			loggingManager.GetLogger("server").
				WithError(err).
				WithContext("component", "file_monitor").
				Error("Failed to create file system monitor")
			fileMonitor = nil
			// Record error for degradation management
			degradationManager.RecordError(errors.ComponentFileSystemMonitoring, err)
		}
	}

	// Initialize prompt manager
	promptLogger := loggingManager.GetLogger("prompts")
	promptManager := prompts.NewPromptManager(config.PromptsBasePath, docCache, fileMonitor, promptLogger)

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
			Tools: &models.MCPToolCapabilities{
				ListChanged: false,
			},
			Completion: &models.MCPCompletionCapabilities{
				ArgumentCompletions: true,
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
	startupLogger := s.loggingManager.GetLogger("startup")

	startupLogger.WithContext("phase", "initialization").Info("Server start")

	// Initialize documentation system
	docInitStart := time.Now()
	if err := s.initializeDocumentationSystem(ctx); err != nil {
		startupLogger.WithContext("duration_ms", time.Since(docInitStart).Milliseconds()).
			WithError(err).Error("Documentation init failed")
		s.logger.WithError(err).Warn("Failed to initialize documentation system")
	} else {
		startupLogger.WithContext("duration_ms", time.Since(docInitStart).Milliseconds()).
			Info("Documentation init completed")
	}

	// Initialize prompts system
	promptInitStart := time.Now()
	if err := s.initializePromptsSystem(); err != nil {
		startupLogger.WithContext("duration_ms", time.Since(promptInitStart).Milliseconds()).
			WithError(err).Error("Prompts init failed")
		s.logger.WithError(err).Warn("Failed to initialize prompts system")
	} else {
		startupLogger.WithContext("duration_ms", time.Since(promptInitStart).Milliseconds()).
			Info("Prompts init completed")
	}

	// Initialize tools system
	toolsInitStart := time.Now()
	if err := s.initializeToolsSystem(); err != nil {
		startupLogger.WithContext("duration_ms", time.Since(toolsInitStart).Milliseconds()).
			WithError(err).Error("Tools init failed")
		s.logger.WithError(err).Warn("Failed to initialize tools system")
	} else {
		startupLogger.WithContext("duration_ms", time.Since(toolsInitStart).Milliseconds()).
			Info("Tools init completed")
	}

	// Start cache refresh coordinator
	go s.cacheRefreshCoordinator(ctx)

	startupLogger.WithContext("total_startup_time_ms", time.Since(startTime).Milliseconds()).
		Info("Server ready")

	s.logger.Info("MCP Architecture Service started successfully")

	// Start JSON-RPC message processing loop
	return s.processMessages(ctx, os.Stdin, os.Stdout)
}

// Shutdown gracefully shuts down the MCP server
func (s *MCPServer) Shutdown(ctx context.Context) error {
	shutdownStart := time.Now()
	shutdownLogger := s.loggingManager.GetLogger("shutdown")

	shutdownLogger.Info("Shutdown start")

	// Signal shutdown to background goroutines
	close(s.shutdownChan)

	// Stop file system monitoring
	monitorShutdownStart := time.Now()
	if s.monitor != nil {
		if err := s.monitor.StopWatching(); err != nil {
			shutdownLogger.WithContext("duration_ms", time.Since(monitorShutdownStart).Milliseconds()).
				WithError(err).Error("Monitor stop failed")
			s.logger.WithError(err).Error("Error stopping file monitor")
		} else {
			shutdownLogger.WithContext("duration_ms", time.Since(monitorShutdownStart).Milliseconds()).
				Info("Monitor stopped")
		}
	}

	// Clear cache and stop cleanup goroutines
	cacheShutdownStart := time.Now()
	s.cache.Close() // Stop cleanup goroutines
	s.cache.Clear()
	shutdownLogger.WithContext("duration_ms", time.Since(cacheShutdownStart).Milliseconds()).
		Info("Cache cleared")

	shutdownLogger.WithContext("total_shutdown_time_ms", time.Since(shutdownStart).Milliseconds()).
		Info("Shutdown complete")

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
				s.logger.WithError(err).Error("Error decoding message")
				continue
			}

			response := s.handleMessage(&message)
			if response != nil {
				if err := encoder.Encode(response); err != nil {
					s.logger.WithError(err).
						WithContext("method", message.Method).
						WithContext("request_id", message.ID).
						Error("Error encoding response")
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

	// Log incoming message
	s.loggingManager.GetLogger("mcp_protocol").
		WithContext("direction", "Client -> Server").
		WithContext("mcp_method", message.Method).
		WithContext("request_id", message.ID).
		Info("Client -> Server: " + message.Method)

	defer func() {
		duration := time.Since(startTime)
		mcpLogger := s.loggingManager.GetLogger("mcp_protocol").
			WithContext("direction", "Server -> Client").
			WithContext("mcp_method", message.Method).
			WithContext("request_id", message.ID).
			WithContext("duration_ms", duration.Milliseconds()).
			WithContext("success", success)

		if !success && errorMsg != "" {
			mcpLogger = mcpLogger.WithContext("error_message", errorMsg)
		}

		if success {
			mcpLogger.Info("MCP message processed successfully")
		} else {
			mcpLogger.Warn("MCP message processing failed")
		}
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
	case "tools/list":
		response = s.handleToolsList(message)
	case "tools/call":
		response = s.handleToolsCall(message)
	case "completion/complete":
		response = s.handleCompletionComplete(message)
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
	s.loggingManager.GetLogger("degradation").
		WithContext("degraded_component", string(component)).
		WithContext("old_level", oldLevel.String()).
		WithContext("new_level", newLevel.String()).
		Warn("Service degradation level changed")

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
