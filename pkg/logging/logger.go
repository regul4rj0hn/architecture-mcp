package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"mcp-architecture-service/pkg/errors"
)

// StructuredLogger provides structured logging capabilities
type StructuredLogger struct {
	logger    *slog.Logger
	component string
	context   map[string]any
	manager   *LoggingManager
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(component string) *StructuredLogger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: "timestamp", Value: slog.StringValue(time.Now().UTC().Format(time.RFC3339Nano))}
			}
			if a.Key == slog.LevelKey {
				return slog.Attr{Key: "level", Value: a.Value}
			}
			if a.Key == slog.MessageKey {
				return slog.Attr{Key: "message", Value: a.Value}
			}
			return a
		},
	}

	return &StructuredLogger{
		logger:    slog.New(slog.NewJSONHandler(os.Stderr, opts)),
		component: component,
		context:   make(map[string]any),
	}
}

// WithContext adds context to the logger (returns a new logger instance)
func (sl *StructuredLogger) WithContext(key string, value any) *StructuredLogger {
	newContext := make(map[string]any, len(sl.context)+1)
	for k, v := range sl.context {
		newContext[k] = v
	}
	newContext[key] = sanitizeValue(key, value)

	return &StructuredLogger{
		logger:    sl.logger,
		component: sl.component,
		context:   newContext,
		manager:   sl.manager,
	}
}

// WithError adds error information to the logger context
func (sl *StructuredLogger) WithError(err error) *StructuredLogger {
	if err == nil {
		return sl
	}

	newLogger := sl.WithContext("error", err.Error())

	// Add structured error details if available
	if structuredErr, ok := err.(*errors.StructuredError); ok {
		newLogger = newLogger.
			WithContext("error_category", structuredErr.Category).
			WithContext("error_code", structuredErr.Code).
			WithContext("error_severity", structuredErr.Severity).
			WithContext("error_recoverable", structuredErr.IsRecoverable())
	}

	return newLogger
}

// buildAttrs creates slog attributes from context
func (sl *StructuredLogger) buildAttrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, len(sl.context)+1)
	attrs = append(attrs, slog.String("component", sl.component))
	for key, value := range sl.context {
		attrs = append(attrs, slog.Any(key, value))
	}
	return attrs
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(message string) {
	if sl.manager != nil && !sl.manager.shouldLog(LogLevelDEBUG) {
		return
	}
	sl.logger.LogAttrs(context.Background(), slog.LevelDebug, message, sl.buildAttrs()...)
}

// Info logs an info message
func (sl *StructuredLogger) Info(message string) {
	if sl.manager != nil && !sl.manager.shouldLog(LogLevelINFO) {
		return
	}
	sl.logger.LogAttrs(context.Background(), slog.LevelInfo, message, sl.buildAttrs()...)
}

// Warn logs a warning message
func (sl *StructuredLogger) Warn(message string) {
	if sl.manager != nil && !sl.manager.shouldLog(LogLevelWARN) {
		return
	}
	sl.logger.LogAttrs(context.Background(), slog.LevelWarn, message, sl.buildAttrs()...)
}

// Error logs an error message
func (sl *StructuredLogger) Error(message string) {
	if sl.manager != nil && !sl.manager.shouldLog(LogLevelERROR) {
		return
	}
	sl.logger.LogAttrs(context.Background(), slog.LevelError, message, sl.buildAttrs()...)
}

// sanitizeValue redacts sensitive information from log values
func sanitizeValue(key string, value any) any {
	keyLower := strings.ToLower(key)

	// Redact sensitive keys
	sensitiveKeys := []string{"password", "token", "secret", "key", "auth", "credential"}
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return "[REDACTED]"
		}
	}

	// Mask long alphanumeric strings (likely tokens)
	if str, ok := value.(string); ok && len(str) > 32 && isAlphanumeric(str) {
		return fmt.Sprintf("[MASKED:%d_chars]", len(str))
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
