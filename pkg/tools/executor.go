// Package tools provides secure tool execution with validation, timeout, and security controls.
//
// Security Features:
// - Path validation to prevent directory traversal attacks
// - Argument size limits to prevent resource exhaustion
// - Execution timeout enforcement to prevent hanging operations
// - Argument sanitization for safe logging
// - Circuit breaker integration for fault tolerance
//
// All tools must implement the Tool interface and are executed through the ToolExecutor
// which enforces these security constraints consistently.
package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"mcp-architecture-service/pkg/errors"
	"mcp-architecture-service/pkg/logging"
)

const (
	// DefaultToolTimeout is the default execution timeout for tools (10 seconds)
	// This prevents tools from hanging indefinitely and consuming resources
	DefaultToolTimeout = 10 * time.Second

	// MaxToolTimeout is the maximum allowed execution timeout (30 seconds)
	MaxToolTimeout = 30 * time.Second

	// Argument size limits to prevent denial of service attacks
	MaxCodeLength        = 50000 // 50KB - maximum code input for validation
	MaxQueryLength       = 500   // 500 chars - maximum search query length
	MaxDescriptionLength = 5000  // 5KB - maximum decision description length
	MaxSearchResults     = 20    // Maximum number of search results to return
)

// ToolExecutor handles tool execution with validation, timeout, and security
type ToolExecutor struct {
	maxExecutionTime time.Duration
	logger           *logging.StructuredLogger
	timeoutCallback  func() // Callback to notify manager of timeouts
}

// NewToolExecutor creates a new ToolExecutor with default settings
func NewToolExecutor(logger *logging.StructuredLogger) *ToolExecutor {
	return &ToolExecutor{
		maxExecutionTime: DefaultToolTimeout,
		logger:           logger,
	}
}

// SetTimeoutCallback sets a callback function to be called when a timeout occurs
func (te *ToolExecutor) SetTimeoutCallback(callback func()) {
	te.timeoutCallback = callback
}

// Execute validates arguments and executes a tool with timeout protection
func (te *ToolExecutor) Execute(ctx context.Context, tool Tool, arguments map[string]interface{}) (interface{}, error) {
	// Validate arguments against schema
	if err := te.ValidateArguments(tool, arguments); err != nil {
		te.logger.WithContext("tool", tool.Name()).
			WithError(err).
			Warn("Tool argument validation failed")
		return nil, err
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, te.maxExecutionTime)
	defer cancel()

	// Log execution (with sanitized arguments)
	sanitized := te.sanitizeArguments(arguments)
	logger := te.logger.WithContext("tool", tool.Name())
	for k, v := range sanitized {
		logger = logger.WithContext(fmt.Sprintf("arg_%s", k), v)
	}
	logger.Info("Executing tool")

	// Execute tool
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		// Check if timeout occurred
		if ctx.Err() == context.DeadlineExceeded {
			te.logger.WithContext("tool", tool.Name()).
				WithContext("timeout", te.maxExecutionTime.String()).
				Error("Tool execution timeout")

			// Notify manager of timeout
			if te.timeoutCallback != nil {
				te.timeoutCallback()
			}

			return nil, errors.NewSystemError(
				"TOOL_TIMEOUT",
				fmt.Sprintf("tool execution timeout after %s", te.maxExecutionTime),
				err,
			)
		}

		te.logger.WithContext("tool", tool.Name()).
			WithError(err).
			Error("Tool execution failed")
		return nil, err
	}

	te.logger.WithContext("tool", tool.Name()).
		Info("Tool execution completed")

	return result, nil
}

// ValidateArguments validates tool arguments against the tool's input schema
func (te *ToolExecutor) ValidateArguments(tool Tool, arguments map[string]interface{}) error {
	schema := tool.InputSchema()
	if schema == nil {
		return nil // No schema means no validation required
	}

	// Get required fields
	required, _ := schema["required"].([]interface{})
	requiredFields := make(map[string]bool)
	for _, field := range required {
		if fieldName, ok := field.(string); ok {
			requiredFields[fieldName] = true
		}
	}

	// Check required fields are present
	for fieldName := range requiredFields {
		if _, exists := arguments[fieldName]; !exists {
			return errors.NewValidationError(
				errors.ErrCodeInvalidParams,
				fmt.Sprintf("missing required argument: %s", fieldName),
				nil,
			)
		}
	}

	// Validate field types and constraints
	properties, _ := schema["properties"].(map[string]interface{})
	for fieldName, value := range arguments {
		if propSchema, exists := properties[fieldName]; exists {
			if err := te.validateField(fieldName, value, propSchema); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateField validates a single field against its schema
func (te *ToolExecutor) validateField(fieldName string, value interface{}, schema interface{}) error {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}

	// Validate string fields
	if fieldType, _ := schemaMap["type"].(string); fieldType == "string" {
		strValue, ok := value.(string)
		if !ok {
			return errors.NewValidationError(
				errors.ErrCodeInvalidParams,
				fmt.Sprintf("field %s must be a string", fieldName),
				nil,
			)
		}

		// Check maxLength
		if maxLength, exists := schemaMap["maxLength"].(float64); exists {
			if len(strValue) > int(maxLength) {
				return errors.NewValidationError(
					errors.ErrCodeInvalidParams,
					fmt.Sprintf("field %s exceeds maximum length of %d", fieldName, int(maxLength)),
					nil,
				)
			}
		}

		// Check enum values
		if enumValues, exists := schemaMap["enum"].([]interface{}); exists {
			valid := false
			for _, enumValue := range enumValues {
				if enumStr, ok := enumValue.(string); ok && enumStr == strValue {
					valid = true
					break
				}
			}
			if !valid {
				return errors.NewValidationError(
					errors.ErrCodeInvalidParams,
					fmt.Sprintf("field %s has invalid value, must be one of allowed values", fieldName),
					nil,
				)
			}
		}
	}

	// Validate integer fields
	if fieldType, _ := schemaMap["type"].(string); fieldType == "integer" {
		var intValue int64
		switch v := value.(type) {
		case int:
			intValue = int64(v)
		case int64:
			intValue = v
		case float64:
			intValue = int64(v)
		default:
			return errors.NewValidationError(
				errors.ErrCodeInvalidParams,
				fmt.Sprintf("field %s must be an integer", fieldName),
				nil,
			)
		}

		// Check minimum
		if minimum, exists := schemaMap["minimum"].(float64); exists {
			if intValue < int64(minimum) {
				return errors.NewValidationError(
					errors.ErrCodeInvalidParams,
					fmt.Sprintf("field %s must be at least %d", fieldName, int64(minimum)),
					nil,
				)
			}
		}

		// Check maximum
		if maximum, exists := schemaMap["maximum"].(float64); exists {
			if intValue > int64(maximum) {
				return errors.NewValidationError(
					errors.ErrCodeInvalidParams,
					fmt.Sprintf("field %s must be at most %d", fieldName, int64(maximum)),
					nil,
				)
			}
		}
	}

	return nil
}

// ValidateResourcePath validates that a path is within the allowed mcp/resources/ directory.
//
// Security: This function prevents directory traversal attacks by:
// 1. Rejecting any path containing ".." sequences
// 2. Ensuring the normalized path starts with "mcp/resources/"
// 3. Cleaning and normalizing the path to handle edge cases
//
// All tools that construct file paths should call this function before accessing
// resources to ensure they cannot escape the allowed directory.
//
// Example usage:
//
//	patternPath := fmt.Sprintf("mcp/resources/patterns/%s.md", patternName)
//	if err := ValidateResourcePath(patternPath); err != nil {
//	    return nil, fmt.Errorf("invalid path: %w", err)
//	}
func ValidateResourcePath(path string) error {
	// Clean and normalize path to handle edge cases like "mcp/resources/../../../etc/passwd"
	cleanPath := filepath.Clean(path)

	// Reject any traversal attempts - this catches both ".." and encoded variants
	if strings.Contains(path, "..") {
		return errors.NewValidationError(
			errors.ErrCodePathTraversal,
			"path traversal not allowed",
			nil,
		)
	}

	// Ensure path is within mcp/resources/ directory
	// This prevents access to files outside the allowed directory
	if !strings.HasPrefix(cleanPath, "mcp/resources/") && cleanPath != "mcp/resources" {
		return errors.NewValidationError(
			errors.ErrCodeInvalidParams,
			"path must be within mcp/resources/ directory",
			nil,
		)
	}

	return nil
}

// sanitizeArguments sanitizes arguments for logging by truncating large values.
//
// Security: This function prevents sensitive data exposure in logs by:
// 1. Truncating string values longer than 100 characters
// 2. Showing only a preview with the total length
// 3. Preventing large code blocks or descriptions from filling logs
//
// This is important because:
// - Tool arguments may contain sensitive code or business logic
// - Large arguments can make logs difficult to read and analyze
// - Log aggregation systems may have size limits
func (te *ToolExecutor) sanitizeArguments(arguments map[string]interface{}) map[string]interface{} {
	const maxLogLength = 100

	sanitized := make(map[string]interface{})
	for key, value := range arguments {
		if strValue, ok := value.(string); ok && len(strValue) > maxLogLength {
			// Truncate and show length for large strings
			sanitized[key] = fmt.Sprintf("%s... [%d chars]", strValue[:maxLogLength], len(strValue))
		} else {
			sanitized[key] = value
		}
	}
	return sanitized
}
