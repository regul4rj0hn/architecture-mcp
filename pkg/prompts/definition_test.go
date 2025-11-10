package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPromptDefinitionValidation(t *testing.T) {
	tests := []struct {
		name    string
		def     PromptDefinition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid prompt definition",
			def: PromptDefinition{
				Name:        "test-prompt",
				Description: "A test prompt",
				Arguments: []ArgumentDefinition{
					{Name: "arg1", Required: true, MaxLength: 100},
				},
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test message with {{arg1}}",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing prompt name",
			def: PromptDefinition{
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "prompt name is required",
		},
		{
			name: "invalid prompt name with uppercase",
			def: PromptDefinition{
				Name: "Test-Prompt",
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "prompt name must match pattern",
		},
		{
			name: "invalid prompt name with special chars",
			def: PromptDefinition{
				Name: "test_prompt!",
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "prompt name must match pattern",
		},
		{
			name: "no messages",
			def: PromptDefinition{
				Name:     "test-prompt",
				Messages: []MessageTemplate{},
			},
			wantErr: true,
			errMsg:  "prompt must have at least one message",
		},
		{
			name: "message with empty role",
			def: PromptDefinition{
				Name: "test-prompt",
				Messages: []MessageTemplate{
					{
						Role: "",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "role is required",
		},
		{
			name: "message with invalid role",
			def: PromptDefinition{
				Name: "test-prompt",
				Messages: []MessageTemplate{
					{
						Role: "system",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "role must be 'user' or 'assistant'",
		},
		{
			name: "message with empty content type",
			def: PromptDefinition{
				Name: "test-prompt",
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "content type is required",
		},
		{
			name: "message with invalid content type",
			def: PromptDefinition{
				Name: "test-prompt",
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "image",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "content type must be 'text'",
		},
		{
			name: "message with empty text",
			def: PromptDefinition{
				Name: "test-prompt",
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "content text is required",
		},
		{
			name: "duplicate argument names",
			def: PromptDefinition{
				Name: "test-prompt",
				Arguments: []ArgumentDefinition{
					{Name: "arg1", Required: true},
					{Name: "arg1", Required: false},
				},
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate argument name",
		},
		{
			name: "argument with empty name",
			def: PromptDefinition{
				Name: "test-prompt",
				Arguments: []ArgumentDefinition{
					{Name: "", Required: true},
				},
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "argument with negative maxLength",
			def: PromptDefinition{
				Name: "test-prompt",
				Arguments: []ArgumentDefinition{
					{Name: "arg1", Required: true, MaxLength: -1},
				},
				Messages: []MessageTemplate{
					{
						Role: "user",
						Content: ContentTemplate{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "maxLength must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing '%s', got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateArguments(t *testing.T) {
	def := PromptDefinition{
		Name: "test-prompt",
		Arguments: []ArgumentDefinition{
			{Name: "required-arg", Required: true, MaxLength: 100},
			{Name: "optional-arg", Required: false, MaxLength: 50},
			{Name: "no-limit-arg", Required: false},
		},
		Messages: []MessageTemplate{
			{
				Role: "user",
				Content: ContentTemplate{
					Type: "text",
					Text: "Test",
				},
			},
		},
	}

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid arguments",
			args: map[string]interface{}{
				"required-arg": "test value",
				"optional-arg": "optional",
			},
			wantErr: false,
		},
		{
			name: "missing required argument",
			args: map[string]interface{}{
				"optional-arg": "optional",
			},
			wantErr: true,
			errMsg:  "required argument missing: required-arg",
		},
		{
			name: "unknown argument",
			args: map[string]interface{}{
				"required-arg": "test",
				"unknown-arg":  "value",
			},
			wantErr: true,
			errMsg:  "unknown argument: unknown-arg",
		},
		{
			name: "argument exceeds max length",
			args: map[string]interface{}{
				"required-arg": "this is a very long string that exceeds the maximum length of 100 characters and should trigger a validation error",
			},
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name: "argument at max length boundary",
			args: map[string]interface{}{
				"required-arg": "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			},
			wantErr: false,
		},
		{
			name: "non-string argument with maxLength",
			args: map[string]interface{}{
				"required-arg": 12345,
			},
			wantErr: true,
			errMsg:  "expected string value",
		},
		{
			name: "only required arguments provided",
			args: map[string]interface{}{
				"required-arg": "test",
			},
			wantErr: false,
		},
		{
			name: "argument without length limit",
			args: map[string]interface{}{
				"required-arg": "test",
				"no-limit-arg": "this can be any length without validation error",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := def.ValidateArguments(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateArguments() expected error containing '%s', got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateArguments() error = %v, want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateArguments() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(*testing.T, *PromptDefinition)
	}{
		{
			name: "valid prompt file",
			content: `{
				"name": "test-prompt",
				"description": "A test prompt",
				"arguments": [
					{
						"name": "code",
						"description": "Code to review",
						"required": true,
						"maxLength": 10000
					}
				],
				"messages": [
					{
						"role": "user",
						"content": {
							"type": "text",
							"text": "Review this code: {{code}}"
						}
					}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, def *PromptDefinition) {
				if def.Name != "test-prompt" {
					t.Errorf("Expected name 'test-prompt', got '%s'", def.Name)
				}
				if def.Description != "A test prompt" {
					t.Errorf("Expected description 'A test prompt', got '%s'", def.Description)
				}
				if len(def.Arguments) != 1 {
					t.Errorf("Expected 1 argument, got %d", len(def.Arguments))
				}
				if len(def.Messages) != 1 {
					t.Errorf("Expected 1 message, got %d", len(def.Messages))
				}
			},
		},
		{
			name:    "invalid JSON",
			content: `{"name": "test", invalid json}`,
			wantErr: true,
		},
		{
			name:    "empty file",
			content: ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpDir, tt.name+".json")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Load the file
			def, err := LoadFromFile(filePath)
			if tt.wantErr {
				if err == nil {
					t.Error("LoadFromFile() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("LoadFromFile() unexpected error = %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, def)
				}
			}
		})
	}

	// Test non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		_, err := LoadFromFile(filepath.Join(tmpDir, "does-not-exist.json"))
		if err == nil {
			t.Error("LoadFromFile() expected error for non-existent file, got nil")
		}
	})
}

func TestToMCPPrompt(t *testing.T) {
	def := PromptDefinition{
		Name:        "test-prompt",
		Description: "A test prompt",
		Arguments: []ArgumentDefinition{
			{
				Name:        "code",
				Description: "Code to review",
				Required:    true,
			},
			{
				Name:        "language",
				Description: "Programming language",
				Required:    false,
			},
		},
		Messages: []MessageTemplate{
			{
				Role: "user",
				Content: ContentTemplate{
					Type: "text",
					Text: "Test message",
				},
			},
		},
	}

	mcpPrompt := def.ToMCPPrompt()

	if mcpPrompt.Name != "test-prompt" {
		t.Errorf("Expected name 'test-prompt', got '%s'", mcpPrompt.Name)
	}

	if mcpPrompt.Description != "A test prompt" {
		t.Errorf("Expected description 'A test prompt', got '%s'", mcpPrompt.Description)
	}

	if len(mcpPrompt.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(mcpPrompt.Arguments))
	}

	if mcpPrompt.Arguments[0].Name != "code" {
		t.Errorf("Expected first argument name 'code', got '%s'", mcpPrompt.Arguments[0].Name)
	}

	if !mcpPrompt.Arguments[0].Required {
		t.Error("Expected first argument to be required")
	}

	if mcpPrompt.Arguments[1].Required {
		t.Error("Expected second argument to be optional")
	}
}

func TestPromptDefinitionJSONSerialization(t *testing.T) {
	original := PromptDefinition{
		Name:        "test-prompt",
		Description: "Test description",
		Arguments: []ArgumentDefinition{
			{
				Name:        "arg1",
				Description: "First argument",
				Required:    true,
				MaxLength:   100,
			},
		},
		Messages: []MessageTemplate{
			{
				Role: "user",
				Content: ContentTemplate{
					Type: "text",
					Text: "Test message with {{arg1}}",
				},
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal prompt definition: %v", err)
	}

	// Deserialize back
	var parsed PromptDefinition
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal prompt definition: %v", err)
	}

	// Verify fields
	if parsed.Name != original.Name {
		t.Errorf("Expected name '%s', got '%s'", original.Name, parsed.Name)
	}

	if parsed.Description != original.Description {
		t.Errorf("Expected description '%s', got '%s'", original.Description, parsed.Description)
	}

	if len(parsed.Arguments) != len(original.Arguments) {
		t.Errorf("Expected %d arguments, got %d", len(original.Arguments), len(parsed.Arguments))
	}

	if len(parsed.Messages) != len(original.Messages) {
		t.Errorf("Expected %d messages, got %d", len(original.Messages), len(parsed.Messages))
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
