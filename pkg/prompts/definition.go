package prompts

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"mcp-architecture-service/internal/models"
)

// PromptDefinition represents the internal structure of a prompt loaded from JSON
type PromptDefinition struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Arguments   []ArgumentDefinition `json:"arguments,omitempty"`
	Messages    []MessageTemplate    `json:"messages"`
}

// ArgumentDefinition represents an argument that a prompt accepts
type ArgumentDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	MaxLength   int    `json:"maxLength,omitempty"`
}

// MessageTemplate represents a message template in the prompt
type MessageTemplate struct {
	Role    string          `json:"role"`
	Content ContentTemplate `json:"content"`
}

// ContentTemplate represents the content of a message template
type ContentTemplate struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

var (
	// promptNamePattern validates prompt names (lowercase alphanumeric and hyphens only)
	promptNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// LoadFromFile loads a prompt definition from a JSON file
func LoadFromFile(path string) (*PromptDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file: %w", err)
	}

	var def PromptDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse prompt JSON: %w", err)
	}

	return &def, nil
}

// ToMCPPrompt converts the internal definition to the MCP protocol format
func (pd *PromptDefinition) ToMCPPrompt() models.MCPPrompt {
	args := make([]models.MCPPromptArgument, len(pd.Arguments))
	for i, arg := range pd.Arguments {
		args[i] = models.MCPPromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		}
	}

	return models.MCPPrompt{
		Name:        pd.Name,
		Description: pd.Description,
		Arguments:   args,
	}
}

// Validate checks the structural integrity of the prompt definition
func (pd *PromptDefinition) Validate() error {
	// Validate prompt name
	if pd.Name == "" {
		return fmt.Errorf("prompt name is required")
	}
	if !promptNamePattern.MatchString(pd.Name) {
		return fmt.Errorf("prompt name must match pattern ^[a-z0-9-]+$, got: %s", pd.Name)
	}

	// Validate messages
	if len(pd.Messages) == 0 {
		return fmt.Errorf("prompt must have at least one message")
	}

	for i, msg := range pd.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message %d: role is required", i)
		}
		if msg.Role != "user" && msg.Role != "assistant" {
			return fmt.Errorf("message %d: role must be 'user' or 'assistant', got: %s", i, msg.Role)
		}
		if msg.Content.Type == "" {
			return fmt.Errorf("message %d: content type is required", i)
		}
		if msg.Content.Type != "text" {
			return fmt.Errorf("message %d: content type must be 'text', got: %s", i, msg.Content.Type)
		}
		if msg.Content.Text == "" {
			return fmt.Errorf("message %d: content text is required", i)
		}
	}

	// Validate arguments
	argNames := make(map[string]bool)
	for i, arg := range pd.Arguments {
		if arg.Name == "" {
			return fmt.Errorf("argument %d: name is required", i)
		}
		if argNames[arg.Name] {
			return fmt.Errorf("duplicate argument name: %s", arg.Name)
		}
		argNames[arg.Name] = true

		if arg.MaxLength < 0 {
			return fmt.Errorf("argument %s: maxLength must be non-negative", arg.Name)
		}
	}

	return nil
}

// ValidateArguments validates user-provided arguments against the definition
func (pd *PromptDefinition) ValidateArguments(args map[string]interface{}) error {
	// Check for required arguments
	for _, argDef := range pd.Arguments {
		if argDef.Required {
			if _, exists := args[argDef.Name]; !exists {
				return fmt.Errorf("required argument missing: %s", argDef.Name)
			}
		}
	}

	// Validate provided arguments
	for name, value := range args {
		// Find the argument definition
		var argDef *ArgumentDefinition
		for i := range pd.Arguments {
			if pd.Arguments[i].Name == name {
				argDef = &pd.Arguments[i]
				break
			}
		}

		// Check if argument is defined
		if argDef == nil {
			return fmt.Errorf("unknown argument: %s", name)
		}

		// Validate string length if maxLength is specified
		if argDef.MaxLength > 0 {
			strValue, ok := value.(string)
			if !ok {
				return fmt.Errorf("argument %s: expected string value", name)
			}
			if len(strValue) > argDef.MaxLength {
				return fmt.Errorf("argument %s: value exceeds maximum length of %d characters", name, argDef.MaxLength)
			}
		}
	}

	return nil
}
