package logging

import (
	"testing"
	"time"

	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

func TestLoggingManager(t *testing.T) {
	t.Run("NewLoggingManager creates manager with empty state", func(t *testing.T) {
		manager := NewLoggingManager()

		if manager.loggers == nil {
			t.Errorf("Expected loggers map to be initialized")
		}
		if manager.globalContext == nil {
			t.Errorf("Expected global context to be initialized")
		}
		if manager.stats.MessagesByLevel == nil {
			t.Errorf("Expected stats maps to be initialized")
		}
	})

	t.Run("GetLogger creates and returns logger", func(t *testing.T) {
		manager := NewLoggingManager()

		logger1 := manager.GetLogger("test-component")
		logger2 := manager.GetLogger("test-component")

		// Should return same instance for same component
		if logger1 != logger2 {
			t.Errorf("Expected same logger instance for same component")
		}

		if logger1.component != "test-component" {
			t.Errorf("Expected component 'test-component', got %s", logger1.component)
		}
	})

	t.Run("SetGlobalContext adds context to all loggers", func(t *testing.T) {
		manager := NewLoggingManager()

		// Create a logger before setting global context
		_ = manager.GetLogger("component1")

		// Set global context
		manager.SetGlobalContext("service", "test-service")
		manager.SetGlobalContext("version", "1.0.0")

		// Create another logger after setting global context
		logger2 := manager.GetLogger("component2")

		// Both loggers should have global context
		// Note: existing loggers get updated when global context is set
		updatedLogger1 := manager.GetLogger("component1")
		if updatedLogger1.context["service"] != "test-service" {
			t.Errorf("Expected existing logger to have global context, got %v", updatedLogger1.context["service"])
		}
		if logger2.context["service"] != "test-service" {
			t.Errorf("Expected new logger to have global context")
		}
	})

	t.Run("GetGlobalContext returns copy of context", func(t *testing.T) {
		manager := NewLoggingManager()

		manager.SetGlobalContext("key1", "value1")
		manager.SetGlobalContext("key2", "value2")

		context := manager.GetGlobalContext()

		if len(context) != 2 {
			t.Errorf("Expected 2 context items, got %d", len(context))
		}
		if context["key1"] != "value1" {
			t.Errorf("Expected key1 to be 'value1', got %v", context["key1"])
		}

		// Modify returned context - should not affect manager
		context["key3"] = "value3"

		originalContext := manager.GetGlobalContext()
		if len(originalContext) != 2 {
			t.Errorf("Expected original context to remain unchanged")
		}
	})

	t.Run("RemoveGlobalContext removes key", func(t *testing.T) {
		manager := NewLoggingManager()

		manager.SetGlobalContext("key1", "value1")
		manager.SetGlobalContext("key2", "value2")

		manager.RemoveGlobalContext("key1")

		context := manager.GetGlobalContext()
		if len(context) != 1 {
			t.Errorf("Expected 1 context item after removal, got %d", len(context))
		}
		if _, exists := context["key1"]; exists {
			t.Errorf("Expected key1 to be removed")
		}
		if context["key2"] != "value2" {
			t.Errorf("Expected key2 to remain")
		}
	})
}

func TestLoggingManagerSpecializedMethods(t *testing.T) {
	t.Run("LogApplicationEvent updates stats", func(t *testing.T) {
		manager := NewLoggingManager()

		details := map[string]interface{}{
			"event_type": "startup",
			"duration":   1500,
		}

		manager.LogApplicationEvent("service_started", details)

		stats := manager.GetStats()
		if stats.TotalMessages != 1 {
			t.Errorf("Expected 1 total message, got %d", stats.TotalMessages)
		}
		if stats.MessagesByLevel["INFO"] != 1 {
			t.Errorf("Expected 1 INFO message, got %d", stats.MessagesByLevel["INFO"])
		}
		if stats.MessagesByLogger["application"] != 1 {
			t.Errorf("Expected 1 message from application logger, got %d", stats.MessagesByLogger["application"])
		}
	})

	t.Run("LogError updates error count", func(t *testing.T) {
		manager := NewLoggingManager()

		testErr := errors.NewValidationError("INVALID_INPUT", "Invalid input", nil)
		context := map[string]interface{}{
			"field": "username",
		}

		manager.LogError("validator", testErr, "Validation failed", context)

		stats := manager.GetStats()
		if stats.ErrorCount != 1 {
			t.Errorf("Expected 1 error, got %d", stats.ErrorCount)
		}
		if stats.MessagesByLevel["ERROR"] != 1 {
			t.Errorf("Expected 1 ERROR message, got %d", stats.MessagesByLevel["ERROR"])
		}
	})

	t.Run("LogMCPRequest logs with correct stats", func(t *testing.T) {
		manager := NewLoggingManager()

		// Successful request
		manager.LogMCPRequest("resources/list", "req-123", 100*time.Millisecond, true, "")

		// Failed request
		manager.LogMCPRequest("resources/read", "req-124", 50*time.Millisecond, false, "Resource not found")

		stats := manager.GetStats()
		if stats.TotalMessages != 2 {
			t.Errorf("Expected 2 total messages, got %d", stats.TotalMessages)
		}
		if stats.MessagesByLevel["INFO"] != 1 {
			t.Errorf("Expected 1 INFO message, got %d", stats.MessagesByLevel["INFO"])
		}
		if stats.MessagesByLevel["WARN"] != 1 {
			t.Errorf("Expected 1 WARN message, got %d", stats.MessagesByLevel["WARN"])
		}
	})

	t.Run("LogCacheRefresh logs cache operations", func(t *testing.T) {
		manager := NewLoggingManager()

		affectedFiles := []string{"doc1.md", "doc2.md"}

		// Successful refresh
		manager.LogCacheRefresh("batch_update", affectedFiles, 200*time.Millisecond, true)

		// Failed refresh
		manager.LogCacheRefresh("single_update", []string{"doc3.md"}, 50*time.Millisecond, false)

		stats := manager.GetStats()
		if stats.MessagesByLogger["cache"] != 2 {
			t.Errorf("Expected 2 messages from cache logger, got %d", stats.MessagesByLogger["cache"])
		}
	})

	t.Run("LogDocumentScan handles errors correctly", func(t *testing.T) {
		manager := NewLoggingManager()

		// Scan with no errors
		manager.LogDocumentScan(config.GuidelinesPath, 5, []string{}, 300*time.Millisecond)

		// Scan with errors
		scanErrors := []string{"Error parsing doc1.md", "Error parsing doc2.md"}
		manager.LogDocumentScan(config.PatternsPath, 3, scanErrors, 250*time.Millisecond)

		stats := manager.GetStats()
		if stats.MessagesByLogger["scanner"] != 2 {
			t.Errorf("Expected 2 messages from scanner logger, got %d", stats.MessagesByLogger["scanner"])
		}
		if stats.MessagesByLevel["INFO"] != 1 {
			t.Errorf("Expected 1 INFO message (successful scan), got %d", stats.MessagesByLevel["INFO"])
		}
		if stats.MessagesByLevel["WARN"] != 1 {
			t.Errorf("Expected 1 WARN message (scan with errors), got %d", stats.MessagesByLevel["WARN"])
		}
	})

	t.Run("LogStartupSequence tracks startup phases", func(t *testing.T) {
		manager := NewLoggingManager()

		// Successful phase
		manager.LogStartupSequence("init", map[string]interface{}{"component": "cache"}, 100*time.Millisecond, true)

		// Failed phase
		manager.LogStartupSequence("monitor_start", map[string]interface{}{"component": "file_monitor"}, 50*time.Millisecond, false)

		stats := manager.GetStats()
		if stats.MessagesByLogger["startup"] != 2 {
			t.Errorf("Expected 2 messages from startup logger, got %d", stats.MessagesByLogger["startup"])
		}
		if stats.ErrorCount != 1 {
			t.Errorf("Expected 1 error from failed startup phase, got %d", stats.ErrorCount)
		}
	})

	t.Run("LogCircuitBreakerStateChange logs state changes", func(t *testing.T) {
		manager := NewLoggingManager()

		manager.LogCircuitBreakerStateChange("file_operations", errors.CircuitBreakerClosed, errors.CircuitBreakerOpen)

		stats := manager.GetStats()
		if stats.MessagesByLogger["circuit_breaker"] != 1 {
			t.Errorf("Expected 1 message from circuit_breaker logger, got %d", stats.MessagesByLogger["circuit_breaker"])
		}
		if stats.MessagesByLevel["WARN"] != 1 {
			t.Errorf("Expected 1 WARN message, got %d", stats.MessagesByLevel["WARN"])
		}
	})

	t.Run("LogDegradationStateChange logs degradation changes", func(t *testing.T) {
		manager := NewLoggingManager()

		manager.LogDegradationStateChange(errors.ComponentFileSystemMonitoring, errors.DegradationNone, errors.DegradationMinor)

		stats := manager.GetStats()
		if stats.MessagesByLogger["degradation"] != 1 {
			t.Errorf("Expected 1 message from degradation logger, got %d", stats.MessagesByLogger["degradation"])
		}
	})
}

func TestLoggingStats(t *testing.T) {
	t.Run("GetStats returns correct statistics", func(t *testing.T) {
		manager := NewLoggingManager()

		// Generate some log messages
		manager.LogApplicationEvent("test1", map[string]interface{}{})
		manager.LogError("test", errors.NewSystemError("TEST", "Test error", nil), "Test error", map[string]interface{}{})
		manager.LogApplicationEvent("test2", map[string]interface{}{})

		stats := manager.GetStats()

		if stats.TotalMessages != 3 {
			t.Errorf("Expected 3 total messages, got %d", stats.TotalMessages)
		}
		if stats.ErrorCount != 1 {
			t.Errorf("Expected 1 error, got %d", stats.ErrorCount)
		}
		if stats.MessagesByLevel["INFO"] != 2 {
			t.Errorf("Expected 2 INFO messages, got %d", stats.MessagesByLevel["INFO"])
		}
		if stats.MessagesByLevel["ERROR"] != 1 {
			t.Errorf("Expected 1 ERROR message, got %d", stats.MessagesByLevel["ERROR"])
		}
		if stats.LastLogTime.IsZero() {
			t.Errorf("Expected LastLogTime to be set")
		}
	})

	t.Run("ResetStats clears statistics", func(t *testing.T) {
		manager := NewLoggingManager()

		// Generate some log messages
		manager.LogApplicationEvent("test", map[string]interface{}{})
		manager.LogError("test", errors.NewSystemError("TEST", "Test error", nil), "Test error", map[string]interface{}{})

		// Verify stats are not zero
		stats := manager.GetStats()
		if stats.TotalMessages == 0 {
			t.Errorf("Expected non-zero stats before reset")
		}

		// Reset stats
		manager.ResetStats()

		// Verify stats are reset
		stats = manager.GetStats()
		if stats.TotalMessages != 0 {
			t.Errorf("Expected zero total messages after reset, got %d", stats.TotalMessages)
		}
		if stats.ErrorCount != 0 {
			t.Errorf("Expected zero error count after reset, got %d", stats.ErrorCount)
		}
		if len(stats.MessagesByLevel) != 0 {
			t.Errorf("Expected empty MessagesByLevel after reset, got %d items", len(stats.MessagesByLevel))
		}
	})

	t.Run("GetLoggerNames returns all logger names", func(t *testing.T) {
		manager := NewLoggingManager()

		// Create some loggers
		manager.GetLogger("component1")
		manager.GetLogger("component2")
		manager.GetLogger("component3")

		names := manager.GetLoggerNames()

		if len(names) != 3 {
			t.Errorf("Expected 3 logger names, got %d", len(names))
		}

		// Check that all expected names are present
		nameMap := make(map[string]bool)
		for _, name := range names {
			nameMap[name] = true
		}

		expectedNames := []string{"component1", "component2", "component3"}
		for _, expected := range expectedNames {
			if !nameMap[expected] {
				t.Errorf("Expected logger name '%s' to be present", expected)
			}
		}
	})
}

func TestLoggingManagerConcurrency(t *testing.T) {
	t.Run("Concurrent logger creation is safe", func(t *testing.T) {
		manager := NewLoggingManager()

		// Create multiple goroutines that create loggers concurrently
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				logger := manager.GetLogger("concurrent-test")
				if logger == nil {
					t.Errorf("Expected logger to be created")
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should only have one logger instance
		names := manager.GetLoggerNames()
		if len(names) != 1 {
			t.Errorf("Expected 1 logger, got %d", len(names))
		}
	})

	t.Run("Concurrent stats updates are safe", func(t *testing.T) {
		manager := NewLoggingManager()

		// Create multiple goroutines that log messages concurrently
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func(id int) {
				manager.LogApplicationEvent("concurrent-test", map[string]interface{}{"id": id})
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 100; i++ {
			<-done
		}

		stats := manager.GetStats()
		if stats.TotalMessages != 100 {
			t.Errorf("Expected 100 total messages, got %d", stats.TotalMessages)
		}
	})
}
