package logging

import (
	"strings"
	"sync"
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
	loggers       map[string]*StructuredLogger
	globalContext map[string]any
	logLevel      LogLevel
	mutex         sync.RWMutex
}

// NewLoggingManager creates a new logging manager
func NewLoggingManager() *LoggingManager {
	return &LoggingManager{
		loggers:       make(map[string]*StructuredLogger),
		globalContext: make(map[string]any),
		logLevel:      LogLevelINFO,
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
	logger.manager = lm

	// Add global context
	for key, value := range lm.globalContext {
		logger = logger.WithContext(key, value)
	}

	lm.loggers[component] = logger
	return logger
}

// SetLogLevel sets the logging level for all loggers
func (lm *LoggingManager) SetLogLevel(level string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	switch strings.ToUpper(level) {
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

// SetGlobalContext sets global context that will be added to all log entries
func (lm *LoggingManager) SetGlobalContext(key string, value any) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.globalContext[key] = value

	// Update existing loggers
	for component, logger := range lm.loggers {
		lm.loggers[component] = logger.WithContext(key, value)
	}
}

// shouldLog checks if a message at the given level should be logged
func (lm *LoggingManager) shouldLog(level LogLevel) bool {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()
	return level >= lm.logLevel
}
