package server

import (
	"mcp-architecture-service/internal/models"
)

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
	s.logger.WithContext("request_id", message.ID).Info("MCP server initialized successfully")
	return nil // No response for notifications
}
