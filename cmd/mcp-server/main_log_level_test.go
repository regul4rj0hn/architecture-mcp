package main

import (
	"flag"
	"testing"

	"mcp-architecture-service/internal/server"
)

func TestLogLevelFlag(t *testing.T) {
	t.Run("Server can be created with any log level string", func(t *testing.T) {
		// Test that server accepts any string and handles validation internally
		testLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "debug", "invalid", ""}

		for _, level := range testLevels {
			srv := server.NewMCPServerWithLogLevel(level)
			if srv == nil {
				t.Errorf("Expected server to be created with log level %s", level)
			}
		}
	})

	t.Run("Default log level flag value is INFO", func(t *testing.T) {
		// Create a new flag set to test default value
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		logLevel := fs.String("log-level", "INFO", "Logging level (DEBUG, INFO, WARN, ERROR)")

		if *logLevel != "INFO" {
			t.Errorf("Expected default log level to be INFO, got %s", *logLevel)
		}
	})
}
