package models

// MCPPrompt represents a prompt available to clients
type MCPPrompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

// MCPPromptArgument represents an argument that a prompt accepts
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// MCPPromptsListResult represents the result of prompts/list
type MCPPromptsListResult struct {
	Prompts []MCPPrompt `json:"prompts"`
}

// MCPPromptsGetParams represents parameters for prompts/get
type MCPPromptsGetParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPPromptMessage represents a message in the prompt response
type MCPPromptMessage struct {
	Role    string           `json:"role"`
	Content MCPPromptContent `json:"content"`
}

// MCPPromptContent represents the content of a prompt message
type MCPPromptContent struct {
	Type string `json:"type"` // "text" or "resource"
	Text string `json:"text,omitempty"`
}

// MCPPromptsGetResult represents the result of prompts/get
type MCPPromptsGetResult struct {
	Description string             `json:"description,omitempty"`
	Messages    []MCPPromptMessage `json:"messages"`
}

// MCPPromptCapabilities represents prompt-related capabilities
type MCPPromptCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
