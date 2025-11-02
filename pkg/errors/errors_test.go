package errors

import (
	"testing"
	"time"
)

func TestStructuredError(t *testing.T) {
	t.Run("NewStructuredError creates error with correct fields", func(t *testing.T) {
		err := NewStructuredError(ErrorCategoryFileSystem, ErrorSeverityHigh, "TEST_CODE", "Test message")

		if err.Category != ErrorCategoryFileSystem {
			t.Errorf("Expected category %s, got %s", ErrorCategoryFileSystem, err.Category)
		}
		if err.Severity != ErrorSeverityHigh {
			t.Errorf("Expected severity %s, got %s", ErrorSeverityHigh, err.Severity)
		}
		if err.Code != "TEST_CODE" {
			t.Errorf("Expected code TEST_CODE, got %s", err.Code)
		}
		if err.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got %s", err.Message)
		}
		if err.Recoverable != true {
			t.Errorf("Expected recoverable to be true for non-critical error")
		}
	})

	t.Run("Critical errors are not recoverable", func(t *testing.T) {
		err := NewStructuredError(ErrorCategorySystem, ErrorSeverityCritical, "CRITICAL", "Critical error")

		if err.Recoverable != false {
			t.Errorf("Expected critical error to not be recoverable")
		}
	})

	t.Run("WithDetails adds details", func(t *testing.T) {
		err := NewStructuredError(ErrorCategoryCache, ErrorSeverityMedium, "CACHE_ERROR", "Cache error").
			WithDetails("Additional details")

		if err.Details != "Additional details" {
			t.Errorf("Expected details 'Additional details', got %s", err.Details)
		}
	})

	t.Run("WithContext adds context", func(t *testing.T) {
		err := NewStructuredError(ErrorCategoryValidation, ErrorSeverityLow, "VALIDATION", "Validation error").
			WithContext("field", "username").
			WithContext("value", "invalid")

		if err.Context["field"] != "username" {
			t.Errorf("Expected context field 'username', got %v", err.Context["field"])
		}
		if err.Context["value"] != "invalid" {
			t.Errorf("Expected context value 'invalid', got %v", err.Context["value"])
		}
	})

	t.Run("Error method returns formatted string", func(t *testing.T) {
		err := NewStructuredError(ErrorCategoryMCP, ErrorSeverityMedium, "MCP_ERROR", "MCP error")
		expected := "[mcp:MCP_ERROR] MCP error"

		if err.Error() != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("Error method includes details when present", func(t *testing.T) {
		err := NewStructuredError(ErrorCategoryMCP, ErrorSeverityMedium, "MCP_ERROR", "MCP error").
			WithDetails("Additional info")
		expected := "[mcp:MCP_ERROR] MCP error: Additional info"

		if err.Error() != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
		}
	})
}

func TestStructuredErrorToMCPError(t *testing.T) {
	t.Run("Validation error maps to invalid params", func(t *testing.T) {
		err := NewValidationError("INVALID_URI", "Invalid URI format", nil).
			WithContext("uri", "test://invalid")

		mcpErr := err.ToMCPError()

		if mcpErr.Code != -32602 {
			t.Errorf("Expected MCP code -32602, got %d", mcpErr.Code)
		}
		if mcpErr.Message != "Invalid URI format" {
			t.Errorf("Expected message 'Invalid URI format', got %s", mcpErr.Message)
		}

		// Check data contains structured error info
		data, ok := mcpErr.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected MCP error data to be map[string]interface{}")
		}
		if data["category"] != ErrorCategoryValidation {
			t.Errorf("Expected category in data to be %s", ErrorCategoryValidation)
		}
	})

	t.Run("File not found error maps to internal error", func(t *testing.T) {
		err := NewFileSystemError(ErrCodeFileNotFound, "File not found", nil)

		mcpErr := err.ToMCPError()

		if mcpErr.Code != -32603 {
			t.Errorf("Expected MCP code -32603, got %d", mcpErr.Code)
		}
	})
}

func TestPredefinedErrorConstructors(t *testing.T) {
	t.Run("NewFileSystemError creates correct error", func(t *testing.T) {
		err := NewFileSystemError(ErrCodeFileNotFound, "File not found", nil)

		if err.Category != ErrorCategoryFileSystem {
			t.Errorf("Expected category %s, got %s", ErrorCategoryFileSystem, err.Category)
		}
		if err.Code != ErrCodeFileNotFound {
			t.Errorf("Expected code %s, got %s", ErrCodeFileNotFound, err.Code)
		}
		if err.Severity != ErrorSeverityLow {
			t.Errorf("Expected severity %s for file not found, got %s", ErrorSeverityLow, err.Severity)
		}
	})

	t.Run("NewParsingError creates correct error", func(t *testing.T) {
		err := NewParsingError(ErrCodeMalformedMarkdown, "Invalid markdown", nil)

		if err.Category != ErrorCategoryParsing {
			t.Errorf("Expected category %s, got %s", ErrorCategoryParsing, err.Category)
		}
		if err.Severity != ErrorSeverityLow {
			t.Errorf("Expected severity %s for parsing error, got %s", ErrorSeverityLow, err.Severity)
		}
	})

	t.Run("NewCacheError with memory exhausted has high severity", func(t *testing.T) {
		err := NewCacheError(ErrCodeMemoryExhausted, "Out of memory", nil)

		if err.Severity != ErrorSeverityHigh {
			t.Errorf("Expected severity %s for memory exhausted, got %s", ErrorSeverityHigh, err.Severity)
		}
	})

	t.Run("NewSystemError has critical severity", func(t *testing.T) {
		err := NewSystemError(ErrCodeInitializationFailed, "Init failed", nil)

		if err.Category != ErrorCategorySystem {
			t.Errorf("Expected category %s, got %s", ErrorCategorySystem, err.Category)
		}
		if err.Severity != ErrorSeverityCritical {
			t.Errorf("Expected severity %s for system error, got %s", ErrorSeverityCritical, err.Severity)
		}
	})
}

func TestErrorUnwrapping(t *testing.T) {
	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		originalErr := NewValidationError("ORIGINAL", "Original error", nil)
		structuredErr := NewMCPError("MCP_ERROR", "Wrapped error", originalErr)

		unwrapped := structuredErr.Unwrap()
		if unwrapped != originalErr {
			t.Errorf("Expected unwrapped error to be original error")
		}
	})

	t.Run("Unwrap returns nil when no cause", func(t *testing.T) {
		structuredErr := NewValidationError("VALIDATION", "No cause", nil)

		unwrapped := structuredErr.Unwrap()
		if unwrapped != nil {
			t.Errorf("Expected unwrapped error to be nil when no cause")
		}
	})
}

func TestErrorRecoverability(t *testing.T) {
	t.Run("IsRecoverable returns correct value", func(t *testing.T) {
		recoverableErr := NewCacheError(ErrCodeCacheMiss, "Cache miss", nil)
		if !recoverableErr.IsRecoverable() {
			t.Errorf("Expected cache miss to be recoverable")
		}

		criticalErr := NewSystemError(ErrCodeUnexpectedPanic, "Panic", nil)
		if criticalErr.IsRecoverable() {
			t.Errorf("Expected system panic to not be recoverable")
		}
	})

	t.Run("SetRecoverable changes recoverability", func(t *testing.T) {
		err := NewCacheError(ErrCodeCacheMiss, "Cache miss", nil).
			SetRecoverable(false)

		if err.IsRecoverable() {
			t.Errorf("Expected error to not be recoverable after SetRecoverable(false)")
		}
	})
}

func TestErrorTimestamp(t *testing.T) {
	t.Run("Error has recent timestamp", func(t *testing.T) {
		before := time.Now()
		err := NewValidationError("TEST", "Test error", nil)
		after := time.Now()

		if err.Timestamp.Before(before) || err.Timestamp.After(after) {
			t.Errorf("Expected error timestamp to be between %v and %v, got %v",
				before, after, err.Timestamp)
		}
	})
}
