package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"mcp-architecture-service/pkg/errors"
)

// Helper to create test logger with buffer
func newTestLogger() (*StructuredLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &StructuredLogger{
		logger:    slog.New(handler),
		component: "test",
		context:   make(map[string]any),
	}
	return logger, &buf
}

func TestStructuredLogger(t *testing.T) {
	t.Run("Initialization", func(t *testing.T) {
		logger := NewStructuredLogger("test-component")
		if logger.component != "test-component" || logger.context == nil {
			t.Error("Expected logger to be initialized correctly")
		}
	})

	t.Run("WithContext immutability", func(t *testing.T) {
		logger := NewStructuredLogger("test")
		newLogger := logger.WithContext("user_id", "value1").WithContext("count", 42)

		if len(logger.context) != 0 || len(newLogger.context) != 2 {
			t.Error("Expected WithContext to return new logger without modifying original")
		}
		if newLogger.context["user_id"] != "value1" {
			t.Errorf("Expected user_id to be 'value1', got %v", newLogger.context["user_id"])
		}
		if newLogger.context["count"] != 42 {
			t.Errorf("Expected count to be 42, got %v", newLogger.context["count"])
		}
	})

	t.Run("WithError", func(t *testing.T) {
		logger := NewStructuredLogger("test")
		testErr := errors.NewValidationError("INVALID_INPUT", "Invalid input", nil).
			WithContext("field", "username")

		newLogger := logger.WithError(testErr)
		if _, ok := newLogger.context["error"]; !ok {
			t.Error("Expected error to be added to context")
		}
		if _, ok := newLogger.context["error_category"]; !ok {
			t.Error("Expected error_category to be added for structured errors")
		}
	})

	t.Run("Log levels", func(t *testing.T) {
		logger, buf := newTestLogger()

		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")

		output := buf.String()
		if !strings.Contains(output, "debug message") {
			t.Error("Expected debug message in output")
		}
		if !strings.Contains(output, "info message") {
			t.Error("Expected info message in output")
		}
		if !strings.Contains(output, "warn message") {
			t.Error("Expected warn message in output")
		}
		if !strings.Contains(output, "error message") {
			t.Error("Expected error message in output")
		}
	})

	t.Run("Context in output", func(t *testing.T) {
		logger, buf := newTestLogger()
		logger.WithContext("user_id", 123).
			WithContext("action", "login").
			Info("User logged in")

		var logEntry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["user_id"] != float64(123) {
			t.Error("Expected user_id in log output")
		}
		if logEntry["action"] != "login" {
			t.Error("Expected action in log output")
		}
		if logEntry["component"] != "test" {
			t.Error("Expected component in log output")
		}
	})

	t.Run("Sensitive data redaction", func(t *testing.T) {
		logger, buf := newTestLogger()
		logger.WithContext("password", "secret123").
			WithContext("token", "abc123xyz").
			Info("Login attempt")

		output := buf.String()
		if strings.Contains(output, "secret123") {
			t.Error("Expected password to be redacted")
		}
		if strings.Contains(output, "abc123xyz") {
			t.Error("Expected token to be redacted")
		}
		if !strings.Contains(output, "[REDACTED]") {
			t.Error("Expected [REDACTED] in output")
		}
	})
}

func TestSanitization(t *testing.T) {
	t.Run("Sensitive keys", func(t *testing.T) {
		tests := []struct {
			key      string
			value    string
			expected string
		}{
			{"password", "secret", "[REDACTED]"},
			{"api_token", "xyz123", "[REDACTED]"},
			{"secret_key", "abc", "[REDACTED]"},
			{"username", "john", "john"},
			{"email", "test@example.com", "test@example.com"},
		}

		for _, tt := range tests {
			result := sanitizeValue(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("sanitizeValue(%q, %q) = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		}
	})

	t.Run("Long alphanumeric strings", func(t *testing.T) {
		longToken := strings.Repeat("a", 40)
		result := sanitizeValue("data", longToken)
		if !strings.Contains(result.(string), "[MASKED:") {
			t.Error("Expected long alphanumeric string to be masked")
		}
	})
}
