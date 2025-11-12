package server

import (
	"runtime"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
)

// handlePerformanceMetrics handles requests for server performance metrics
func (s *MCPServer) handlePerformanceMetrics(message *models.MCPMessage) *models.MCPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect performance metrics from various components
	cacheMetrics := s.cache.GetPerformanceMetrics()
	promptMetrics := s.promptManager.GetPerformanceMetrics()

	// Add server-level metrics
	serverMetrics := map[string]interface{}{
		"server_info":    s.serverInfo,
		"initialized":    s.initialized,
		"cache_metrics":  cacheMetrics,
		"prompt_metrics": promptMetrics,
		"goroutines":     runtime.NumGoroutine(),
		"memory_stats":   getMemoryStats(),
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	// Add tool metrics if toolManager is initialized
	if s.toolManager != nil {
		serverMetrics["tool_metrics"] = s.toolManager.GetPerformanceMetrics()
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  serverMetrics,
	}
}

// createErrorResponse creates an MCP error response
func (s *MCPServer) createErrorResponse(id interface{}, code int, message string) *models.MCPMessage {
	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &models.MCPError{
			Code:    code,
			Message: message,
		},
	}
}

// createStructuredErrorResponse creates an MCP error response from a structured error
func (s *MCPServer) createStructuredErrorResponse(id interface{}, structuredErr *errors.StructuredError) *models.MCPMessage {
	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   structuredErr.ToMCPError(),
	}
}

// getMemoryStats returns current memory statistics
func getMemoryStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"alloc_bytes":       m.Alloc,
		"total_alloc_bytes": m.TotalAlloc,
		"sys_bytes":         m.Sys,
		"num_gc":            m.NumGC,
		"gc_cpu_fraction":   m.GCCPUFraction,
	}
}
