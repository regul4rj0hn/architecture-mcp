package prompts

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/logging"
	"mcp-architecture-service/pkg/monitor"
)

func TestNewPromptManager(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	if pm == nil {
		t.Fatal("NewPromptManager() returned nil")
	}

	if pm.promptsDir != "prompts" {
		t.Errorf("Expected promptsDir 'prompts', got '%s'", pm.promptsDir)
	}

	if pm.registry == nil {
		t.Error("Expected registry to be initialized")
	}

	if pm.cache == nil {
		t.Error("Expected cache to be set")
	}

	if pm.renderer == nil {
		t.Error("Expected renderer to be initialized")
	}
}

func TestLoadPrompts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test prompt files
	validPrompt := `{
		"name": "test-prompt",
		"description": "A test prompt",
		"arguments": [
			{
				"name": "code",
				"required": true,
				"maxLength": 1000
			}
		],
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Review: {{code}}"
				}
			}
		]
	}`

	anotherPrompt := `{
		"name": "another-prompt",
		"description": "Another test prompt",
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Hello"
				}
			}
		]
	}`

	invalidPrompt := `{
		"name": "invalid",
		"messages": []
	}`

	// Write test files
	if err := os.WriteFile(filepath.Join(tmpDir, "test-prompt.json"), []byte(validPrompt), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "another-prompt.json"), []byte(anotherPrompt), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "invalid-prompt.json"), []byte(invalidPrompt), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create non-JSON file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("not a prompt"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager(tmpDir, cache, monitor, logger)

	// Load prompts
	err = pm.LoadPrompts()
	if err != nil {
		t.Fatalf("LoadPrompts() unexpected error: %v", err)
	}

	// Verify valid prompts were loaded (invalid one should be skipped)
	if len(pm.registry) != 2 {
		t.Errorf("Expected 2 prompts in registry, got %d", len(pm.registry))
	}

	// Verify specific prompts
	if _, exists := pm.registry["test-prompt"]; !exists {
		t.Error("Expected 'test-prompt' to be in registry")
	}

	if _, exists := pm.registry["another-prompt"]; !exists {
		t.Error("Expected 'another-prompt' to be in registry")
	}

	if _, exists := pm.registry["invalid"]; exists {
		t.Error("Expected 'invalid' prompt to be skipped")
	}
}

func TestLoadPromptsNonExistentDirectory(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("/non/existent/directory", cache, monitor, logger)

	// Should not error, just log warning
	err = pm.LoadPrompts()
	if err != nil {
		t.Errorf("LoadPrompts() unexpected error for non-existent directory: %v", err)
	}

	if len(pm.registry) != 0 {
		t.Errorf("Expected empty registry, got %d prompts", len(pm.registry))
	}
}

func TestGetPrompt(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	// Add test prompt to registry
	testPrompt := &PromptDefinition{
		Name:        "test-prompt",
		Description: "Test",
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

	pm.registry["test-prompt"] = testPrompt

	// Test getting existing prompt
	prompt, err := pm.GetPrompt("test-prompt")
	if err != nil {
		t.Errorf("GetPrompt() unexpected error: %v", err)
	}

	if prompt == nil {
		t.Fatal("GetPrompt() returned nil prompt")
	}

	if prompt.Name != "test-prompt" {
		t.Errorf("Expected prompt name 'test-prompt', got '%s'", prompt.Name)
	}

	// Test getting non-existent prompt
	_, err = pm.GetPrompt("non-existent")
	if err == nil {
		t.Error("GetPrompt() expected error for non-existent prompt, got nil")
	}
}

func TestListPrompts(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	// Add test prompts in non-alphabetical order
	pm.registry["zebra-prompt"] = &PromptDefinition{
		Name: "zebra-prompt",
		Messages: []MessageTemplate{
			{Role: "user", Content: ContentTemplate{Type: "text", Text: "Test"}},
		},
	}

	pm.registry["alpha-prompt"] = &PromptDefinition{
		Name: "alpha-prompt",
		Messages: []MessageTemplate{
			{Role: "user", Content: ContentTemplate{Type: "text", Text: "Test"}},
		},
	}

	pm.registry["middle-prompt"] = &PromptDefinition{
		Name: "middle-prompt",
		Messages: []MessageTemplate{
			{Role: "user", Content: ContentTemplate{Type: "text", Text: "Test"}},
		},
	}

	// List prompts
	prompts := pm.ListPrompts()

	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(prompts))
	}

	// Verify alphabetical order
	if prompts[0].Name != "alpha-prompt" {
		t.Errorf("Expected first prompt 'alpha-prompt', got '%s'", prompts[0].Name)
	}

	if prompts[1].Name != "middle-prompt" {
		t.Errorf("Expected second prompt 'middle-prompt', got '%s'", prompts[1].Name)
	}

	if prompts[2].Name != "zebra-prompt" {
		t.Errorf("Expected third prompt 'zebra-prompt', got '%s'", prompts[2].Name)
	}
}

func TestListPromptsEmpty(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	prompts := pm.ListPrompts()

	if len(prompts) != 0 {
		t.Errorf("Expected empty list, got %d prompts", len(prompts))
	}
}

func TestRenderPrompt(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	// Add test document to cache
	testDoc := &models.Document{
		Metadata: models.DocumentMetadata{
			Path:     config.PatternsPath + "/test-pattern.md",
			Category: "patterns",
			Title:    "Test Pattern",
		},
		Content: models.DocumentContent{
			RawContent: "This is a test pattern",
		},
	}
	cache.Set(config.PatternsPath+"/test-pattern.md", testDoc)

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	// Add test prompt
	pm.registry["test-prompt"] = &PromptDefinition{
		Name:        "test-prompt",
		Description: "Test prompt",
		Arguments: []ArgumentDefinition{
			{Name: "code", Required: true, MaxLength: 1000},
			{Name: "language", Required: false},
		},
		Messages: []MessageTemplate{
			{
				Role: "user",
				Content: ContentTemplate{
					Type: "text",
					Text: "Review this {{language}} code: {{code}}",
				},
			},
		},
	}

	// Test successful render
	result, err := pm.RenderPrompt("test-prompt", map[string]interface{}{
		"code":     "func main() {}",
		"language": "Go",
	})

	if err != nil {
		t.Fatalf("RenderPrompt() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("RenderPrompt() returned nil result")
	}

	if result.Description != "Test prompt" {
		t.Errorf("Expected description 'Test prompt', got '%s'", result.Description)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(result.Messages))
	}

	expectedText := "Review this Go code: func main() {}"
	if result.Messages[0].Content.Text != expectedText {
		t.Errorf("Expected text '%s', got '%s'", expectedText, result.Messages[0].Content.Text)
	}

	// Test missing required argument
	_, err = pm.RenderPrompt("test-prompt", map[string]interface{}{
		"language": "Go",
	})

	if err == nil {
		t.Error("RenderPrompt() expected error for missing required argument, got nil")
	}

	// Test non-existent prompt
	_, err = pm.RenderPrompt("non-existent", map[string]interface{}{})

	if err == nil {
		t.Error("RenderPrompt() expected error for non-existent prompt, got nil")
	}
}

func TestReloadPrompts(t *testing.T) {
	tmpDir := t.TempDir()

	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager(tmpDir, cache, monitor, logger)

	// Initial load (empty directory)
	err = pm.LoadPrompts()
	if err != nil {
		t.Fatalf("LoadPrompts() unexpected error: %v", err)
	}

	if len(pm.registry) != 0 {
		t.Errorf("Expected empty registry, got %d prompts", len(pm.registry))
	}

	// Add a prompt file
	promptContent := `{
		"name": "new-prompt",
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Test"
				}
			}
		]
	}`

	if err := os.WriteFile(filepath.Join(tmpDir, "new-prompt.json"), []byte(promptContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Reload
	err = pm.ReloadPrompts()
	if err != nil {
		t.Fatalf("ReloadPrompts() unexpected error: %v", err)
	}

	if len(pm.registry) != 1 {
		t.Errorf("Expected 1 prompt after reload, got %d", len(pm.registry))
	}

	if _, exists := pm.registry["new-prompt"]; !exists {
		t.Error("Expected 'new-prompt' to be in registry after reload")
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	// Add test prompt
	pm.registry["test-prompt"] = &PromptDefinition{
		Name: "test-prompt",
		Arguments: []ArgumentDefinition{
			{Name: "arg1", Required: true},
		},
		Messages: []MessageTemplate{
			{
				Role: "user",
				Content: ContentTemplate{
					Type: "text",
					Text: "Test {{arg1}}",
				},
			},
		},
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				pm.GetPrompt("test-prompt")
				pm.ListPrompts()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHandleFileEvent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial prompt file
	promptContent := `{
		"name": "test-prompt",
		"messages": [
			{
				"role": "user",
				"content": {
					"type": "text",
					"text": "Test"
				}
			}
		]
	}`

	promptPath := filepath.Join(tmpDir, "test-prompt.json")
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager(tmpDir, cache, monitor, logger)

	// Load initial prompts
	if err := pm.LoadPrompts(); err != nil {
		t.Fatalf("LoadPrompts() unexpected error: %v", err)
	}

	if len(pm.registry) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(pm.registry))
	}

	// Simulate file event
	event := models.FileEvent{
		Type:  "modify",
		Path:  promptPath,
		IsDir: false,
	}

	pm.handleFileEvent(event)

	// Wait for debounce timer
	time.Sleep(600 * time.Millisecond)

	// Verify reload happened (registry should still have the prompt)
	if len(pm.registry) != 1 {
		t.Errorf("Expected 1 prompt after file event, got %d", len(pm.registry))
	}
}

func TestHandleFileEventNonJSON(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	monitor, err := monitor.NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.StopWatching()

	logger := logging.NewStructuredLogger("test")
	pm := NewPromptManager("prompts", cache, monitor, logger)

	// Simulate non-JSON file event (should be ignored)
	event := models.FileEvent{
		Type:  "modify",
		Path:  "readme.txt",
		IsDir: false,
	}

	// Should not panic or error
	pm.handleFileEvent(event)
}
