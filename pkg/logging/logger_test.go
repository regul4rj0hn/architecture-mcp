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

// validateLogOutput checks if the log output contains expected messages
func validateLogOutput(t *testing.T, output string, expectedMessages []string) {
	t.Helper()
	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("Expected %q in output", msg)
		}
	}
}

func TestStructuredLoggerInitialization(t *testing.T) {
	logger := NewStructuredLogger("test-component")
	if logger.component != "test-component" || logger.context == nil {
		t.Error("Expected logger to be initialized correctly")
	}
}

func TestStructuredLoggerWithContextImmutability(t *testing.T) {
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
}

func TestStructuredLoggerWithError(t *testing.T) {
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
}

func TestStructuredLoggerLogLevels(t *testing.T) {
	logger, buf := newTestLogger()

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	validateLogOutput(t, output, []string{
		"debug message",
		"info message",
		"warn message",
		"error message",
	})
}

func TestStructuredLoggerContextInOutput(t *testing.T) {
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
}

func TestStructuredLoggerSensitiveDataRedaction(t *testing.T) {
	tests := []struct {
		name             string
		contextKey       string
		contextValue     string
		shouldBeRedacted bool
	}{
		{"password redacted", "password", "secret123", true},
		{"token redacted", "token", "abc123xyz", true},
		{"api_key redacted", "api_key", "key123", true},
		{"secret redacted", "secret", "mysecret", true},
		{"username not redacted", "username", "john", false},
		{"email not redacted", "email", "test@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newTestLogger()
			logger.WithContext(tt.contextKey, tt.contextValue).Info("Test message")

			output := buf.String()
			if tt.shouldBeRedacted {
				if strings.Contains(output, tt.contextValue) {
					t.Errorf("Expected %q to be redacted in output", tt.contextValue)
				}
				if !strings.Contains(output, "[REDACTED]") {
					t.Error("Expected [REDACTED] in output")
				}
			} else {
				if !strings.Contains(output, tt.contextValue) {
					t.Errorf("Expected %q to be present in output", tt.contextValue)
				}
			}
		})
	}
}

func TestSanitization(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		expected  string
		checkMask bool
	}{
		{"password redacted", "password", "secret", "[REDACTED]", false},
		{"api_token redacted", "api_token", "xyz123", "[REDACTED]", false},
		{"secret_key redacted", "secret_key", "abc", "[REDACTED]", false},
		{"username not redacted", "username", "john", "john", false},
		{"email not redacted", "email", "test@example.com", "test@example.com", false},
		{"long alphanumeric masked", "data", strings.Repeat("a", 40), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeValue(tt.key, tt.value)
			if tt.checkMask {
				if !strings.Contains(result.(string), "[MASKED:") {
					t.Error("Expected long alphanumeric string to be masked")
				}
			} else {
				if result != tt.expected {
					t.Errorf("sanitizeValue(%q, %q) = %v, want %v", tt.key, tt.value, result, tt.expected)
				}
			}
		})
	}
}
