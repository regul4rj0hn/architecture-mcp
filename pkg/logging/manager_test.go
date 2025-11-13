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
		tests := []struct {
			name     string
			input    string
			expected LogLevel
		}{
			{"valid DEBUG", "DEBUG", LogLevelDEBUG},
			{"valid INFO", "INFO", LogLevelINFO},
			{"valid WARN", "WARN", LogLevelWARN},
			{"valid ERROR", "ERROR", LogLevelERROR},
			{"invalid defaults to INFO", "invalid", LogLevelINFO},
			{"empty defaults to INFO", "", LogLevelINFO},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				manager := NewLoggingManager()
				manager.SetLogLevel(tt.input)
				if manager.logLevel != tt.expected {
					t.Errorf("SetLogLevel(%q) = %v, want %v", tt.input, manager.logLevel, tt.expected)
				}
			})
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
		tests := []struct {
			name           string
			managerLevel   string
			testLevel      LogLevel
			shouldBeLogged bool
		}{
			{"DEBUG filtered when WARN", "WARN", LogLevelDEBUG, false},
			{"INFO filtered when WARN", "WARN", LogLevelINFO, false},
			{"WARN passes when WARN", "WARN", LogLevelWARN, true},
			{"ERROR passes when WARN", "WARN", LogLevelERROR, true},
			{"INFO filtered when ERROR", "ERROR", LogLevelINFO, false},
			{"WARN filtered when ERROR", "ERROR", LogLevelWARN, false},
			{"ERROR passes when ERROR", "ERROR", LogLevelERROR, true},
			{"DEBUG passes when DEBUG", "DEBUG", LogLevelDEBUG, true},
			{"INFO passes when DEBUG", "DEBUG", LogLevelINFO, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				manager := NewLoggingManager()
				manager.SetLogLevel(tt.managerLevel)
				result := manager.shouldLog(tt.testLevel)
				if result != tt.shouldBeLogged {
					t.Errorf("shouldLog(%v) with level %s = %v, want %v",
						tt.testLevel, tt.managerLevel, result, tt.shouldBeLogged)
				}
			})
		}
	})
}
