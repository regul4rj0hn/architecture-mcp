package prompts

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
)

const (
	// MaxResourcesPerPrompt limits the number of resources that can be embedded
	MaxResourcesPerPrompt = 50
	// MaxTotalContentSize limits the total size of embedded content (1MB)
	MaxTotalContentSize = 1024 * 1024
)

// TemplateRenderer handles template variable substitution and resource embedding
type TemplateRenderer struct {
	cache         *cache.DocumentCache
	statsRecorder StatsRecorder
	toolManager   ToolManagerInterface
}

// StatsRecorder is an interface for recording statistics
type StatsRecorder interface {
	RecordResourceEmbedding(cacheHit bool)
}

// NewTemplateRenderer creates a new template renderer with access to the document cache
func NewTemplateRenderer(cache *cache.DocumentCache) *TemplateRenderer {
	return &TemplateRenderer{
		cache:         cache,
		statsRecorder: nil, // Will be set later by SetStatsRecorder
		toolManager:   nil, // Will be set later by SetToolManager
	}
}

// SetStatsRecorder sets the stats recorder for tracking metrics
func (tr *TemplateRenderer) SetStatsRecorder(recorder StatsRecorder) {
	tr.statsRecorder = recorder
}

// SetToolManager sets the tool manager for tool reference expansion
func (tr *TemplateRenderer) SetToolManager(manager ToolManagerInterface) {
	tr.toolManager = manager
}

var (
	// variablePattern matches {{variableName}} for substitution
	variablePattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_-]+)\}\}`)
	// resourcePattern matches {{resource:uri}} for resource embedding
	resourcePattern = regexp.MustCompile(`\{\{resource:([^}]+)\}\}`)
	// toolPattern matches {{tool:tool-name}} for tool reference embedding
	toolPattern = regexp.MustCompile(`\{\{tool:([a-z0-9-]+)\}\}`)
)

// RenderTemplate performs variable substitution on a template string
// Variables are specified as {{variableName}} and replaced with values from args
func (tr *TemplateRenderer) RenderTemplate(template string, args map[string]any) (string, error) {
	matches := variablePattern.FindAllStringSubmatch(template, -1)
	result := template

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		placeholder := match[0] // Full match like {{variableName}}
		varName := match[1]     // Variable name without braces

		value, exists := args[varName]
		if !exists {
			// Variable not provided - leave placeholder as-is
			// Arguments are validated before rendering, so missing required args won't reach here
			continue
		}

		strValue := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, strValue)
	}

	return result, nil
}

// EmbedResources processes resource embedding patterns in the template
// Resource patterns are specified as {{resource:uri}} where uri can include wildcards
func (tr *TemplateRenderer) EmbedResources(template string) (string, error) {
	matches := resourcePattern.FindAllStringSubmatch(template, -1)
	result := template
	totalSize := 0
	resourceCount := 0

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		placeholder := match[0] // Full match like {{resource:architecture://patterns/*}}
		pattern := match[1]     // URI pattern

		documents, err := tr.ResolveResourcePattern(pattern)
		if err != nil {
			return "", fmt.Errorf("failed to resolve resource pattern %s: %w", pattern, err)
		}

		resourceCount += len(documents)
		if resourceCount > MaxResourcesPerPrompt {
			return "", fmt.Errorf("resource limit exceeded: maximum %d resources allowed per prompt", MaxResourcesPerPrompt)
		}

		embeddedContent, size, err := tr.buildEmbeddedContent(documents, totalSize)
		if err != nil {
			return "", err
		}
		totalSize = size

		result = strings.ReplaceAll(result, placeholder, embeddedContent)
	}

	return result, nil
}

// EmbedTools processes tool reference patterns in the template
// Tool patterns are specified as {{tool:tool-name}} and are expanded to include
// the tool's description and input schema
func (tr *TemplateRenderer) EmbedTools(template string) (string, error) {
	if tr.toolManager == nil {
		// If no tool manager is set, return template unchanged
		// This allows the renderer to work without tools support
		return template, nil
	}

	matches := toolPattern.FindAllStringSubmatch(template, -1)
	result := template

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		placeholder := match[0] // Full match like {{tool:validate-against-pattern}}
		toolName := match[1]    // Tool name

		tool, err := tr.toolManager.GetTool(toolName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve tool reference %s: %w", toolName, err)
		}

		expandedContent := tr.buildToolReference(tool)
		result = strings.ReplaceAll(result, placeholder, expandedContent)
	}

	return result, nil
}

// buildToolReference formats a tool into an expanded reference with description and schema
func (tr *TemplateRenderer) buildToolReference(tool ToolInterface) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Tool: %s\n", tool.Name()))
	builder.WriteString(fmt.Sprintf("Description: %s\n", tool.Description()))

	schema := tool.InputSchema()
	if schema != nil {
		builder.WriteString("Parameters:\n")

		// Extract properties from schema
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			required := make(map[string]bool)
			if reqList, ok := schema["required"].([]interface{}); ok {
				for _, r := range reqList {
					if reqStr, ok := r.(string); ok {
						required[reqStr] = true
					}
				}
			}

			for paramName, paramDef := range properties {
				if paramMap, ok := paramDef.(map[string]interface{}); ok {
					requiredStr := ""
					if required[paramName] {
						requiredStr = " (required)"
					} else {
						requiredStr = " (optional)"
					}

					description := ""
					if desc, ok := paramMap["description"].(string); ok {
						description = desc
					}

					// Format parameter with constraints
					constraints := tr.formatParameterConstraints(paramMap)
					if constraints != "" {
						builder.WriteString(fmt.Sprintf("  - %s%s: %s %s\n", paramName, requiredStr, description, constraints))
					} else {
						builder.WriteString(fmt.Sprintf("  - %s%s: %s\n", paramName, requiredStr, description))
					}
				}
			}
		}
	}

	return builder.String()
}

// formatParameterConstraints extracts and formats parameter constraints from schema
func (tr *TemplateRenderer) formatParameterConstraints(paramMap map[string]interface{}) string {
	var constraints []string

	if maxLength, ok := paramMap["maxLength"].(float64); ok {
		constraints = append(constraints, fmt.Sprintf("max %d chars", int(maxLength)))
	}

	if minLength, ok := paramMap["minLength"].(float64); ok {
		constraints = append(constraints, fmt.Sprintf("min %d chars", int(minLength)))
	}

	if maximum, ok := paramMap["maximum"].(float64); ok {
		constraints = append(constraints, fmt.Sprintf("max %d", int(maximum)))
	}

	if minimum, ok := paramMap["minimum"].(float64); ok {
		constraints = append(constraints, fmt.Sprintf("min %d", int(minimum)))
	}

	if enum, ok := paramMap["enum"].([]interface{}); ok {
		enumStrs := make([]string, 0, len(enum))
		for _, e := range enum {
			enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
		}
		constraints = append(constraints, fmt.Sprintf("one of: %s", strings.Join(enumStrs, ", ")))
	}

	if len(constraints) > 0 {
		return "(" + strings.Join(constraints, ", ") + ")"
	}

	return ""
}

// buildEmbeddedContent formats documents into embedded content with size checking
func (tr *TemplateRenderer) buildEmbeddedContent(documents []*models.Document, currentSize int) (string, int, error) {
	var builder strings.Builder
	totalSize := currentSize

	for i, doc := range documents {
		content := doc.Content.RawContent
		totalSize += len(content)

		if totalSize > MaxTotalContentSize {
			return "", 0, fmt.Errorf("content size limit exceeded: maximum %d bytes allowed per prompt", MaxTotalContentSize)
		}

		if i > 0 {
			builder.WriteString("\n\n---\n\n")
		}

		builder.WriteString(fmt.Sprintf("# %s\n", doc.Metadata.Title))
		builder.WriteString(fmt.Sprintf("Source: %s\n\n", doc.Metadata.Path))
		builder.WriteString(content)
	}

	return builder.String(), totalSize, nil
}

// ResolveResourcePattern matches a URI pattern against cached documents
// Supports wildcards like architecture://patterns/* to match multiple documents
func (tr *TemplateRenderer) ResolveResourcePattern(pattern string) ([]*models.Document, error) {
	if !strings.HasPrefix(pattern, "architecture://") {
		return nil, fmt.Errorf("invalid resource URI scheme: must start with architecture://")
	}

	category, resourcePath, err := tr.parseResourceURI(pattern)
	if err != nil {
		return nil, err
	}

	allDocs := tr.cache.GetAllDocuments()
	matchedDocs := tr.matchDocuments(allDocs, category, resourcePath)

	if len(matchedDocs) == 0 {
		return nil, fmt.Errorf("no resources found matching pattern: %s", pattern)
	}

	// Record resource embedding with cache hit (all documents come from cache)
	if tr.statsRecorder != nil {
		for range matchedDocs {
			tr.statsRecorder.RecordResourceEmbedding(true)
		}
	}

	return matchedDocs, nil
}

// parseResourceURI extracts category and resource path from a URI pattern
func (tr *TemplateRenderer) parseResourceURI(pattern string) (string, string, error) {
	path := strings.TrimPrefix(pattern, "architecture://")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 1 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid resource URI: missing category")
	}

	category := parts[0]
	resourcePath := ""
	if len(parts) > 1 {
		resourcePath = parts[1]
	}

	return category, resourcePath, nil
}

// matchDocuments finds all documents matching the category and resource path pattern
func (tr *TemplateRenderer) matchDocuments(allDocs map[string]*models.Document, category, resourcePath string) []*models.Document {
	var matchedDocs []*models.Document
	isWildcard := strings.Contains(resourcePath, "*")

	for docPath, doc := range allDocs {
		if doc.Metadata.Category != category {
			continue
		}

		if tr.documentMatches(docPath, resourcePath, isWildcard, category) {
			matchedDocs = append(matchedDocs, doc)
		}
	}

	return matchedDocs
}

// documentMatches checks if a document path matches the resource pattern
func (tr *TemplateRenderer) documentMatches(docPath, resourcePath string, isWildcard bool, category string) bool {
	if isWildcard {
		return tr.wildcardMatch(docPath, resourcePath)
	}
	return tr.exactMatch(docPath, resourcePath, category)
}

// wildcardMatch performs wildcard pattern matching
func (tr *TemplateRenderer) wildcardMatch(docPath, resourcePath string) bool {
	if resourcePath == "*" {
		return true
	}

	matched, err := filepath.Match(resourcePath, filepath.Base(docPath))
	return err == nil && matched
}

// exactMatch performs exact path matching
func (tr *TemplateRenderer) exactMatch(docPath, resourcePath, category string) bool {
	// Map category to the appropriate path constant
	var basePath string
	switch category {
	case config.CategoryGuideline:
		basePath = config.GuidelinesPath
	case config.CategoryPattern:
		basePath = config.PatternsPath
	case config.CategoryADR:
		basePath = config.ADRPath
	default:
		basePath = filepath.Join(config.ResourcesBasePath, category)
	}

	expectedPath := filepath.Join(basePath, resourcePath)
	if !strings.HasSuffix(expectedPath, ".md") {
		expectedPath += ".md"
	}
	return docPath == expectedPath
}
