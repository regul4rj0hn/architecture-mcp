package models

// MCPTool represents a tool definition in MCP protocol
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolsListResult represents the result of tools/list
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPToolsCallParams represents parameters for tools/call
type MCPToolsCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPToolsCallResult represents the result of tools/call
type MCPToolsCallResult struct {
	Content []MCPToolContent `json:"content"`
}

// MCPToolContent represents tool execution result content
type MCPToolContent struct {
	Type string `json:"type"` // "text" or "resource" or "image"
	Text string `json:"text,omitempty"`
}

// MCPToolCapabilities represents tool-related capabilities
type MCPToolCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
