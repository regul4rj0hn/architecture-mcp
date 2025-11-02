package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"mcp-architecture-service/pkg/errors"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogContext represents contextual information for log entries
type LogContext map[string]interface{}

// StructuredLogger provides structured logging capabilities
type StructuredLogger struct {
	logger    *slog.Logger
	component string
	context   LogContext
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(component string) *StructuredLogger {
	// Create JSON handler for structured output
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(time.Now().UTC().Format(time.RFC3339Nano)),
				}
			}
			// Rename level key
			if a.Key == slog.LevelKey {
				return slog.Attr{
					Key:   "level",
					Value: a.Value,
				}
			}
			// Rename message key
			if a.Key == slog.MessageKey {
				return slog.Attr{
					Key:   "message",
					Value: a.Value,
				}
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &StructuredLogger{
		logger:    logger,
		component: component,
		context:   make(LogContext),
	}
}

// WithContext adds context to the logger (returns a new logger instance)
func (sl *StructuredLogger) WithContext(key string, value interface{}) *StructuredLogger {
	newLogger := &StructuredLogger{
		logger:    sl.logger,
		component: sl.component,
		context:   make(LogContext),
	}

	// Copy existing context
	for k, v := range sl.context {
		newLogger.context[k] = v
	}

	// Add new context
	newLogger.context[key] = value
	return newLogger
}

// WithError adds error information to the logger context
func (sl *StructuredLogger) WithError(err error) *StructuredLogger {
	if err == nil {
		return sl
	}

	newLogger := sl.WithContext("error", err.Error())

	// Add structured error information if available
	if structuredErr, ok := err.(*errors.StructuredError); ok {
		newLogger = newLogger.
			WithContext("error_category", structuredErr.Category).
			WithContext("error_code", structuredErr.Code).
			WithContext("error_severity", structuredErr.Severity).
			WithContext("error_recoverable", structuredErr.IsRecoverable())

		// Add error context if available
		if structuredErr.Context != nil {
			for k, v := range structuredErr.Context {
				newLogger = newLogger.WithContext(fmt.Sprintf("error_ctx_%s", k), v)
			}
		}
	}

	return newLogger
}

// buildLogAttributes creates slog attributes from context
func (sl *StructuredLogger) buildLogAttributes() []slog.Attr {
	attrs := []slog.Attr{
		slog.String("component", sl.component),
	}

	// Add context attributes
	for key, value := range sl.context {
		attrs = append(attrs, slog.Any(key, value))
	}

	return attrs
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(message string) {
	sl.logger.LogAttrs(context.Background(), slog.LevelDebug, message, sl.buildLogAttributes()...)
}

// Info logs an info message
func (sl *StructuredLogger) Info(message string) {
	sl.logger.LogAttrs(context.Background(), slog.LevelInfo, message, sl.buildLogAttributes()...)
}

// Warn logs a warning message
func (sl *StructuredLogger) Warn(message string) {
	sl.logger.LogAttrs(context.Background(), slog.LevelWarn, message, sl.buildLogAttributes()...)
}

// Error logs an error message
func (sl *StructuredLogger) Error(message string) {
	sl.logger.LogAttrs(context.Background(), slog.LevelError, message, sl.buildLogAttributes()...)
}

// LogMCPMessage logs an MCP protocol message with timing information
func (sl *StructuredLogger) LogMCPMessage(method string, requestID interface{}, duration time.Duration, success bool) {
	logger := sl.WithContext("mcp_method", method).
		WithContext("request_id", requestID).
		WithContext("duration_ms", duration.Milliseconds()).
		WithContext("success", success)

	if success {
		logger.Info("MCP message processed successfully")
	} else {
		logger.Warn("MCP message processing failed")
	}
}

// LogStartup logs application startup events
func (sl *StructuredLogger) LogStartup(event string, details map[string]interface{}) {
	logger := sl.WithContext("startup_event", event)
	for k, v := range details {
		logger = logger.WithContext(k, v)
	}
	logger.Info("Application startup event")
}

// LogShutdown logs application shutdown events
func (sl *StructuredLogger) LogShutdown(event string, details map[string]interface{}) {
	logger := sl.WithContext("shutdown_event", event)
	for k, v := range details {
		logger = logger.WithContext(k, v)
	}
	logger.Info("Application shutdown event")
}

// LogCacheOperation logs cache-related operations
func (sl *StructuredLogger) LogCacheOperation(operation string, key string, success bool, details map[string]interface{}) {
	logger := sl.WithContext("cache_operation", operation).
		WithContext("cache_key", key).
		WithContext("success", success)

	for k, v := range details {
		logger = logger.WithContext(k, v)
	}

	if success {
		logger.Debug("Cache operation completed")
	} else {
		logger.Warn("Cache operation failed")
	}
}

// LogFileSystemEvent logs file system monitoring events
func (sl *StructuredLogger) LogFileSystemEvent(eventType string, path string, details map[string]interface{}) {
	logger := sl.WithContext("fs_event_type", eventType).
		WithContext("fs_path", path)

	for k, v := range details {
		logger = logger.WithContext(k, v)
	}

	logger.Info("File system event detected")
}

// LogCircuitBreakerEvent logs circuit breaker state changes
func (sl *StructuredLogger) LogCircuitBreakerEvent(name string, oldState, newState errors.CircuitBreakerState) {
	sl.WithContext("circuit_breaker", name).
		WithContext("old_state", oldState.String()).
		WithContext("new_state", newState.String()).
		Warn("Circuit breaker state changed")
}

// LogDegradationEvent logs service degradation events
func (sl *StructuredLogger) LogDegradationEvent(component errors.ServiceComponent, oldLevel, newLevel errors.DegradationLevel) {
	sl.WithContext("degraded_component", string(component)).
		WithContext("old_level", oldLevel.String()).
		WithContext("new_level", newLevel.String()).
		Warn("Service degradation level changed")
}

// LogPerformanceMetric logs performance-related metrics
func (sl *StructuredLogger) LogPerformanceMetric(metric string, value interface{}, unit string) {
	sl.WithContext("metric_name", metric).
		WithContext("metric_value", value).
		WithContext("metric_unit", unit).
		Debug("Performance metric recorded")
}

// LogSecurityEvent logs security-related events (without sensitive data)
func (sl *StructuredLogger) LogSecurityEvent(eventType string, details map[string]interface{}) {
	logger := sl.WithContext("security_event", eventType)

	// Sanitize details to ensure no sensitive information is logged
	sanitizedDetails := sanitizeLogData(details)
	for k, v := range sanitizedDetails {
		logger = logger.WithContext(k, v)
	}

	logger.Warn("Security event detected")
}

// sanitizeLogData removes or masks sensitive information from log data
func sanitizeLogData(data map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	sensitiveKeys := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"private", "confidential", "sensitive",
	}

	for k, v := range data {
		keyLower := strings.ToLower(k)
		isSensitive := false

		// Check if key contains sensitive terms
		for _, sensitiveKey := range sensitiveKeys {
			if strings.Contains(keyLower, sensitiveKey) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			sanitized[k] = "[REDACTED]"
		} else {
			// For string values, check content for potential sensitive data
			if str, ok := v.(string); ok {
				sanitized[k] = sanitizeStringValue(str)
			} else {
				sanitized[k] = v
			}
		}
	}

	return sanitized
}

// sanitizeStringValue sanitizes string values to remove potential sensitive data
func sanitizeStringValue(value string) interface{} {
	// If string looks like a token or key (long alphanumeric), mask it
	if len(value) > 20 && isAlphanumeric(value) {
		return fmt.Sprintf("[MASKED:%d_chars]", len(value))
	}

	// If string contains file paths, sanitize them
	if strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return sanitizeFilePath(value)
	}

	return value
}

// isAlphanumeric checks if a string contains only alphanumeric characters
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// sanitizeFilePath sanitizes file paths to prevent information disclosure
func sanitizeFilePath(path string) string {
	// Only show relative paths within the docs directory
	if strings.Contains(path, "docs/") {
		parts := strings.Split(path, "docs/")
		if len(parts) > 1 {
			return "docs/" + parts[len(parts)-1]
		}
	}

	// For other paths, just show the filename
	if strings.Contains(path, "/") {
		parts := strings.Split(path, "/")
		return parts[len(parts)-1]
	}
	if strings.Contains(path, "\\") {
		parts := strings.Split(path, "\\")
		return parts[len(parts)-1]
	}

	return path
}

// GetCallerInfo returns information about the calling function
func GetCallerInfo(skip int) (string, string, int) {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown", "unknown", 0
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
		// Extract just the function name without package path
		if lastSlash := strings.LastIndex(funcName, "/"); lastSlash >= 0 {
			funcName = funcName[lastSlash+1:]
		}
		if lastDot := strings.LastIndex(funcName, "."); lastDot >= 0 {
			funcName = funcName[lastDot+1:]
		}
	} else {
		funcName = "unknown"
	}

	// Extract just the filename without full path
	if lastSlash := strings.LastIndex(file, "/"); lastSlash >= 0 {
		file = file[lastSlash+1:]
	}

	return funcName, file, line
}

// LogWithCaller logs a message with caller information
func (sl *StructuredLogger) LogWithCaller(level LogLevel, message string) {
	funcName, file, line := GetCallerInfo(1)

	logger := sl.WithContext("caller_func", funcName).
		WithContext("caller_file", file).
		WithContext("caller_line", line)

	switch level {
	case LogLevelDebug:
		logger.Debug(message)
	case LogLevelInfo:
		logger.Info(message)
	case LogLevelWarn:
		logger.Warn(message)
	case LogLevelError:
		logger.Error(message)
	}
}

// LogJSON logs a message with JSON-serializable data
func (sl *StructuredLogger) LogJSON(level LogLevel, message string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		sl.WithContext("json_error", err.Error()).Error("Failed to marshal JSON data for logging")
		return
	}

	logger := sl.WithContext("json_data", string(jsonData))

	switch level {
	case LogLevelDebug:
		logger.Debug(message)
	case LogLevelInfo:
		logger.Info(message)
	case LogLevelWarn:
		logger.Warn(message)
	case LogLevelError:
		logger.Error(message)
	}
}
