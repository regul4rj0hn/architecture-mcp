package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mcp-architecture-service/internal/server"
)

func main() {
	// Parse command-line flags
	logLevel := flag.String("log-level", "INFO", "Logging level (DEBUG, INFO, WARN, ERROR)")
	flag.Parse()

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
			log.Printf("MCP server error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case <-sigChan:
		log.Println("Received shutdown signal, gracefully shutting down...")
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	// Perform graceful shutdown
	if err := mcpServer.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
