package models

// MCPMessage represents a JSON-RPC 2.0 message for MCP protocol
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an error in MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPServerInfo represents server information for MCP initialization
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPCapabilities represents server capabilities
type MCPCapabilities struct {
	Resources  *MCPResourceCapabilities   `json:"resources,omitempty"`
	Prompts    *MCPPromptCapabilities     `json:"prompts,omitempty"`
	Tools      *MCPToolCapabilities       `json:"tools,omitempty"`
	Completion *MCPCompletionCapabilities `json:"completion,omitempty"`
}

// MCPResourceCapabilities represents resource-related capabilities
type MCPResourceCapabilities struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPInitializeParams represents initialization parameters
type MCPInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      MCPClientInfo          `json:"clientInfo"`
}

// MCPClientInfo represents client information
type MCPClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult represents initialization result
type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ServerInfo      MCPServerInfo   `json:"serverInfo"`
}

// MCPResource represents an MCP resource
type MCPResource struct {
	URI         string            `json:"uri"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	MimeType    string            `json:"mimeType,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// MCPResourceContent represents the content of an MCP resource
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// MCPResourcesListParams represents parameters for resources/list
type MCPResourcesListParams struct {
	Cursor string `json:"cursor,omitempty"`
}

// MCPResourcesListResult represents result for resources/list
type MCPResourcesListResult struct {
	Resources  []MCPResource `json:"resources"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// MCPResourcesReadParams represents parameters for resources/read
type MCPResourcesReadParams struct {
	URI string `json:"uri"`
}

// MCPResourcesReadResult represents result for resources/read
type MCPResourcesReadResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// MCPCompletionCapabilities represents completion-related capabilities
type MCPCompletionCapabilities struct {
	ArgumentCompletions bool `json:"argumentCompletions"`
}

// MCPCompletionCompleteParams represents parameters for completion/complete
type MCPCompletionCompleteParams struct {
	Ref      MCPCompletionRef       `json:"ref"`
	Argument MCPCompletionArgument  `json:"argument"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// MCPCompletionRef represents a reference to a prompt
type MCPCompletionRef struct {
	Type string `json:"type"` // Must be "ref/prompt"
	Name string `json:"name"` // Prompt name
}

// MCPCompletionArgument represents the argument being completed
type MCPCompletionArgument struct {
	Name  string `json:"name"`  // Argument name
	Value string `json:"value"` // Current partial value
}

// MCPCompletionItem represents a single completion suggestion
type MCPCompletionItem struct {
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// MCPCompletionResult represents the result of completion/complete
type MCPCompletionResult struct {
	Completion MCPCompletion `json:"completion"`
}

// MCPCompletion contains the completion values
type MCPCompletion struct {
	Values  []MCPCompletionItem `json:"values"`
	Total   int                 `json:"total,omitempty"`
	HasMore bool                `json:"hasMore,omitempty"`
}
