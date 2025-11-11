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

func TestStructuredLogger(t *testing.T) {
	t.Run("NewStructuredLogger creates logger with component", func(t *testing.T) {
		logger := NewStructuredLogger("test-component")

		if logger.component != "test-component" {
			t.Errorf("Expected component 'test-component', got %s", logger.component)
		}
		if logger.context == nil {
			t.Errorf("Expected context to be initialized")
		}
	})

	t.Run("WithContext adds context and returns new logger", func(t *testing.T) {
		logger := NewStructuredLogger("test")
		newLogger := logger.WithContext("key1", "value1").WithContext("key2", 42)

		// Original logger should not be modified
		if len(logger.context) != 0 {
			t.Errorf("Expected original logger context to be empty, got %d items", len(logger.context))
		}

		// New logger should have context
		if len(newLogger.context) != 2 {
			t.Errorf("Expected new logger to have 2 context items, got %d", len(newLogger.context))
		}
		if newLogger.context["key1"] != "value1" {
			t.Errorf("Expected key1 to be 'value1', got %v", newLogger.context["key1"])
		}
		if newLogger.context["key2"] != 42 {
			t.Errorf("Expected key2 to be 42, got %v", newLogger.context["key2"])
		}
	})

	t.Run("WithError adds error information to context", func(t *testing.T) {
		logger := NewStructuredLogger("test")
		testErr := errors.NewValidationError("INVALID_INPUT", "Invalid input provided", nil).
			WithContext("field", "username")

		errorLogger := logger.WithError(testErr)

		if errorLogger.context["error"] != "[validation:INVALID_INPUT] Invalid input provided" {
			t.Errorf("Expected error message in context, got %v", errorLogger.context["error"])
		}
		if errorLogger.context["error_category"] != errors.ErrorCategoryValidation {
			t.Errorf("Expected error category in context")
		}
		if errorLogger.context["error_code"] != "INVALID_INPUT" {
			t.Errorf("Expected error code in context")
		}
		if errorLogger.context["error_ctx_field"] != "username" {
			t.Errorf("Expected error context field in logger context")
		}
	})

	t.Run("WithError handles nil error", func(t *testing.T) {
		logger := NewStructuredLogger("test")
		errorLogger := logger.WithError(nil)

		// Should return same logger when error is nil
		if errorLogger != logger {
			t.Errorf("Expected same logger instance when error is nil")
		}
	})
}

func TestLoggingMethods(t *testing.T) {
	// Capture log output for testing
	var buf bytes.Buffer

	// Create a logger that writes to our buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := &StructuredLogger{
		logger:    slog.New(handler),
		component: "test",
		context:   make(LogContext),
	}

	t.Run("Info logs message with correct level", func(t *testing.T) {
		buf.Reset()
		testLogger.Info("Test info message")

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["level"] != "INFO" {
			t.Errorf("Expected level INFO, got %v", logEntry["level"])
		}
		if logEntry["msg"] != "Test info message" {
			t.Errorf("Expected message 'Test info message', got %v", logEntry["msg"])
		}
		if logEntry["component"] != "test" {
			t.Errorf("Expected component 'test', got %v", logEntry["component"])
		}
	})

	t.Run("Error logs message with correct level", func(t *testing.T) {
		buf.Reset()
		testLogger.Error("Test error message")

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["level"] != "ERROR" {
			t.Errorf("Expected level ERROR, got %v", logEntry["level"])
		}
	})

	t.Run("Context is included in log output", func(t *testing.T) {
		buf.Reset()
		contextLogger := testLogger.WithContext("user_id", "12345").WithContext("action", "login")
		contextLogger.Info("User action")

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["user_id"] != "12345" {
			t.Errorf("Expected user_id '12345', got %v", logEntry["user_id"])
		}
		if logEntry["action"] != "login" {
			t.Errorf("Expected action 'login', got %v", logEntry["action"])
		}
	})
}

func TestSpecializedLoggingMethods(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := &StructuredLogger{
		logger:    slog.New(handler),
		component: "test",
		context:   make(LogContext),
	}

	t.Run("LogMCPMessage logs with correct context", func(t *testing.T) {
		buf.Reset()
		testLogger.LogMCPMessage("resources/list", "req-123", 150*time.Millisecond, true)

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["mcp_method"] != "resources/list" {
			t.Errorf("Expected mcp_method 'resources/list', got %v", logEntry["mcp_method"])
		}
		if logEntry["request_id"] != "req-123" {
			t.Errorf("Expected request_id 'req-123', got %v", logEntry["request_id"])
		}
		if logEntry["duration_ms"] != float64(150) {
			t.Errorf("Expected duration_ms 150, got %v", logEntry["duration_ms"])
		}
		if logEntry["success"] != true {
			t.Errorf("Expected success true, got %v", logEntry["success"])
		}
	})

	t.Run("LogCacheOperation logs cache operations", func(t *testing.T) {
		buf.Reset()
		details := map[string]interface{}{
			"cache_size": 100,
			"hit_ratio":  0.85,
		}
		testLogger.LogCacheOperation("get", "doc-123", true, details)

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["cache_operation"] != "get" {
			t.Errorf("Expected cache_operation 'get', got %v", logEntry["cache_operation"])
		}
		if logEntry["cache_key"] != "doc-123" {
			t.Errorf("Expected cache_key 'doc-123', got %v", logEntry["cache_key"])
		}
		if logEntry["cache_size"] != float64(100) {
			t.Errorf("Expected cache_size 100, got %v", logEntry["cache_size"])
		}
	})

	t.Run("LogFileSystemEvent logs file system events", func(t *testing.T) {
		buf.Reset()
		details := map[string]interface{}{
			"processing_time_ms": 25,
		}
		testLogger.LogFileSystemEvent("modify", config.ResourcesBasePath+"/test.md", details)

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["fs_event_type"] != "modify" {
			t.Errorf("Expected fs_event_type 'modify', got %v", logEntry["fs_event_type"])
		}
		if logEntry["fs_path"] != config.ResourcesBasePath+"/test.md" {
			t.Errorf("Expected fs_path '/mcp/resources/test.md', got %v", logEntry["fs_path"])
		}
	})
}

func TestDataSanitization(t *testing.T) {
	t.Run("sanitizeLogData removes sensitive keys", func(t *testing.T) {
		data := map[string]interface{}{
			"username": "john",
			"password": "secret123",
			"token":    "abc123xyz",
			"email":    "john@example.com",
		}

		sanitized := sanitizeLogData(data)

		if sanitized["username"] != "john" {
			t.Errorf("Expected username to be preserved")
		}
		if sanitized["email"] != "john@example.com" {
			t.Errorf("Expected email to be preserved")
		}
		if sanitized["password"] != "[REDACTED]" {
			t.Errorf("Expected password to be redacted, got %v", sanitized["password"])
		}
		if sanitized["token"] != "[REDACTED]" {
			t.Errorf("Expected token to be redacted, got %v", sanitized["token"])
		}
	})

	t.Run("sanitizeStringValue masks long alphanumeric strings", func(t *testing.T) {
		longToken := "abcdefghijklmnopqrstuvwxyz123456"
		result := sanitizeStringValue(longToken)

		expected := "[MASKED:32_chars]"
		if result != expected {
			t.Errorf("Expected %s, got %v", expected, result)
		}
	})

	t.Run("sanitizeFilePath preserves mcp/resources paths", func(t *testing.T) {
		path := "/home/user/project/mcp/resources/guidelines/api.md"
		result := sanitizeFilePath(path)

		expected := config.ResourcesBasePath + "/guidelines/api.md"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("sanitizeFilePath returns filename for non-mcp paths", func(t *testing.T) {
		path := "/etc/passwd"
		result := sanitizeFilePath(path)

		expected := "passwd"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
}

func TestGetCallerInfo(t *testing.T) {
	t.Run("GetCallerInfo returns function information", func(t *testing.T) {
		funcName, file, line := GetCallerInfo(0)

		// Function name extraction may vary, just check it's not empty
		if funcName == "" || funcName == "unknown" {
			t.Errorf("Expected valid function name, got %s", funcName)
		}
		if !strings.HasSuffix(file, "logger_test.go") {
			t.Errorf("Expected file to end with 'logger_test.go', got %s", file)
		}
		if line <= 0 {
			t.Errorf("Expected positive line number, got %d", line)
		}
	})
}

func TestLogWithCaller(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := &StructuredLogger{
		logger:    slog.New(handler),
		component: "test",
		context:   make(LogContext),
	}

	t.Run("LogWithCaller includes caller information", func(t *testing.T) {
		buf.Reset()
		testLogger.LogWithCaller(LogLevelInfo, "Test message with caller")

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		if logEntry["caller_func"] == nil {
			t.Errorf("Expected caller_func to be present")
		}
		if logEntry["caller_file"] == nil {
			t.Errorf("Expected caller_file to be present")
		}
		if logEntry["caller_line"] == nil {
			t.Errorf("Expected caller_line to be present")
		}
	})
}

func TestLogJSON(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := &StructuredLogger{
		logger:    slog.New(handler),
		component: "test",
		context:   make(LogContext),
	}

	t.Run("LogJSON serializes data correctly", func(t *testing.T) {
		buf.Reset()
		data := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}
		testLogger.LogJSON(LogLevelInfo, "JSON test", data)

		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		jsonData, exists := logEntry["json_data"]
		if !exists {
			t.Errorf("Expected json_data to be present")
		}

		// Parse the JSON data
		var parsedData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData.(string)), &parsedData); err != nil {
			t.Fatalf("Failed to parse JSON data: %v", err)
		}

		if parsedData["key1"] != "value1" {
			t.Errorf("Expected key1 to be 'value1', got %v", parsedData["key1"])
		}
		if parsedData["key2"] != float64(42) {
			t.Errorf("Expected key2 to be 42, got %v", parsedData["key2"])
		}
	})

	t.Run("LogJSON handles marshal error gracefully", func(t *testing.T) {
		buf.Reset()
		// Create data that can't be marshaled (circular reference)
		data := make(map[string]interface{})
		data["self"] = data

		testLogger.LogJSON(LogLevelInfo, "JSON error test", data)

		// Should log an error about JSON marshaling failure
		logOutput := buf.String()
		if !strings.Contains(logOutput, "Failed to marshal JSON data") {
			t.Errorf("Expected error message about JSON marshaling failure")
		}
	})
}
