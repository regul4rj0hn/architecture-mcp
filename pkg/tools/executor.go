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
	// DefaultToolTimeout is the default execution timeout for tools
	DefaultToolTimeout = 10 * time.Second

	// MaxToolTimeout is the maximum allowed execution timeout
	MaxToolTimeout = 30 * time.Second

	// Argument size limits
	MaxCodeLength        = 50000 // 50KB
	MaxQueryLength       = 500
	MaxDescriptionLength = 5000
	MaxSearchResults     = 20
)

// ToolExecutor handles tool execution with validation, timeout, and security
type ToolExecutor struct {
	maxExecutionTime time.Duration
	logger           *logging.StructuredLogger
}

// NewToolExecutor creates a new ToolExecutor with default settings
func NewToolExecutor(logger *logging.StructuredLogger) *ToolExecutor {
	return &ToolExecutor{
		maxExecutionTime: DefaultToolTimeout,
		logger:           logger,
	}
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

// ValidateResourcePath validates that a path is within the allowed mcp/resources/ directory
func ValidateResourcePath(path string) error {
	// Clean and normalize path
	cleanPath := filepath.Clean(path)

	// Reject traversal attempts
	if strings.Contains(path, "..") {
		return errors.NewValidationError(
			errors.ErrCodeInvalidParams,
			"path traversal not allowed",
			nil,
		)
	}

	// Ensure path is within mcp/resources/
	if !strings.HasPrefix(cleanPath, "mcp/resources/") && cleanPath != "mcp/resources" {
		return errors.NewValidationError(
			errors.ErrCodeInvalidParams,
			"path must be within mcp/resources/ directory",
			nil,
		)
	}

	return nil
}

// sanitizeArguments sanitizes arguments for logging by truncating large values
func (te *ToolExecutor) sanitizeArguments(arguments map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for key, value := range arguments {
		if strValue, ok := value.(string); ok && len(strValue) > 100 {
			sanitized[key] = fmt.Sprintf("%s... [%d chars]", strValue[:100], len(strValue))
		} else {
			sanitized[key] = value
		}
	}
	return sanitized
}
