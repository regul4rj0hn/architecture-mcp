package errors

import (
	"fmt"
	"time"

	"mcp-architecture-service/internal/models"
)

// ErrorCategory represents different types of errors in the system
type ErrorCategory string

const (
	// File system related errors
	ErrorCategoryFileSystem ErrorCategory = "filesystem"
	// Document parsing related errors
	ErrorCategoryParsing ErrorCategory = "parsing"
	// Cache operation related errors
	ErrorCategoryCache ErrorCategory = "cache"
	// MCP protocol related errors
	ErrorCategoryMCP ErrorCategory = "mcp"
	// Validation related errors
	ErrorCategoryValidation ErrorCategory = "validation"
	// System/internal errors
	ErrorCategorySystem ErrorCategory = "system"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	ErrorSeverityLow      ErrorSeverity = "low"
	ErrorSeverityMedium   ErrorSeverity = "medium"
	ErrorSeverityHigh     ErrorSeverity = "high"
	ErrorSeverityCritical ErrorSeverity = "critical"
)

// StructuredError represents a structured error with additional context
type StructuredError struct {
	Category    ErrorCategory          `json:"category"`
	Severity    ErrorSeverity          `json:"severity"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Recoverable bool                   `json:"recoverable"`
	Cause       error                  `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (se *StructuredError) Error() string {
	if se.Details != "" {
		return fmt.Sprintf("[%s:%s] %s: %s", se.Category, se.Code, se.Message, se.Details)
	}
	return fmt.Sprintf("[%s:%s] %s", se.Category, se.Code, se.Message)
}

// Unwrap returns the underlying error for error unwrapping
func (se *StructuredError) Unwrap() error {
	return se.Cause
}

// ToMCPError converts a StructuredError to an MCP protocol error
func (se *StructuredError) ToMCPError() *models.MCPError {
	// Map error categories to MCP error codes
	var mcpCode int
	switch se.Category {
	case ErrorCategoryValidation:
		mcpCode = -32602 // Invalid params
	case ErrorCategoryMCP:
		mcpCode = -32600 // Invalid request
	case ErrorCategoryFileSystem:
		if se.Code == "FILE_NOT_FOUND" {
			mcpCode = -32603 // Internal error (resource not found)
		} else {
			mcpCode = -32603 // Internal error
		}
	case ErrorCategoryParsing:
		mcpCode = -32603 // Internal error
	case ErrorCategoryCache:
		mcpCode = -32603 // Internal error
	case ErrorCategorySystem:
		mcpCode = -32603 // Internal error
	default:
		mcpCode = -32603 // Internal error
	}

	return &models.MCPError{
		Code:    mcpCode,
		Message: se.Message,
		Data: map[string]interface{}{
			"category":  se.Category,
			"code":      se.Code,
			"severity":  se.Severity,
			"timestamp": se.Timestamp,
			"context":   se.Context,
		},
	}
}

// NewStructuredError creates a new structured error
func NewStructuredError(category ErrorCategory, severity ErrorSeverity, code, message string) *StructuredError {
	return &StructuredError{
		Category:    category,
		Severity:    severity,
		Code:        code,
		Message:     message,
		Timestamp:   time.Now(),
		Recoverable: severity != ErrorSeverityCritical,
		Context:     make(map[string]interface{}),
	}
}

// WithDetails adds details to the error
func (se *StructuredError) WithDetails(details string) *StructuredError {
	se.Details = details
	return se
}

// WithContext adds context information to the error
func (se *StructuredError) WithContext(key string, value interface{}) *StructuredError {
	if se.Context == nil {
		se.Context = make(map[string]interface{})
	}
	se.Context[key] = value
	return se
}

// WithCause sets the underlying cause error
func (se *StructuredError) WithCause(err error) *StructuredError {
	se.Cause = err
	return se
}

// IsRecoverable returns whether the error is recoverable
func (se *StructuredError) IsRecoverable() bool {
	return se.Recoverable
}

// SetRecoverable sets the recoverable flag
func (se *StructuredError) SetRecoverable(recoverable bool) *StructuredError {
	se.Recoverable = recoverable
	return se
}

// Predefined error constructors for common error scenarios

// NewFileSystemError creates a file system related error
func NewFileSystemError(code, message string, err error) *StructuredError {
	severity := ErrorSeverityMedium
	if code == "FILE_NOT_FOUND" || code == "DIRECTORY_NOT_FOUND" {
		severity = ErrorSeverityLow
	} else if code == "PERMISSION_DENIED" {
		severity = ErrorSeverityHigh
	}

	return NewStructuredError(ErrorCategoryFileSystem, severity, code, message).WithCause(err)
}

// NewParsingError creates a document parsing related error
func NewParsingError(code, message string, err error) *StructuredError {
	return NewStructuredError(ErrorCategoryParsing, ErrorSeverityLow, code, message).WithCause(err)
}

// NewCacheError creates a cache operation related error
func NewCacheError(code, message string, err error) *StructuredError {
	severity := ErrorSeverityMedium
	if code == "CACHE_MISS" {
		severity = ErrorSeverityLow
	} else if code == "MEMORY_EXHAUSTED" {
		severity = ErrorSeverityHigh
	}

	return NewStructuredError(ErrorCategoryCache, severity, code, message).WithCause(err)
}

// NewMCPError creates an MCP protocol related error
func NewMCPError(code, message string, err error) *StructuredError {
	return NewStructuredError(ErrorCategoryMCP, ErrorSeverityMedium, code, message).WithCause(err)
}

// NewValidationError creates a validation related error
func NewValidationError(code, message string, err error) *StructuredError {
	return NewStructuredError(ErrorCategoryValidation, ErrorSeverityLow, code, message).WithCause(err)
}

// NewSystemError creates a system/internal error
func NewSystemError(code, message string, err error) *StructuredError {
	return NewStructuredError(ErrorCategorySystem, ErrorSeverityCritical, code, message).WithCause(err)
}

// Common error codes
const (
	// File system error codes
	ErrCodeFileNotFound          = "FILE_NOT_FOUND"
	ErrCodeDirectoryNotFound     = "DIRECTORY_NOT_FOUND"
	ErrCodePermissionDenied      = "PERMISSION_DENIED"
	ErrCodeFileSystemUnavailable = "FILESYSTEM_UNAVAILABLE"

	// Parsing error codes
	ErrCodeMalformedMarkdown = "MALFORMED_MARKDOWN"
	ErrCodeInvalidMetadata   = "INVALID_METADATA"
	ErrCodeEncodingIssue     = "ENCODING_ISSUE"

	// Cache error codes
	ErrCodeCacheMiss        = "CACHE_MISS"
	ErrCodeMemoryExhausted  = "MEMORY_EXHAUSTED"
	ErrCodeCacheCorruption  = "CACHE_CORRUPTION"
	ErrCodeConcurrentAccess = "CONCURRENT_ACCESS"

	// MCP protocol error codes
	ErrCodeInvalidRequest   = "INVALID_REQUEST"
	ErrCodeMethodNotFound   = "METHOD_NOT_FOUND"
	ErrCodeInvalidParams    = "INVALID_PARAMS"
	ErrCodeResourceNotFound = "RESOURCE_NOT_FOUND"

	// Validation error codes
	ErrCodePathTraversal   = "PATH_TRAVERSAL"
	ErrCodeInvalidURI      = "INVALID_URI"
	ErrCodeInvalidCategory = "INVALID_CATEGORY"

	// System error codes
	ErrCodeInitializationFailed = "INITIALIZATION_FAILED"
	ErrCodeShutdownFailed       = "SHUTDOWN_FAILED"
	ErrCodeUnexpectedPanic      = "UNEXPECTED_PANIC"
)
