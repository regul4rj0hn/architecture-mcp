package tools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/logging"
)

// mockTool is a simple mock implementation of the Tool interface for testing
type mockTool struct {
	name        string
	description string
	schema      map[string]interface{}
	executeFunc func(ctx context.Context, arguments map[string]interface{}) (interface{}, error)
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) InputSchema() map[string]interface{} {
	return m.schema
}

func (m *mockTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, arguments)
	}
	return map[string]interface{}{"result": "success"}, nil
}

func TestNewToolManager(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	if manager == nil {
		t.Fatal("NewToolManager() returned nil")
	}

	if manager.registry == nil {
		t.Error("ToolManager registry should be initialized")
	}

	if manager.executor == nil {
		t.Error("ToolManager executor should be initialized")
	}

	if manager.logger == nil {
		t.Error("ToolManager logger should be initialized")
	}
}

func TestToolManager_RegisterTool(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	t.Run("RegisterValidTool", func(t *testing.T) {
		tool := &mockTool{
			name:        "test-tool",
			description: "A test tool",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{
						"type": "string",
					},
				},
			},
		}

		err := manager.RegisterTool(tool)
		if err != nil {
			t.Errorf("RegisterTool() failed: %v", err)
		}

		// Verify tool is in registry
		retrievedTool, err := manager.GetTool("test-tool")
		if err != nil {
			t.Errorf("GetTool() failed: %v", err)
		}

		if retrievedTool.Name() != "test-tool" {
			t.Errorf("Expected tool name 'test-tool', got '%s'", retrievedTool.Name())
		}
	})

	t.Run("RegisterDuplicateTool", func(t *testing.T) {
		tool1 := &mockTool{
			name:        "duplicate-tool",
			description: "First tool",
			schema:      map[string]interface{}{"type": "object"},
		}

		tool2 := &mockTool{
			name:        "duplicate-tool",
			description: "Second tool",
			schema:      map[string]interface{}{"type": "object"},
		}

		err := manager.RegisterTool(tool1)
		if err != nil {
			t.Errorf("First RegisterTool() failed: %v", err)
		}

		err = manager.RegisterTool(tool2)
		if err == nil {
			t.Error("Expected error when registering duplicate tool")
		}
	})

	t.Run("RegisterNilTool", func(t *testing.T) {
		err := manager.RegisterTool(nil)
		if err == nil {
			t.Error("Expected error when registering nil tool")
		}
	})

	t.Run("RegisterToolWithEmptyName", func(t *testing.T) {
		tool := &mockTool{
			name:        "",
			description: "Tool with empty name",
			schema:      map[string]interface{}{"type": "object"},
		}

		err := manager.RegisterTool(tool)
		if err == nil {
			t.Error("Expected error when registering tool with empty name")
		}
	})
}

func TestToolManager_GetTool(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	tool := &mockTool{
		name:        "get-test-tool",
		description: "Tool for get test",
		schema:      map[string]interface{}{"type": "object"},
	}

	manager.RegisterTool(tool)

	t.Run("GetExistingTool", func(t *testing.T) {
		retrievedTool, err := manager.GetTool("get-test-tool")
		if err != nil {
			t.Errorf("GetTool() failed: %v", err)
		}

		if retrievedTool.Name() != "get-test-tool" {
			t.Errorf("Expected tool name 'get-test-tool', got '%s'", retrievedTool.Name())
		}
	})

	t.Run("GetNonexistentTool", func(t *testing.T) {
		_, err := manager.GetTool("nonexistent-tool")
		if err == nil {
			t.Error("Expected error when getting nonexistent tool")
		}
	})

	t.Run("GetToolWithEmptyName", func(t *testing.T) {
		_, err := manager.GetTool("")
		if err == nil {
			t.Error("Expected error when getting tool with empty name")
		}
	})
}

func TestToolManager_ListTools(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	t.Run("ListEmptyRegistry", func(t *testing.T) {
		tools := manager.ListTools()
		if len(tools) != 0 {
			t.Errorf("Expected 0 tools, got %d", len(tools))
		}
	})

	t.Run("ListMultipleTools", func(t *testing.T) {
		tool1 := &mockTool{
			name:        "tool-1",
			description: "First tool",
			schema:      map[string]interface{}{"type": "object"},
		}

		tool2 := &mockTool{
			name:        "tool-2",
			description: "Second tool",
			schema:      map[string]interface{}{"type": "object"},
		}

		tool3 := &mockTool{
			name:        "tool-3",
			description: "Third tool",
			schema:      map[string]interface{}{"type": "object"},
		}

		manager.RegisterTool(tool1)
		manager.RegisterTool(tool2)
		manager.RegisterTool(tool3)

		tools := manager.ListTools()
		if len(tools) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(tools))
		}

		// Verify all tools are present
		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name] = true

			// Verify required fields
			if tool.Name == "" {
				t.Error("Tool name should not be empty")
			}
			if tool.Description == "" {
				t.Error("Tool description should not be empty")
			}
			if tool.InputSchema == nil {
				t.Error("Tool inputSchema should not be nil")
			}
		}

		expectedNames := []string{"tool-1", "tool-2", "tool-3"}
		for _, name := range expectedNames {
			if !toolNames[name] {
				t.Errorf("Expected tool '%s' not found in list", name)
			}
		}
	})

	t.Run("ListToolsSorted", func(t *testing.T) {
		// Tools should be sorted alphabetically by name
		tools := manager.ListTools()

		for i := 1; i < len(tools); i++ {
			if tools[i-1].Name > tools[i].Name {
				t.Errorf("Tools not sorted: '%s' comes after '%s'", tools[i-1].Name, tools[i].Name)
			}
		}
	})
}

func TestToolManager_ExecuteTool(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	t.Run("ExecuteSuccessfulTool", func(t *testing.T) {
		tool := &mockTool{
			name:        "success-tool",
			description: "Tool that succeeds",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type": "string",
					},
				},
			},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{
					"result": "success",
					"input":  arguments["input"],
				}, nil
			},
		}

		manager.RegisterTool(tool)

		ctx := context.Background()
		result, err := manager.ExecuteTool(ctx, "success-tool", map[string]interface{}{
			"input": "test value",
		})

		if err != nil {
			t.Errorf("ExecuteTool() failed: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result to be map[string]interface{}")
		}

		if resultMap["result"] != "success" {
			t.Errorf("Expected result 'success', got '%v'", resultMap["result"])
		}
	})

	t.Run("ExecuteNonexistentTool", func(t *testing.T) {
		ctx := context.Background()
		_, err := manager.ExecuteTool(ctx, "nonexistent-tool", map[string]interface{}{})

		if err == nil {
			t.Error("Expected error when executing nonexistent tool")
		}
	})

	t.Run("ExecuteToolWithError", func(t *testing.T) {
		tool := &mockTool{
			name:        "error-tool",
			description: "Tool that fails",
			schema:      map[string]interface{}{"type": "object"},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				return nil, fmt.Errorf("tool execution failed")
			},
		}

		manager.RegisterTool(tool)

		ctx := context.Background()
		_, err := manager.ExecuteTool(ctx, "error-tool", map[string]interface{}{})

		if err == nil {
			t.Error("Expected error from tool execution")
		}
	})

	t.Run("ExecuteToolWithTimeout", func(t *testing.T) {
		tool := &mockTool{
			name:        "slow-tool",
			description: "Tool that takes too long",
			schema:      map[string]interface{}{"type": "object"},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				// Simulate slow operation
				select {
				case <-time.After(2 * time.Second):
					return map[string]interface{}{"result": "completed"}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}

		manager.RegisterTool(tool)

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := manager.ExecuteTool(ctx, "slow-tool", map[string]interface{}{})

		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

func TestToolManager_PerformanceMetrics(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	tool := &mockTool{
		name:        "metrics-tool",
		description: "Tool for metrics testing",
		schema:      map[string]interface{}{"type": "object"},
		executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		},
	}

	manager.RegisterTool(tool)

	t.Run("TrackSuccessfulInvocations", func(t *testing.T) {
		ctx := context.Background()

		// Execute tool multiple times
		for i := 0; i < 5; i++ {
			_, err := manager.ExecuteTool(ctx, "metrics-tool", map[string]interface{}{})
			if err != nil {
				t.Errorf("ExecuteTool() failed: %v", err)
			}
		}

		metrics := manager.GetPerformanceMetrics()

		totalInvocations, ok := metrics["total_invocations"].(int64)
		if !ok || totalInvocations < 5 {
			t.Errorf("Expected at least 5 total invocations, got %v", metrics["total_invocations"])
		}
	})

	t.Run("TrackFailedInvocations", func(t *testing.T) {
		errorTool := &mockTool{
			name:        "error-metrics-tool",
			description: "Tool that fails for metrics",
			schema:      map[string]interface{}{"type": "object"},
			executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
				return nil, fmt.Errorf("intentional error")
			},
		}

		manager.RegisterTool(errorTool)

		ctx := context.Background()

		// Execute tool that fails
		for i := 0; i < 3; i++ {
			manager.ExecuteTool(ctx, "error-metrics-tool", map[string]interface{}{})
		}

		metrics := manager.GetPerformanceMetrics()

		failedInvocations, ok := metrics["failed_invocations"].(int64)
		if !ok || failedInvocations < 3 {
			t.Errorf("Expected at least 3 failed invocations, got %v", metrics["failed_invocations"])
		}
	})

	t.Run("TrackInvocationsByName", func(t *testing.T) {
		metrics := manager.GetPerformanceMetrics()

		invocationsByName, ok := metrics["invocations_by_name"].(map[string]int64)
		if !ok {
			t.Fatal("Expected invocations_by_name to be map[string]int64")
		}

		if invocationsByName["metrics-tool"] < 5 {
			t.Errorf("Expected at least 5 invocations for metrics-tool, got %d", invocationsByName["metrics-tool"])
		}
	})
}

func TestToolManager_ConcurrentAccess(t *testing.T) {
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)

	tool := &mockTool{
		name:        "concurrent-tool",
		description: "Tool for concurrency testing",
		schema:      map[string]interface{}{"type": "object"},
		executeFunc: func(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			return map[string]interface{}{"result": "success"}, nil
		},
	}

	manager.RegisterTool(tool)

	t.Run("ConcurrentExecutions", func(t *testing.T) {
		const numGoroutines = 20
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		ctx := context.Background()

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				_, err := manager.ExecuteTool(ctx, "concurrent-tool", map[string]interface{}{
					"index": index,
				})

				if err != nil {
					errors <- err
				}

				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// Success
			case err := <-errors:
				t.Errorf("Concurrent execution failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Concurrent executions timed out")
			}
		}
	})

	t.Run("ConcurrentListAndExecute", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool, numGoroutines*2)

		ctx := context.Background()

		// Half list, half execute
		for i := 0; i < numGoroutines; i++ {
			go func() {
				manager.ListTools()
				done <- true
			}()

			go func() {
				manager.ExecuteTool(ctx, "concurrent-tool", map[string]interface{}{})
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines*2; i++ {
			select {
			case <-done:
				// Success
			case <-time.After(5 * time.Second):
				t.Fatal("Concurrent operations timed out")
			}
		}
	})
}

func TestToolManager_WithRealTools(t *testing.T) {
	// Test with actual tool implementations
	logger := logging.NewStructuredLogger("test")
	manager := NewToolManager(logger)
	testCache := cache.NewDocumentCache()

	t.Run("RegisterRealTools", func(t *testing.T) {
		validateTool := NewValidatePatternTool(testCache, logger)
		searchTool := NewSearchArchitectureTool(testCache, logger)
		adrTool := NewCheckADRAlignmentTool(testCache, logger)

		err := manager.RegisterTool(validateTool)
		if err != nil {
			t.Errorf("Failed to register ValidatePatternTool: %v", err)
		}

		err = manager.RegisterTool(searchTool)
		if err != nil {
			t.Errorf("Failed to register SearchArchitectureTool: %v", err)
		}

		err = manager.RegisterTool(adrTool)
		if err != nil {
			t.Errorf("Failed to register CheckADRAlignmentTool: %v", err)
		}

		tools := manager.ListTools()
		if len(tools) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(tools))
		}
	})

	t.Run("ExecuteRealTools", func(t *testing.T) {
		ctx := context.Background()

		// Test search tool
		_, err := manager.ExecuteTool(ctx, "search-architecture", map[string]interface{}{
			"query": "test",
		})
		if err != nil {
			t.Errorf("Failed to execute search-architecture: %v", err)
		}

		// Test validate tool
		_, err = manager.ExecuteTool(ctx, "validate-against-pattern", map[string]interface{}{
			"code":         "test code",
			"pattern_name": "test-pattern",
		})
		// This may fail due to missing pattern, but should not panic
		if err == nil {
			// Success case
		}

		// Test ADR alignment tool
		_, err = manager.ExecuteTool(ctx, "check-adr-alignment", map[string]interface{}{
			"decision_description": "test decision",
		})
		if err != nil {
			t.Errorf("Failed to execute check-adr-alignment: %v", err)
		}
	})
}
