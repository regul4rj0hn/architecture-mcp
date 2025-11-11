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

	// Performance metrics
	stats PromptStats
}

// PromptStats tracks performance metrics for prompt operations
type PromptStats struct {
	TotalInvocations      int64
	FailedInvocations     int64
	InvocationsByName     map[string]int64
	TotalRenderTimeMs     int64
	RenderTimeByName      map[string]int64
	ResourceEmbeddings    int64
	ResourceEmbedCacheHit int64
	mu                    sync.RWMutex
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(promptsDir string, cache *cache.DocumentCache, monitor *monitor.FileSystemMonitor) *PromptManager {
	renderer := NewTemplateRenderer(cache)
	pm := &PromptManager{
		registry:   make(map[string]*PromptDefinition),
		promptsDir: promptsDir,
		cache:      cache,
		monitor:    monitor,
		renderer:   renderer,
		logger:     logging.NewStructuredLogger("PromptManager"),
		stats: PromptStats{
			InvocationsByName: make(map[string]int64),
			RenderTimeByName:  make(map[string]int64),
		},
	}
	// Set the stats recorder on the renderer
	renderer.SetStatsRecorder(pm)
	return pm
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
	startTime := time.Now()

	// Track invocation
	pm.recordInvocation(name)

	// Sanitize arguments for logging (truncate long values)
	sanitizedArgs := pm.sanitizeArgumentsForLogging(arguments)

	pm.logger.WithContext("prompt_name", name).
		WithContext("arguments", sanitizedArgs).
		Info("Prompt invocation started")

	// Get prompt definition
	prompt, err := pm.GetPrompt(name)
	if err != nil {
		duration := time.Since(startTime)
		pm.recordFailedInvocation(name, duration)
		pm.logger.WithError(err).
			WithContext("prompt_name", name).
			WithContext("duration_ms", duration.Milliseconds()).
			Error("Failed to get prompt definition")
		return nil, err
	}

	// Validate arguments
	if err := prompt.ValidateArguments(arguments); err != nil {
		duration := time.Since(startTime)
		pm.recordFailedInvocation(name, duration)
		pm.logger.WithError(err).
			WithContext("prompt_name", name).
			WithContext("duration_ms", duration.Milliseconds()).
			WithContext("arguments", sanitizedArgs).
			Error("Prompt argument validation failed")
		return nil, fmt.Errorf("argument validation failed: %w", err)
	}

	// Render messages
	messages := make([]models.MCPPromptMessage, 0, len(prompt.Messages))

	for i, msgTemplate := range prompt.Messages {
		// Render template with arguments
		renderedText, err := pm.renderer.RenderTemplate(msgTemplate.Content.Text, arguments)
		if err != nil {
			duration := time.Since(startTime)
			pm.recordFailedInvocation(name, duration)
			pm.logger.WithError(err).
				WithContext("prompt_name", name).
				WithContext("message_index", i).
				WithContext("duration_ms", duration.Milliseconds()).
				Error("Failed to render prompt template")
			return nil, fmt.Errorf("failed to render message %d: %w", i, err)
		}

		// Embed resources
		finalText, err := pm.renderer.EmbedResources(renderedText)
		if err != nil {
			duration := time.Since(startTime)
			pm.recordFailedInvocation(name, duration)
			pm.logger.WithError(err).
				WithContext("prompt_name", name).
				WithContext("message_index", i).
				WithContext("duration_ms", duration.Milliseconds()).
				Error("Failed to embed resources in prompt")
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

	duration := time.Since(startTime)
	pm.recordSuccessfulRender(name, duration)
	pm.logger.WithContext("prompt_name", name).
		WithContext("message_count", len(messages)).
		WithContext("duration_ms", duration.Milliseconds()).
		WithContext("arguments", sanitizedArgs).
		Info("Prompt rendered successfully")

	return result, nil
}

// sanitizeArgumentsForLogging sanitizes argument values for safe logging
func (pm *PromptManager) sanitizeArgumentsForLogging(arguments map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range arguments {
		if strValue, ok := value.(string); ok {
			// Truncate long string values
			if len(strValue) > 100 {
				sanitized[key] = fmt.Sprintf("%s... [%d chars total]", strValue[:100], len(strValue))
			} else {
				sanitized[key] = strValue
			}
		} else {
			sanitized[key] = value
		}
	}

	return sanitized
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

// recordInvocation records a prompt invocation
func (pm *PromptManager) recordInvocation(name string) {
	pm.stats.mu.Lock()
	defer pm.stats.mu.Unlock()

	pm.stats.TotalInvocations++
	pm.stats.InvocationsByName[name]++
}

// recordSuccessfulRender records a successful prompt render with duration
func (pm *PromptManager) recordSuccessfulRender(name string, duration time.Duration) {
	pm.stats.mu.Lock()
	defer pm.stats.mu.Unlock()

	durationMs := duration.Milliseconds()
	pm.stats.TotalRenderTimeMs += durationMs
	pm.stats.RenderTimeByName[name] += durationMs
}

// recordFailedInvocation records a failed prompt invocation
func (pm *PromptManager) recordFailedInvocation(name string, duration time.Duration) {
	pm.stats.mu.Lock()
	defer pm.stats.mu.Unlock()

	pm.stats.FailedInvocations++
	durationMs := duration.Milliseconds()
	pm.stats.TotalRenderTimeMs += durationMs
	pm.stats.RenderTimeByName[name] += durationMs
}

// RecordResourceEmbedding records a resource embedding operation
func (pm *PromptManager) RecordResourceEmbedding(cacheHit bool) {
	pm.stats.mu.Lock()
	defer pm.stats.mu.Unlock()

	pm.stats.ResourceEmbeddings++
	if cacheHit {
		pm.stats.ResourceEmbedCacheHit++
	}
}

// GetPerformanceMetrics returns performance metrics for prompt operations
func (pm *PromptManager) GetPerformanceMetrics() map[string]interface{} {
	pm.mu.RLock()
	totalPrompts := len(pm.registry)
	pm.mu.RUnlock()

	pm.stats.mu.RLock()
	defer pm.stats.mu.RUnlock()

	// Calculate average render time
	var avgRenderTime float64
	successfulInvocations := pm.stats.TotalInvocations - pm.stats.FailedInvocations
	if successfulInvocations > 0 {
		avgRenderTime = float64(pm.stats.TotalRenderTimeMs) / float64(successfulInvocations)
	}

	// Calculate resource embedding cache hit rate
	var resourceCacheHitRate float64
	if pm.stats.ResourceEmbeddings > 0 {
		resourceCacheHitRate = float64(pm.stats.ResourceEmbedCacheHit) / float64(pm.stats.ResourceEmbeddings) * 100.0
	}

	// Copy invocations by name for safe return
	invocationsByName := make(map[string]int64)
	for name, count := range pm.stats.InvocationsByName {
		invocationsByName[name] = count
	}

	// Calculate average render time by name
	avgRenderTimeByName := make(map[string]float64)
	for name, totalTime := range pm.stats.RenderTimeByName {
		if invocations := pm.stats.InvocationsByName[name]; invocations > 0 {
			avgRenderTimeByName[name] = float64(totalTime) / float64(invocations)
		}
	}

	return map[string]interface{}{
		"total_prompts_loaded":       totalPrompts,
		"total_invocations":          pm.stats.TotalInvocations,
		"successful_invocations":     successfulInvocations,
		"failed_invocations":         pm.stats.FailedInvocations,
		"invocations_by_name":        invocationsByName,
		"avg_render_time_ms":         avgRenderTime,
		"avg_render_time_by_name_ms": avgRenderTimeByName,
		"total_render_time_ms":       pm.stats.TotalRenderTimeMs,
		"resource_embeddings":        pm.stats.ResourceEmbeddings,
		"resource_cache_hit_rate":    resourceCacheHitRate,
	}
}
