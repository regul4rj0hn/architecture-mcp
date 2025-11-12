package logging

import (
	"testing"
)

func TestLoggingManager(t *testing.T) {
	t.Run("Initialization", func(t *testing.T) {
		manager := NewLoggingManager()
		if manager.logLevel != LogLevelINFO {
			t.Error("Expected default log level to be INFO")
		}
	})

	t.Run("GetLogger creates and caches loggers", func(t *testing.T) {
		manager := NewLoggingManager()
		logger1 := manager.GetLogger("test")
		logger2 := manager.GetLogger("test")

		if logger1 != logger2 {
			t.Error("Expected GetLogger to return cached logger")
		}
	})

	t.Run("SetLogLevel", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.SetLogLevel("DEBUG")
		if manager.logLevel != LogLevelDEBUG {
			t.Error("Expected log level to be DEBUG")
		}

		manager.SetLogLevel("invalid")
		if manager.logLevel != LogLevelINFO {
			t.Error("Expected invalid log level to default to INFO")
		}
	})

	t.Run("SetGlobalContext", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.SetGlobalContext("service", "test-service")

		logger := manager.GetLogger("test")
		if logger.context["service"] != "test-service" {
			t.Error("Expected global context to be applied to new loggers")
		}
	})

	t.Run("shouldLog respects log level", func(t *testing.T) {
		manager := NewLoggingManager()
		manager.SetLogLevel("WARN")

		if manager.shouldLog(LogLevelDEBUG) {
			t.Error("Expected DEBUG to be filtered when level is WARN")
		}
		if manager.shouldLog(LogLevelINFO) {
			t.Error("Expected INFO to be filtered when level is WARN")
		}
		if !manager.shouldLog(LogLevelWARN) {
			t.Error("Expected WARN to pass when level is WARN")
		}
		if !manager.shouldLog(LogLevelERROR) {
			t.Error("Expected ERROR to pass when level is WARN")
		}
	})
}
