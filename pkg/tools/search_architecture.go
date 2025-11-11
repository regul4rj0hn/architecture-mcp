package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/logging"
)

// SearchArchitectureTool searches architectural documentation by keywords
type SearchArchitectureTool struct {
	cache  *cache.DocumentCache
	logger *logging.StructuredLogger
}

// NewSearchArchitectureTool creates a new SearchArchitectureTool instance
func NewSearchArchitectureTool(cache *cache.DocumentCache, logger *logging.StructuredLogger) *SearchArchitectureTool {
	return &SearchArchitectureTool{
		cache:  cache,
		logger: logger,
	}
}

// Name returns the unique identifier for the tool
func (sat *SearchArchitectureTool) Name() string {
	return "search-architecture"
}

// Description returns a human-readable description
func (sat *SearchArchitectureTool) Description() string {
	return "Searches architectural documentation by keywords or tags to quickly find relevant patterns, guidelines, and ADRs"
}

// InputSchema returns JSON schema for tool parameters
func (sat *SearchArchitectureTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
				"maxLength":   500,
			},
			"resource_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{config.CategoryGuideline, config.CategoryPattern, config.CategoryADR, "all"},
				"description": "Filter by resource type (default: all)",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"minimum":     1,
				"maximum":     20,
				"description": "Maximum results to return (default: 10)",
			},
		},
		"required": []string{"query"},
	}
}

// Execute runs the tool with validated arguments
func (sat *SearchArchitectureTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	// Extract arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query argument must be a string")
	}

	// Validate query length
	if len(query) > 500 {
		return nil, fmt.Errorf("query exceeds maximum length of 500 characters")
	}

	// Extract optional resource_type
	resourceType := "all"
	if rt, ok := arguments["resource_type"].(string); ok {
		resourceType = rt
	}

	// Validate resource_type
	validTypes := map[string]bool{
		config.CategoryGuideline: true,
		config.CategoryPattern:   true,
		config.CategoryADR:       true,
		"all":                    true,
	}
	if !validTypes[resourceType] {
		return nil, fmt.Errorf("invalid resource_type: must be one of %s, %s, %s, all",
			config.CategoryGuideline, config.CategoryPattern, config.CategoryADR)
	}

	// Extract optional max_results
	maxResults := 10
	if mr, ok := arguments["max_results"].(float64); ok {
		maxResults = int(mr)
	} else if mr, ok := arguments["max_results"].(int); ok {
		maxResults = mr
	}

	// Validate max_results
	if maxResults < 1 || maxResults > 20 {
		return nil, fmt.Errorf("max_results must be between 1 and 20")
	}

	sat.logger.WithContext("query", query).
		WithContext("resource_type", resourceType).
		WithContext("max_results", maxResults).
		Info("Searching architecture documentation")

	// Perform search
	results := sat.search(query, resourceType, maxResults)

	return results, nil
}

// searchResult represents a single search result with relevance score
type searchResult struct {
	URI            string
	Title          string
	ResourceType   string
	RelevanceScore float64
	Excerpt        string
}

// search performs the actual search and ranking logic
func (sat *SearchArchitectureTool) search(query, resourceType string, maxResults int) map[string]interface{} {
	// Get all documents from cache
	allDocs := sat.cache.GetAllDocuments()

	// Tokenize query
	queryTokens := sat.tokenize(query)

	// Search and score documents
	var results []searchResult
	for path, doc := range allDocs {
		// Filter by resource type if specified
		if resourceType != "all" && doc.Metadata.Category != resourceType {
			continue
		}

		// Calculate relevance score
		score := sat.calculateRelevance(queryTokens, doc.Content.RawContent, doc.Metadata.Title)
		if score > 0 {
			// Extract excerpt
			excerpt := sat.extractExcerpt(doc.Content.RawContent, queryTokens)

			// Generate URI
			uri := sat.generateURI(doc.Metadata.Category, path)

			results = append(results, searchResult{
				URI:            uri,
				Title:          doc.Metadata.Title,
				ResourceType:   doc.Metadata.Category,
				RelevanceScore: score,
				Excerpt:        excerpt,
			})
		}
	}

	// Sort by relevance score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	// Limit results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	// Convert to output format
	resultList := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		resultList = append(resultList, map[string]interface{}{
			"uri":             result.URI,
			"title":           result.Title,
			"resource_type":   result.ResourceType,
			"relevance_score": result.RelevanceScore,
			"excerpt":         result.Excerpt,
		})
	}

	return map[string]interface{}{
		"results":       resultList,
		"total_matches": len(results),
	}
}

// tokenize splits text into lowercase tokens
func (sat *SearchArchitectureTool) tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split on whitespace and common punctuation
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?'
	})

	// Remove empty tokens and very short tokens
	var filtered []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if len(token) >= 2 {
			filtered = append(filtered, token)
		}
	}

	return filtered
}

// calculateRelevance computes a relevance score for a document
func (sat *SearchArchitectureTool) calculateRelevance(queryTokens []string, content, title string) float64 {
	if len(queryTokens) == 0 {
		return 0
	}

	contentLower := strings.ToLower(content)
	titleLower := strings.ToLower(title)

	var score float64

	// Score based on title matches (higher weight)
	for _, token := range queryTokens {
		if strings.Contains(titleLower, token) {
			score += 10.0
		}
	}

	// Score based on content matches
	for _, token := range queryTokens {
		// Count occurrences in content
		count := strings.Count(contentLower, token)
		if count > 0 {
			// Use logarithmic scoring to avoid over-weighting documents with many matches
			score += 1.0 + float64(count)*0.5
		}
	}

	// Bonus for matching multiple query tokens (phrase proximity)
	matchedTokens := 0
	for _, token := range queryTokens {
		if strings.Contains(contentLower, token) {
			matchedTokens++
		}
	}
	if matchedTokens > 1 {
		score += float64(matchedTokens) * 2.0
	}

	// Normalize by document length to avoid bias toward longer documents
	docLength := float64(len(content))
	if docLength > 0 {
		score = score * 1000.0 / docLength
	}

	return score
}

// extractExcerpt extracts a relevant excerpt from the document
func (sat *SearchArchitectureTool) extractExcerpt(content string, queryTokens []string) string {
	const maxExcerptLength = 200

	if len(content) == 0 {
		return ""
	}

	contentLower := strings.ToLower(content)

	// Find the first occurrence of any query token
	bestPos := -1
	for _, token := range queryTokens {
		pos := strings.Index(contentLower, token)
		if pos != -1 && (bestPos == -1 || pos < bestPos) {
			bestPos = pos
		}
	}

	// If no match found, return beginning of content
	if bestPos == -1 {
		if len(content) <= maxExcerptLength {
			return content
		}
		return content[:maxExcerptLength] + "..."
	}

	// Extract excerpt around the match
	start := bestPos - 50
	if start < 0 {
		start = 0
	}

	end := bestPos + 150
	if end > len(content) {
		end = len(content)
	}

	excerpt := content[start:end]

	// Trim to word boundaries
	excerpt = strings.TrimSpace(excerpt)

	// Add ellipsis if truncated
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(content) {
		excerpt = excerpt + "..."
	}

	return excerpt
}

// generateURI creates a proper architecture:// URI for a document
func (sat *SearchArchitectureTool) generateURI(category, path string) string {
	// Extract filename from path
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	// Remove .md extension
	filename = strings.TrimSuffix(filename, config.MarkdownExtension)

	// Map category to URI path segment
	var uriCategory string
	switch category {
	case config.CategoryGuideline:
		uriCategory = config.URIGuidelines
	case config.CategoryPattern:
		uriCategory = config.URIPatterns
	case config.CategoryADR:
		uriCategory = config.URIADR
	default:
		uriCategory = config.URIUnknown
	}

	// Generate URI using config constants
	return fmt.Sprintf("%s%s/%s", config.URIScheme, uriCategory, filename)
}
