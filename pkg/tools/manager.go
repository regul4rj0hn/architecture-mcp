package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mcp-architecture-service/pkg/logging"
)

// ToolManager manages tool registration, discovery, and execution
type ToolManager struct {
	registry map[string]Tool
	executor *ToolExecutor
	logger   *logging.StructuredLogger
	mu       sync.RWMutex

	// Performance metrics
	stats ToolStats
}

// ToolStats tracks performance metrics for tool invocations
type ToolStats struct {
	TotalInvocations     int64
	FailedInvocations    int64
	InvocationsByName    map[string]int64
	TotalExecutionTimeMs int64
	ExecutionTimeByName  map[string]int64
	TimeoutCount         int64
	mu                   sync.RWMutex
}

// NewToolManager creates a new ToolManager instance
func NewToolManager(logger *logging.StructuredLogger) *ToolManager {
	return &ToolManager{
		registry: make(map[string]Tool),
		executor: NewToolExecutor(logger),
		logger:   logger,
		stats: ToolStats{
			InvocationsByName:   make(map[string]int64),
			ExecutionTimeByName: make(map[string]int64),
		},
	}
}

// RegisterTool registers a new tool in the manager
func (tm *ToolManager) RegisterTool(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("cannot register nil tool")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.registry[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	// Validate tool definition
	if tool.Description() == "" {
		tm.logger.WithContext("tool", name).
			Warn("Tool registered without description")
	}

	if tool.InputSchema() == nil {
		tm.logger.WithContext("tool", name).
			Warn("Tool registered without input schema")
	}

	tm.registry[name] = tool
	tm.logger.WithContext("tool", name).
		Info("Tool registered")

	return nil
}

// GetTool retrieves a tool by name
func (tm *ToolManager) GetTool(name string) (Tool, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tool, exists := tm.registry[name]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool, nil
}

// ListTools returns all registered tool definitions
func (tm *ToolManager) ListTools() []ToolDefinition {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tools := make([]ToolDefinition, 0, len(tm.registry))
	for _, tool := range tm.registry {
		tools = append(tools, NewToolDefinition(tool))
	}

	return tools
}

// ExecuteTool executes a tool by name with the provided arguments
func (tm *ToolManager) ExecuteTool(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error) {
	startTime := time.Now()

	// Get tool
	tool, err := tm.GetTool(name)
	if err != nil {
		tm.recordFailure(name)
		return nil, err
	}

	// Execute tool
	result, err := tm.executor.Execute(ctx, tool, arguments)

	// Record metrics
	executionTime := time.Since(startTime).Milliseconds()
	if err != nil {
		tm.recordFailure(name)
	} else {
		tm.recordSuccess(name, executionTime)
	}

	return result, err
}

// GetPerformanceMetrics returns current performance metrics
func (tm *ToolManager) GetPerformanceMetrics() map[string]interface{} {
	tm.stats.mu.RLock()
	defer tm.stats.mu.RUnlock()

	// Copy invocations by name
	invocationsByName := make(map[string]int64)
	for name, count := range tm.stats.InvocationsByName {
		invocationsByName[name] = count
	}

	// Copy execution time by name
	executionTimeByName := make(map[string]int64)
	for name, time := range tm.stats.ExecutionTimeByName {
		executionTimeByName[name] = time
	}

	return map[string]interface{}{
		"total_invocations":       tm.stats.TotalInvocations,
		"failed_invocations":      tm.stats.FailedInvocations,
		"invocations_by_name":     invocationsByName,
		"total_execution_time_ms": tm.stats.TotalExecutionTimeMs,
		"execution_time_by_name":  executionTimeByName,
		"timeout_count":           tm.stats.TimeoutCount,
	}
}

// recordSuccess records a successful tool invocation
func (tm *ToolManager) recordSuccess(toolName string, executionTimeMs int64) {
	tm.stats.mu.Lock()
	defer tm.stats.mu.Unlock()

	tm.stats.TotalInvocations++
	tm.stats.InvocationsByName[toolName]++
	tm.stats.TotalExecutionTimeMs += executionTimeMs
	tm.stats.ExecutionTimeByName[toolName] += executionTimeMs
}

// recordFailure records a failed tool invocation
func (tm *ToolManager) recordFailure(toolName string) {
	tm.stats.mu.Lock()
	defer tm.stats.mu.Unlock()

	tm.stats.TotalInvocations++
	tm.stats.FailedInvocations++
	tm.stats.InvocationsByName[toolName]++
}

// RecordTimeout records a timeout event
func (tm *ToolManager) RecordTimeout() {
	tm.stats.mu.Lock()
	defer tm.stats.mu.Unlock()

	tm.stats.TimeoutCount++
}
