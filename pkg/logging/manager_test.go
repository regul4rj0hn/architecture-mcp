package logging

import (
	"testing"
	"time"

	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

func TestLoggingManager(t *testing.T) {
	t.Run("Initialization", func(t *testing.T) {
		manager := NewLoggingManager()

		if manager.loggers == nil || manager.globalContext == nil || manager.stats.MessagesByLevel == nil {
			t.Error("Expected manager to be fully initialized")
		}
		if manager.logLevel != LogLevelINFO {
			t.Errorf("Expected default log level INFO, got %v", manager.logLevel)
		}
	})

	t.Run("Logger creation and reuse", func(t *testing.T) {
		manager := NewLoggingManager()
		logger1 := manager.GetLogger("test")
		logger2 := manager.GetLogger("test")

		if logger1 != logger2 {
			t.Error("Expected same logger instance for same component")
		}
		if logger1.component != "test" {
			t.Errorf("Expected component 'test', got %s", logger1.component)
		}
	})

	t.Run("Global context management", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.SetGlobalContext("service", "test-service")
		manager.SetGlobalContext("version", "1.0.0")

		context := manager.GetGlobalContext()
		if len(context) != 2 || context["service"] != "test-service" {
			t.Error("Expected global context to be set correctly")
		}

		// Test immutability
		context["extra"] = "value"
		if len(manager.GetGlobalContext()) != 2 {
			t.Error("Expected returned context to be a copy")
		}

		// Test removal
		manager.RemoveGlobalContext("service")
		if len(manager.GetGlobalContext()) != 1 {
			t.Error("Expected context key to be removed")
		}
	})
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", LogLevelDEBUG},
		{"debug", LogLevelDEBUG},
		{"INFO", LogLevelINFO},
		{"WARN", LogLevelWARN},
		{"ERROR", LogLevelERROR},
		{"invalid", LogLevelINFO}, // defaults to INFO
		{"", LogLevelINFO},
	}

	for _, tt := range tests {
		t.Run("SetLogLevel_"+tt.input, func(t *testing.T) {
			manager := NewLoggingManager()
			manager.SetLogLevel(tt.input)
			if manager.logLevel != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, manager.logLevel)
			}
		})
	}

	t.Run("shouldLog filtering", func(t *testing.T) {
		manager := NewLoggingManager()

		// Test INFO level (default)
		manager.SetLogLevel("INFO")
		if manager.shouldLog(LogLevelDEBUG) || !manager.shouldLog(LogLevelINFO) ||
			!manager.shouldLog(LogLevelWARN) || !manager.shouldLog(LogLevelERROR) {
			t.Error("INFO level filtering incorrect")
		}

		// Test ERROR level
		manager.SetLogLevel("ERROR")
		if manager.shouldLog(LogLevelDEBUG) || manager.shouldLog(LogLevelINFO) ||
			manager.shouldLog(LogLevelWARN) || !manager.shouldLog(LogLevelERROR) {
			t.Error("ERROR level filtering incorrect")
		}

		// Test DEBUG level
		manager.SetLogLevel("DEBUG")
		if !manager.shouldLog(LogLevelDEBUG) || !manager.shouldLog(LogLevelINFO) ||
			!manager.shouldLog(LogLevelWARN) || !manager.shouldLog(LogLevelERROR) {
			t.Error("DEBUG level should allow all levels")
		}
	})
}

func TestSpecializedLogging(t *testing.T) {
	t.Run("LogApplicationEvent", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogApplicationEvent("test_event", map[string]interface{}{"key": "value"})

		stats := manager.GetStats()
		if stats.TotalMessages != 1 || stats.MessagesByLevel["INFO"] != 1 {
			t.Error("Expected application event to be logged")
		}
	})

	t.Run("LogError", func(t *testing.T) {
		manager := NewLoggingManager()
		err := errors.NewValidationError("TEST", "Test error", nil)
		manager.LogError("test", err, "Error occurred", map[string]interface{}{})

		stats := manager.GetStats()
		if stats.ErrorCount != 1 || stats.MessagesByLevel["ERROR"] != 1 {
			t.Error("Expected error to be logged and counted")
		}
	})

	t.Run("LogMCPRequest", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogMCPRequest("resources/list", "req-1", 100*time.Millisecond, true, "")
		manager.LogMCPRequest("resources/read", "req-2", 50*time.Millisecond, false, "Not found")

		stats := manager.GetStats()
		if stats.TotalMessages != 2 || stats.MessagesByLevel["INFO"] != 1 || stats.MessagesByLevel["WARN"] != 1 {
			t.Error("Expected MCP requests to be logged with correct levels")
		}
	})

	t.Run("LogCacheRefresh", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogCacheRefresh("update", []string{"file1.md"}, 100*time.Millisecond, true)

		if manager.GetStats().MessagesByLogger["cache"] != 1 {
			t.Error("Expected cache operation to be logged")
		}
	})

	t.Run("LogDocumentScan", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogDocumentScan(config.GuidelinesPath, 5, []string{}, 100*time.Millisecond)
		manager.LogDocumentScan(config.PatternsPath, 3, []string{"error1"}, 100*time.Millisecond)

		stats := manager.GetStats()
		if stats.MessagesByLogger["scanner"] != 2 || stats.MessagesByLevel["INFO"] != 1 || stats.MessagesByLevel["WARN"] != 1 {
			t.Error("Expected document scans to be logged with correct levels")
		}
	})

	t.Run("LogStartupSequence", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogStartupSequence("init", map[string]interface{}{}, 100*time.Millisecond, true)
		manager.LogStartupSequence("failed", map[string]interface{}{}, 50*time.Millisecond, false)

		stats := manager.GetStats()
		if stats.MessagesByLogger["startup"] != 2 || stats.ErrorCount != 1 {
			t.Error("Expected startup events to be logged with error count")
		}
	})

	t.Run("LogCircuitBreakerStateChange", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogCircuitBreakerStateChange("test", errors.CircuitBreakerClosed, errors.CircuitBreakerOpen)

		if manager.GetStats().MessagesByLogger["circuit_breaker"] != 1 {
			t.Error("Expected circuit breaker event to be logged")
		}
	})

	t.Run("LogDegradationStateChange", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogDegradationStateChange(errors.ComponentFileSystemMonitoring, errors.DegradationNone, errors.DegradationMinor)

		if manager.GetStats().MessagesByLogger["degradation"] != 1 {
			t.Error("Expected degradation event to be logged")
		}
	})
}

func TestLoggingStats(t *testing.T) {
	t.Run("Stats tracking", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogApplicationEvent("event1", map[string]interface{}{})
		manager.LogError("test", errors.NewSystemError("TEST", "Error", nil), "Error", map[string]interface{}{})

		stats := manager.GetStats()
		if stats.TotalMessages != 2 || stats.ErrorCount != 1 || stats.LastLogTime.IsZero() {
			t.Error("Expected stats to be tracked correctly")
		}
	})

	t.Run("Stats reset", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.LogApplicationEvent("test", map[string]interface{}{})
		manager.ResetStats()

		stats := manager.GetStats()
		if stats.TotalMessages != 0 || stats.ErrorCount != 0 || len(stats.MessagesByLevel) != 0 {
			t.Error("Expected stats to be reset")
		}
	})

	t.Run("Logger names", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.GetLogger("comp1")
		manager.GetLogger("comp2")

		names := manager.GetLoggerNames()
		if len(names) != 2 {
			t.Errorf("Expected 2 logger names, got %d", len(names))
		}
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("Concurrent logger creation", func(t *testing.T) {
		manager := NewLoggingManager()
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				logger := manager.GetLogger("concurrent")
				if logger == nil {
					t.Error("Expected logger to be created")
				}
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		if len(manager.GetLoggerNames()) != 1 {
			t.Error("Expected only one logger instance")
		}
	})

	t.Run("Concurrent stats updates", func(t *testing.T) {
		manager := NewLoggingManager()
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func() {
				manager.LogApplicationEvent("test", map[string]interface{}{})
				done <- true
			}()
		}

		for i := 0; i < 100; i++ {
			<-done
		}

		if manager.GetStats().TotalMessages != 100 {
			t.Errorf("Expected 100 messages, got %d", manager.GetStats().TotalMessages)
		}
	})
}
