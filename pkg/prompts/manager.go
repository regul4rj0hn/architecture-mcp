package prompts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/logging"
	"mcp-architecture-service/pkg/monitor"
)

// PromptManager manages the lifecycle of prompt definitions
type PromptManager struct {
	registry      map[string]*PromptDefinition
	promptsDir    string
	cache         *cache.DocumentCache
	monitor       *monitor.FileSystemMonitor
	renderer      *TemplateRenderer
	mu            sync.RWMutex
	logger        *logging.StructuredLogger
	debounceTimer *time.Timer
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(promptsDir string, cache *cache.DocumentCache, monitor *monitor.FileSystemMonitor) *PromptManager {
	return &PromptManager{
		registry:   make(map[string]*PromptDefinition),
		promptsDir: promptsDir,
		cache:      cache,
		monitor:    monitor,
		renderer:   NewTemplateRenderer(cache),
		logger:     logging.NewStructuredLogger("PromptManager"),
	}
}

// LoadPrompts scans the prompts directory and loads all JSON prompt definitions
func (pm *PromptManager) LoadPrompts() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if prompts directory exists
	if _, err := os.Stat(pm.promptsDir); os.IsNotExist(err) {
		pm.logger.WithContext("prompts_dir", pm.promptsDir).
			Warn("Prompts directory does not exist, starting with empty registry")
		return nil
	}

	// Clear existing registry
	pm.registry = make(map[string]*PromptDefinition)

	// Read all files in prompts directory
	entries, err := os.ReadDir(pm.promptsDir)
	if err != nil {
		return fmt.Errorf("failed to read prompts directory: %w", err)
	}

	loadedCount := 0
	errorCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process JSON files
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(pm.promptsDir, entry.Name())
		if err := pm.loadPromptFile(filePath); err != nil {
			pm.logger.WithError(err).
				WithContext("file", filePath).
				Error("Failed to load prompt definition, skipping")
			errorCount++
			continue
		}

		loadedCount++
	}

	pm.logger.WithContext("loaded", loadedCount).
		WithContext("errors", errorCount).
		WithContext("total", len(pm.registry)).
		Info("Prompt definitions loaded")

	return nil
}

// loadPromptFile loads a single prompt definition file
func (pm *PromptManager) loadPromptFile(filePath string) error {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var def PromptDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate definition
	if err := def.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Add to registry
	pm.registry[def.Name] = &def

	pm.logger.WithContext("prompt_name", def.Name).
		WithContext("file", filepath.Base(filePath)).
		Debug("Prompt definition loaded")

	return nil
}

// GetPrompt retrieves a prompt definition by name
func (pm *PromptManager) GetPrompt(name string) (*PromptDefinition, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	prompt, exists := pm.registry[name]
	if !exists {
		return nil, fmt.Errorf("prompt not found: %s", name)
	}

	return prompt, nil
}

// ListPrompts returns all available prompts sorted alphabetically by name
func (pm *PromptManager) ListPrompts() []models.MCPPrompt {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Collect all prompts
	prompts := make([]models.MCPPrompt, 0, len(pm.registry))
	for _, def := range pm.registry {
		prompts = append(prompts, def.ToMCPPrompt())
	}

	// Sort alphabetically by name
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})

	return prompts
}

// RenderPrompt validates arguments, renders templates, and embeds resources
func (pm *PromptManager) RenderPrompt(name string, arguments map[string]interface{}) (*models.MCPPromptsGetResult, error) {
	// Get prompt definition
	prompt, err := pm.GetPrompt(name)
	if err != nil {
		return nil, err
	}

	// Validate arguments
	if err := prompt.ValidateArguments(arguments); err != nil {
		return nil, fmt.Errorf("argument validation failed: %w", err)
	}

	// Render messages
	messages := make([]models.MCPPromptMessage, 0, len(prompt.Messages))

	for i, msgTemplate := range prompt.Messages {
		// Render template with arguments
		renderedText, err := pm.renderer.RenderTemplate(msgTemplate.Content.Text, arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to render message %d: %w", i, err)
		}

		// Embed resources
		finalText, err := pm.renderer.EmbedResources(renderedText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed resources in message %d: %w", i, err)
		}

		messages = append(messages, models.MCPPromptMessage{
			Role: msgTemplate.Role,
			Content: models.MCPPromptContent{
				Type: msgTemplate.Content.Type,
				Text: finalText,
			},
		})
	}

	result := &models.MCPPromptsGetResult{
		Description: prompt.Description,
		Messages:    messages,
	}

	pm.logger.WithContext("prompt_name", name).
		WithContext("message_count", len(messages)).
		Debug("Prompt rendered successfully")

	return result, nil
}

// ReloadPrompts refreshes the prompt registry by reloading all definitions
func (pm *PromptManager) ReloadPrompts() error {
	pm.logger.Info("Reloading prompt definitions")

	if err := pm.LoadPrompts(); err != nil {
		pm.logger.WithError(err).Error("Failed to reload prompts")
		return err
	}

	pm.logger.WithContext("total_prompts", len(pm.registry)).
		Info("Prompts reloaded successfully")

	return nil
}

// StartWatching sets up file system monitoring for the prompts directory
func (pm *PromptManager) StartWatching() error {
	// Check if prompts directory exists
	if _, err := os.Stat(pm.promptsDir); os.IsNotExist(err) {
		pm.logger.WithContext("prompts_dir", pm.promptsDir).
			Warn("Prompts directory does not exist, skipping file system monitoring")
		return nil
	}

	// Set up file system monitoring with debounced callback
	err := pm.monitor.WatchDirectory(pm.promptsDir, pm.handleFileEvent)
	if err != nil {
		return fmt.Errorf("failed to watch prompts directory: %w", err)
	}

	pm.logger.WithContext("prompts_dir", pm.promptsDir).
		Info("Started watching prompts directory for changes")

	return nil
}

// handleFileEvent processes file system events with debouncing
func (pm *PromptManager) handleFileEvent(event models.FileEvent) {
	// Only process JSON files
	if filepath.Ext(event.Path) != ".json" {
		return
	}

	pm.logger.WithContext("event_type", event.Type).
		WithContext("file", filepath.Base(event.Path)).
		Debug("Prompt file event detected")

	// Debounce rapid file changes (500ms delay)
	pm.mu.Lock()
	if pm.debounceTimer != nil {
		pm.debounceTimer.Stop()
	}

	pm.debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
		if err := pm.ReloadPrompts(); err != nil {
			pm.logger.WithError(err).Error("Failed to reload prompts after file change")
		}
	})
	pm.mu.Unlock()
}
