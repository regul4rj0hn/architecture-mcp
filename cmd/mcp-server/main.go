package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mcp-architecture-service/internal/server"
)

func main() {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize and start MCP server
	mcpServer := server.NewMCPServer()

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
