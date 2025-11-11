package tools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mcp-architecture-service/pkg/logging"
)

func TestValidateResourcePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid pattern path",
			path:    "mcp/resources/patterns/repository-pattern.md",
			wantErr: false,
		},
		{
			name:    "valid guideline path",
			path:    "mcp/resources/guidelines/api-design.md",
			wantErr: false,
		},
		{
			name:    "valid adr path",
			path:    "mcp/resources/adr/001-microservices.md",
			wantErr: false,
		},
		{
			name:    "valid base path",
			path:    "mcp/resources",
			wantErr: false,
		},
		{
			name:    "directory traversal with ..",
			path:    "mcp/resources/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path outside resources",
			path:    "mcp/other/file.md",
			wantErr: true,
		},
		{
			name:    "absolute path outside resources",
			path:    "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "relative path outside resources",
			path:    "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path with .. in middle",
			path:    "mcp/resources/patterns/../../../etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourcePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResourcePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolExecutor_ValidateArguments(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	executor := NewToolExecutor(logger)

	t.Run("ValidArgumentsWithRequiredFields", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "test-tool",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
					"age": map[string]interface{}{
						"type": "integer",
					},
				},
				"required": []interface{}{"name"},
			},
		}

		arguments := map[string]interface{}{
			"name": "John",
			"age":  30,
		}

		err := executor.ValidateArguments(tool, arguments)
		if err != nil {
			t.Errorf("ValidateArguments() failed: %v", err)
		}
	})

	t.Run("MissingRequiredField", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "test-tool",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"name"},
			},
		}

		arguments := map[string]interface{}{
			"age": 30,
		}

		err := executor.ValidateArguments(tool, arguments)
		if err == nil {
			t.Error("Expected error for missing required field")
		}
	})

	t.Run("InvalidFieldType", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "test-tool",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]interface{}{
						"type": "integer",
					},
				},
			},
		}

		arguments := map[string]interface{}{
			"age": "not-a-number",
		}

		err := executor.ValidateArguments(tool, arguments)
		if err == nil {
			t.Error("Expected error for invalid field type")
		}
	})

	t.Run("ExtraFieldsAllowed", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "test-tool",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
		}

		arguments := map[string]interface{}{
			"name":  "John",
			"extra": "field",
		}

		// Extra fields should be allowed (not strict validation)
		err := executor.ValidateArguments(tool, arguments)
		if err != nil {
			t.Errorf("ValidateArguments() should allow extra fields: %v", err)
		}
	})
}

func TestToolExecutor_Execute(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	executor := NewToolExecutor(logger)

	t.Run("SuccessfulExecution", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "success-tool",
			schema: map[string]interface{}{
				"type": "object",
			},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{"status": "success"}, nil
			},
		}

		ctx := context.Background()
		result, err := executor.Execute(ctx, tool, map[string]interface{}{})

		if err != nil {
			t.Errorf("Execute() failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result to be map[string]interface{}")
		}

		if resultMap["status"] != "success" {
			t.Errorf("Expected status 'success', got '%v'", resultMap["status"])
		}
	})

	t.Run("ExecutionWithTimeout", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "slow-tool",
			schema: map[string]interface{}{
				"type": "object",
			},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				select {
				case <-time.After(2 * time.Second):
					return nil, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}

		// Create context with very short timeout to test timeout handling quickly
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := executor.Execute(ctx, tool, map[string]interface{}{})

		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("ExecutionWithError", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "error-tool",
			schema: map[string]interface{}{
				"type": "object",
			},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				return nil, fmt.Errorf("execution failed")
			},
		}

		ctx := context.Background()
		_, err := executor.Execute(ctx, tool, map[string]interface{}{})

		if err == nil {
			t.Error("Expected execution error")
		}
	})

	t.Run("ExecutionWithCancelledContext", func(t *testing.T) {
		tool := &mockToolForExecutor{
			name: "cancel-tool",
			schema: map[string]interface{}{
				"type": "object",
			},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := executor.Execute(ctx, tool, map[string]interface{}{})

		if err == nil {
			t.Error("Expected context cancellation error")
		}
	})
}

func TestToolExecutor_SanitizeArguments(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	executor := NewToolExecutor(logger)

	t.Run("SanitizeLongStrings", func(t *testing.T) {
		longString := string(make([]byte, 200))
		for i := range longString {
			longString = longString[:i] + "a" + longString[i+1:]
		}

		arguments := map[string]interface{}{
			"long_field":  longString,
			"short_field": "short",
		}

		sanitized := executor.sanitizeArguments(arguments)

		// Long strings should be truncated
		longValue, ok := sanitized["long_field"].(string)
		if !ok {
			t.Fatal("Expected long_field to be string")
		}

		if len(longValue) > 120 { // 100 chars + "... [N chars]" suffix
			t.Errorf("Expected long_field to be truncated, got length %d", len(longValue))
		}

		// Short strings should remain unchanged
		if sanitized["short_field"] != "short" {
			t.Errorf("Expected short_field to be 'short', got '%v'", sanitized["short_field"])
		}
	})

	t.Run("SanitizeNestedStructures", func(t *testing.T) {
		arguments := map[string]interface{}{
			"nested": map[string]interface{}{
				"field": "value",
			},
			"array": []interface{}{1, 2, 3},
		}

		sanitized := executor.sanitizeArguments(arguments)

		// Nested structures should be preserved
		if sanitized["nested"] == nil {
			t.Error("Expected nested field to be preserved")
		}

		if sanitized["array"] == nil {
			t.Error("Expected array field to be preserved")
		}
	})
}

// mockToolForExecutor is a mock tool for executor tests
type mockToolForExecutor struct {
	name        string
	schema      map[string]interface{}
	executeFunc func(ctx context.Context, arguments map[string]interface{}) (interface{}, error)
}

func (m *mockToolForExecutor) Name() string {
	return m.name
}

func (m *mockToolForExecutor) Description() string {
	return "Mock tool for executor tests"
}

func (m *mockToolForExecutor) InputSchema() map[string]interface{} {
	return m.schema
}

func (m *mockToolForExecutor) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, arguments)
	}
	return nil, nil
}
