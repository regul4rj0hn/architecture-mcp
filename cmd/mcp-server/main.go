package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"mcp-architecture-service/internal/server"
	"mcp-architecture-service/pkg/logging"
)

func main() {
	// Parse command-line flags
	logLevel := flag.String("log-level", "INFO", "Logging level (DEBUG, INFO, WARN, ERROR)")
	flag.Parse()

	// Initialize logging system
	loggingManager := logging.NewLoggingManager()
	loggingManager.SetGlobalContext("service", "mcp-server")
	loggingManager.SetGlobalContext("version", "1.0.0")
	loggingManager.SetLogLevel(*logLevel)
	logger := loggingManager.GetLogger("main")

	logger.Info("Starting MCP Server")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize and start MCP server with log level
	mcpServer := server.NewMCPServerWithLogLevel(*logLevel)

	// Start server in a goroutine
	go func() {
		if err := mcpServer.Start(ctx); err != nil {
			logger.WithError(err).Error("MCP server error")
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal, gracefully shutting down")
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down")
	}

	// Perform graceful shutdown
	if err := mcpServer.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Error during shutdown")
	}
}
