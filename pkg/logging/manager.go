package logging

import (
	"strings"
	"sync"
	"time"

	"mcp-architecture-service/pkg/errors"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDEBUG LogLevel = iota
	LogLevelINFO
	LogLevelWARN
	LogLevelERROR
)

// LoggingManager manages structured logging across the application
type LoggingManager struct {
	loggers map[string]*StructuredLogger
	mutex   sync.RWMutex

	// Global context that gets added to all log entries
	globalContext LogContext

	// Statistics
	stats LoggingStats

	// Log level for filtering
	logLevel LogLevel
}

// LoggingStats tracks logging statistics
type LoggingStats struct {
	TotalMessages    int64            `json:"totalMessages"`
	MessagesByLevel  map[string]int64 `json:"messagesByLevel"`
	MessagesByLogger map[string]int64 `json:"messagesByLogger"`
	ErrorCount       int64            `json:"errorCount"`
	LastLogTime      time.Time        `json:"lastLogTime"`
}

// NewLoggingManager creates a new logging manager
func NewLoggingManager() *LoggingManager {
	return &LoggingManager{
		loggers:       make(map[string]*StructuredLogger),
		globalContext: make(LogContext),
		stats: LoggingStats{
			MessagesByLevel:  make(map[string]int64),
			MessagesByLogger: make(map[string]int64),
		},
		logLevel: LogLevelINFO, // Default to INFO level
	}
}

// GetLogger gets or creates a logger for a specific component
func (lm *LoggingManager) GetLogger(component string) *StructuredLogger {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if logger, exists := lm.loggers[component]; exists {
		return logger
	}

	// Create new logger with global context
	logger := NewStructuredLogger(component)
	logger.manager = lm // Set reference to manager for log level checks

	// Add global context to the logger
	for key, value := range lm.globalContext {
		logger = logger.WithContext(key, value)
	}

	lm.loggers[component] = logger
	return logger
}

// SetLogLevel sets the logging level for all loggers
// Accepts any string and defaults to INFO for invalid levels
func (lm *LoggingManager) SetLogLevel(level string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	// Convert to uppercase for case-insensitive matching
	upperLevel := strings.ToUpper(level)

	switch upperLevel {
	case "DEBUG":
		lm.logLevel = LogLevelDEBUG
	case "INFO":
		lm.logLevel = LogLevelINFO
	case "WARN":
		lm.logLevel = LogLevelWARN
	case "ERROR":
		lm.logLevel = LogLevelERROR
	default:
		lm.logLevel = LogLevelINFO
	}
}

// shouldLog checks if a message at the given level should be logged
func (lm *LoggingManager) shouldLog(level LogLevel) bool {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()
	return level >= lm.logLevel
}

// SetGlobalContext sets global context that will be added to all log entries
func (lm *LoggingManager) SetGlobalContext(key string, value interface{}) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.globalContext[key] = value

	// Update existing loggers with new global context
	for component, logger := range lm.loggers {
		updatedLogger := logger.WithContext(key, value)
		lm.loggers[component] = updatedLogger
	}
}

// RemoveGlobalContext removes a key from global context
func (lm *LoggingManager) RemoveGlobalContext(key string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	delete(lm.globalContext, key)

	// Note: We don't update existing loggers here as it would be complex
	// New loggers will not have the removed context
}

// GetGlobalContext returns a copy of the global context
func (lm *LoggingManager) GetGlobalContext() LogContext {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	context := make(LogContext)
	for k, v := range lm.globalContext {
		context[k] = v
	}
	return context
}

// LogApplicationEvent logs application-wide events
func (lm *LoggingManager) LogApplicationEvent(event string, details map[string]interface{}) {
	logger := lm.GetLogger("application")

	contextLogger := logger.WithContext("app_event", event)
	for k, v := range details {
		contextLogger = contextLogger.WithContext(k, v)
	}

	contextLogger.Info("Application event")
	lm.updateStats("application", "INFO")
}

// LogSystemHealth logs system health information
func (lm *LoggingManager) LogSystemHealth(healthData map[string]interface{}) {
	logger := lm.GetLogger("health")

	contextLogger := logger
	for k, v := range healthData {
		contextLogger = contextLogger.WithContext(k, v)
	}

	contextLogger.Info("System health check")
	lm.updateStats("health", "INFO")
}

// LogError logs an error with full context
func (lm *LoggingManager) LogError(component string, err error, message string, context map[string]interface{}) {
	logger := lm.GetLogger(component).WithError(err)

	for k, v := range context {
		logger = logger.WithContext(k, v)
	}

	logger.Error(message)
	lm.updateStats(component, "ERROR")
}

// LogCircuitBreakerStateChange logs circuit breaker state changes
func (lm *LoggingManager) LogCircuitBreakerStateChange(name string, oldState, newState errors.CircuitBreakerState) {
	logger := lm.GetLogger("circuit_breaker")
	logger.LogCircuitBreakerEvent(name, oldState, newState)
	lm.updateStats("circuit_breaker", "WARN")
}

// LogDegradationStateChange logs service degradation state changes
func (lm *LoggingManager) LogDegradationStateChange(component errors.ServiceComponent, oldLevel, newLevel errors.DegradationLevel) {
	logger := lm.GetLogger("degradation")
	logger.LogDegradationEvent(component, oldLevel, newLevel)
	lm.updateStats("degradation", "WARN")
}

// LogMCPRequest logs MCP protocol requests with timing
func (lm *LoggingManager) LogMCPRequest(method string, requestID interface{}, duration time.Duration, success bool, errorMsg string) {
	logger := lm.GetLogger("mcp_protocol")

	if !success && errorMsg != "" {
		logger = logger.WithContext("error_message", errorMsg)
	}

	logger.LogMCPMessage(method, requestID, duration, success)

	level := "INFO"
	if !success {
		level = "WARN"
	}
	lm.updateStats("mcp_protocol", level)
}

// LogCacheRefresh logs cache refresh operations
func (lm *LoggingManager) LogCacheRefresh(operation string, affectedFiles []string, duration time.Duration, success bool) {
	logger := lm.GetLogger("cache")

	contextLogger := logger.
		WithContext("cache_operation", operation).
		WithContext("affected_files_count", len(affectedFiles)).
		WithContext("duration_ms", duration.Milliseconds()).
		WithContext("success", success)

	if len(affectedFiles) > 0 && len(affectedFiles) <= 10 {
		// Log file names if not too many
		contextLogger = contextLogger.WithContext("affected_files", affectedFiles)
	}

	if success {
		contextLogger.Info("Cache refresh completed")
		lm.updateStats("cache", "INFO")
	} else {
		contextLogger.Warn("Cache refresh failed")
		lm.updateStats("cache", "WARN")
	}
}

// LogDocumentScan logs document scanning operations
func (lm *LoggingManager) LogDocumentScan(directory string, documentsFound int, errors []string, duration time.Duration) {
	logger := lm.GetLogger("scanner")

	contextLogger := logger.
		WithContext("scan_directory", directory).
		WithContext("documents_found", documentsFound).
		WithContext("error_count", len(errors)).
		WithContext("duration_ms", duration.Milliseconds())

	if len(errors) > 0 {
		// Log first few errors for context
		errorSample := errors
		if len(errors) > 5 {
			errorSample = errors[:5]
		}
		contextLogger = contextLogger.WithContext("sample_errors", errorSample)
		contextLogger.Warn("Document scan completed with errors")
		lm.updateStats("scanner", "WARN")
	} else {
		contextLogger.Info("Document scan completed successfully")
		lm.updateStats("scanner", "INFO")
	}
}

// LogFileSystemEvent logs file system monitoring events
func (lm *LoggingManager) LogFileSystemEvent(eventType string, path string, processingTime time.Duration) {
	logger := lm.GetLogger("file_monitor")

	details := map[string]interface{}{
		"processing_time_ms": processingTime.Milliseconds(),
	}

	logger.LogFileSystemEvent(eventType, path, details)
	lm.updateStats("file_monitor", "INFO")
}

// LogStartupSequence logs application startup sequence
func (lm *LoggingManager) LogStartupSequence(phase string, details map[string]interface{}, duration time.Duration, success bool) {
	logger := lm.GetLogger("startup")

	startupDetails := make(map[string]interface{})
	for k, v := range details {
		startupDetails[k] = v
	}
	startupDetails["duration_ms"] = duration.Milliseconds()
	startupDetails["success"] = success

	logger.LogStartup(phase, startupDetails)

	level := "INFO"
	if !success {
		level = "ERROR"
	}
	lm.updateStats("startup", level)
}

// LogShutdownSequence logs application shutdown sequence
func (lm *LoggingManager) LogShutdownSequence(phase string, details map[string]interface{}, duration time.Duration, success bool) {
	logger := lm.GetLogger("shutdown")

	shutdownDetails := make(map[string]interface{})
	for k, v := range details {
		shutdownDetails[k] = v
	}
	shutdownDetails["duration_ms"] = duration.Milliseconds()
	shutdownDetails["success"] = success

	logger.LogShutdown(phase, shutdownDetails)

	level := "INFO"
	if !success {
		level = "ERROR"
	}
	lm.updateStats("shutdown", level)
}

// updateStats updates logging statistics
func (lm *LoggingManager) updateStats(component, level string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.stats.TotalMessages++
	lm.stats.MessagesByLevel[level]++
	lm.stats.MessagesByLogger[component]++
	lm.stats.LastLogTime = time.Now()

	if level == "ERROR" {
		lm.stats.ErrorCount++
	}
}

// GetStats returns current logging statistics
func (lm *LoggingManager) GetStats() LoggingStats {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	// Return a copy to avoid concurrent access issues
	stats := LoggingStats{
		TotalMessages:    lm.stats.TotalMessages,
		ErrorCount:       lm.stats.ErrorCount,
		LastLogTime:      lm.stats.LastLogTime,
		MessagesByLevel:  make(map[string]int64),
		MessagesByLogger: make(map[string]int64),
	}

	for k, v := range lm.stats.MessagesByLevel {
		stats.MessagesByLevel[k] = v
	}
	for k, v := range lm.stats.MessagesByLogger {
		stats.MessagesByLogger[k] = v
	}

	return stats
}

// GetLoggerNames returns the names of all registered loggers
func (lm *LoggingManager) GetLoggerNames() []string {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	names := make([]string, 0, len(lm.loggers))
	for name := range lm.loggers {
		names = append(names, name)
	}
	return names
}

// ResetStats resets logging statistics
func (lm *LoggingManager) ResetStats() {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.stats = LoggingStats{
		MessagesByLevel:  make(map[string]int64),
		MessagesByLogger: make(map[string]int64),
	}
}
