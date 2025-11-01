package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"

	"mcp-architecture-service/internal/models"
)

// MCPServer represents the main MCP server
type MCPServer struct {
	serverInfo   models.MCPServerInfo
	capabilities models.MCPCapabilities
	initialized  bool
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer() *MCPServer {
	return &MCPServer{
		serverInfo: models.MCPServerInfo{
			Name:    "mcp-architecture-service",
			Version: "1.0.0",
		},
		capabilities: models.MCPCapabilities{
			Resources: &models.MCPResourceCapabilities{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		initialized: false,
	}
}

// Start begins the MCP server operation
func (s *MCPServer) Start(ctx context.Context) error {
	log.Println("Starting MCP Architecture Service...")

	// Start JSON-RPC message processing loop
	return s.processMessages(ctx, os.Stdin, os.Stdout)
}

// Shutdown gracefully shuts down the MCP server
func (s *MCPServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down MCP Architecture Service...")
	return nil
}

// processMessages handles the JSON-RPC message processing loop
func (s *MCPServer) processMessages(ctx context.Context, reader io.Reader, writer io.Writer) error {
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(writer)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var message models.MCPMessage
			if err := decoder.Decode(&message); err != nil {
				if err == io.EOF {
					return nil
				}
				log.Printf("Error decoding message: %v", err)
				continue
			}

			response := s.handleMessage(&message)
			if response != nil {
				if err := encoder.Encode(response); err != nil {
					log.Printf("Error encoding response: %v", err)
				}
			}
		}
	}
}

// handleMessage processes individual MCP messages
func (s *MCPServer) handleMessage(message *models.MCPMessage) *models.MCPMessage {
	switch message.Method {
	case "initialize":
		return s.handleInitialize(message)
	case "notifications/initialized":
		return s.handleInitialized(message)
	case "resources/list":
		return s.handleResourcesList(message)
	case "resources/read":
		return s.handleResourcesRead(message)
	default:
		return s.createErrorResponse(message.ID, -32601, "Method not found")
	}
}

// handleInitialize handles the MCP initialize method
func (s *MCPServer) handleInitialize(message *models.MCPMessage) *models.MCPMessage {
	result := models.MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    s.capabilities,
		ServerInfo:      s.serverInfo,
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleInitialized handles the notifications/initialized method
func (s *MCPServer) handleInitialized(message *models.MCPMessage) *models.MCPMessage {
	s.initialized = true
	log.Println("MCP server initialized successfully")
	return nil // No response for notifications
}

// handleResourcesList handles the resources/list method
func (s *MCPServer) handleResourcesList(message *models.MCPMessage) *models.MCPMessage {
	// Return empty list for now - will be implemented in later tasks
	result := models.MCPResourcesListResult{
		Resources: []models.MCPResource{},
	}

	return &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

// handleResourcesRead handles the resources/read method
func (s *MCPServer) handleResourcesRead(message *models.MCPMessage) *models.MCPMessage {
	// Return error for now - will be implemented in later tasks
	return s.createErrorResponse(message.ID, -32602, "Resource reading not yet implemented")
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
