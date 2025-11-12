package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

// Helper to create test logger with buffer
func newTestLogger() (*StructuredLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &StructuredLogger{
		logger:    slog.New(handler),
		component: "test",
		context:   make(LogContext),
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
		newLogger := logger.WithContext("key1", "value1").WithContext("key2", 42)

		if len(logger.context) != 0 || len(newLogger.context) != 2 {
			t.Error("Expected WithContext to return new logger without modifying original")
		}
		if newLogger.context["key1"] != "value1" || newLogger.context["key2"] != 42 {
			t.Error("Expected context values to be set correctly")
		}
	})

	t.Run("WithError", func(t *testing.T) {
		logger := NewStructuredLogger("test")
		testErr := errors.NewValidationError("INVALID_INPUT", "Invalid input", nil).
			WithContext("field", "username")

		errorLogger := logger.WithError(testErr)

		if errorLogger.context["error_category"] != errors.ErrorCategoryValidation ||
			errorLogger.context["error_code"] != "INVALID_INPUT" ||
			errorLogger.context["error_ctx_field"] != "username" {
			t.Error("Expected error context to be added correctly")
		}

		// Nil error should return same logger
		if logger.WithError(nil) != logger {
			t.Error("Expected same logger when error is nil")
		}
	})
}

func TestLoggingOutput(t *testing.T) {
	testLogger, buf := newTestLogger()

	tests := []struct {
		name      string
		logFunc   func()
		wantLevel string
		wantMsg   string
	}{
		{"Info", func() { testLogger.Info("Test info") }, "INFO", "Test info"},
		{"Error", func() { testLogger.Error("Test error") }, "ERROR", "Test error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()

			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output: %v", err)
			}

			if logEntry["level"] != tt.wantLevel || logEntry["msg"] != tt.wantMsg {
				t.Errorf("Expected level=%s msg=%s, got level=%v msg=%v",
					tt.wantLevel, tt.wantMsg, logEntry["level"], logEntry["msg"])
			}
			if logEntry["component"] != "test" {
				t.Errorf("Expected component 'test', got %v", logEntry["component"])
			}
		})
	}

	t.Run("Context in output", func(t *testing.T) {
		buf.Reset()
		testLogger.WithContext("user_id", "12345").WithContext("action", "login").Info("User action")

		var logEntry map[string]interface{}
		json.Unmarshal(buf.Bytes(), &logEntry)

		if logEntry["user_id"] != "12345" || logEntry["action"] != "login" {
			t.Error("Expected context to be included in log output")
		}
	})
}

func TestSpecializedLoggerMethods(t *testing.T) {
	testLogger, buf := newTestLogger()

	t.Run("LogMCPMessage", func(t *testing.T) {
		buf.Reset()
		testLogger.LogMCPMessage("resources/list", "req-123", 150*time.Millisecond, true)

		var logEntry map[string]interface{}
		json.Unmarshal(buf.Bytes(), &logEntry)

		if logEntry["mcp_method"] != "resources/list" || logEntry["request_id"] != "req-123" ||
			logEntry["duration_ms"] != float64(150) || logEntry["success"] != true {
			t.Error("Expected MCP message context to be logged correctly")
		}
	})

	t.Run("LogCacheOperation", func(t *testing.T) {
		buf.Reset()
		testLogger.LogCacheOperation("get", "doc-123", true, map[string]interface{}{"cache_size": 100})

		var logEntry map[string]interface{}
		json.Unmarshal(buf.Bytes(), &logEntry)

		if logEntry["cache_operation"] != "get" || logEntry["cache_key"] != "doc-123" ||
			logEntry["cache_size"] != float64(100) {
			t.Error("Expected cache operation context to be logged correctly")
		}
	})

	t.Run("LogFileSystemEvent", func(t *testing.T) {
		buf.Reset()
		testLogger.LogFileSystemEvent("modify", config.ResourcesBasePath+"/test.md",
			map[string]interface{}{"processing_time_ms": 25})

		var logEntry map[string]interface{}
		json.Unmarshal(buf.Bytes(), &logEntry)

		if logEntry["fs_event_type"] != "modify" {
			t.Error("Expected file system event to be logged correctly")
		}
	})
}

func TestDataSanitization(t *testing.T) {
	t.Run("Sensitive keys redacted", func(t *testing.T) {
		data := map[string]interface{}{
			"username": "john",
			"password": "secret123",
			"token":    "abc123xyz",
			"email":    "john@example.com",
		}
		sanitized := sanitizeLogData(data)

		if sanitized["username"] != "john" || sanitized["email"] != "john@example.com" {
			t.Error("Expected non-sensitive data to be preserved")
		}
		if sanitized["password"] != "[REDACTED]" || sanitized["token"] != "[REDACTED]" {
			t.Error("Expected sensitive data to be redacted")
		}
	})

	t.Run("Long strings masked", func(t *testing.T) {
		result := sanitizeStringValue("abcdefghijklmnopqrstuvwxyz123456")
		if result != "[MASKED:32_chars]" {
			t.Errorf("Expected long string to be masked, got %v", result)
		}
	})

	t.Run("File paths sanitized", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"/home/user/project/mcp/resources/guidelines/api.md", config.ResourcesBasePath + "/guidelines/api.md"},
			{"/etc/passwd", "passwd"},
		}

		for _, tt := range tests {
			if got := sanitizeFilePath(tt.input); got != tt.want {
				t.Errorf("sanitizeFilePath(%s) = %s, want %s", tt.input, got, tt.want)
			}
		}
	})
}

func TestCallerInfo(t *testing.T) {
	funcName, file, line := GetCallerInfo(0)

	if funcName == "" || funcName == "unknown" {
		t.Error("Expected valid function name")
	}
	if !strings.HasSuffix(file, "logger_test.go") {
		t.Errorf("Expected file to end with 'logger_test.go', got %s", file)
	}
	if line <= 0 {
		t.Errorf("Expected positive line number, got %d", line)
	}
}

func TestLogWithCaller(t *testing.T) {
	testLogger, buf := newTestLogger()
	testLogger.LogWithCaller(LogLevelINFO, "Test message")

	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes(), &logEntry)

	if logEntry["caller_func"] == nil || logEntry["caller_file"] == nil || logEntry["caller_line"] == nil {
		t.Error("Expected caller information to be present")
	}
}

func TestLogJSON(t *testing.T) {
	testLogger, buf := newTestLogger()

	t.Run("Serializes correctly", func(t *testing.T) {
		buf.Reset()
		testLogger.LogJSON(LogLevelINFO, "JSON test", map[string]interface{}{"key1": "value1", "key2": 42})

		var logEntry map[string]interface{}
		json.Unmarshal(buf.Bytes(), &logEntry)

		var parsedData map[string]interface{}
		json.Unmarshal([]byte(logEntry["json_data"].(string)), &parsedData)

		if parsedData["key1"] != "value1" || parsedData["key2"] != float64(42) {
			t.Error("Expected JSON data to be serialized correctly")
		}
	})

	t.Run("Handles marshal error", func(t *testing.T) {
		buf.Reset()
		data := make(map[string]interface{})
		data["self"] = data // circular reference

		testLogger.LogJSON(LogLevelINFO, "JSON error", data)

		if !strings.Contains(buf.String(), "Failed to marshal JSON data") {
			t.Error("Expected error message about JSON marshaling failure")
		}
	})
}

func TestLoggerRespectsLogLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	manager := NewLoggingManager()

	tests := []struct {
		managerLevel string
		logMethod    func(*StructuredLogger)
		shouldLog    bool
	}{
		{"INFO", func(l *StructuredLogger) { l.Debug("test") }, false},
		{"INFO", func(l *StructuredLogger) { l.Info("test") }, true},
		{"WARN", func(l *StructuredLogger) { l.Info("test") }, false},
		{"WARN", func(l *StructuredLogger) { l.Warn("test") }, true},
		{"ERROR", func(l *StructuredLogger) { l.Warn("test") }, false},
		{"ERROR", func(l *StructuredLogger) { l.Error("test") }, true},
	}

	for _, tt := range tests {
		manager.SetLogLevel(tt.managerLevel)
		testLogger := &StructuredLogger{
			logger:    slog.New(handler),
			component: "test",
			context:   make(LogContext),
			manager:   manager,
		}

		buf.Reset()
		tt.logMethod(testLogger)

		if (buf.Len() > 0) != tt.shouldLog {
			t.Errorf("Level %s: expected shouldLog=%v, got %v", tt.managerLevel, tt.shouldLog, buf.Len() > 0)
		}
	}

	t.Run("Logger without manager logs all levels", func(t *testing.T) {
		testLogger := &StructuredLogger{
			logger:    slog.New(handler),
			component: "test",
			context:   make(LogContext),
			manager:   nil,
		}

		buf.Reset()
		testLogger.Debug("test")
		if buf.Len() == 0 {
			t.Error("Expected logger without manager to log all levels")
		}
	})
}
