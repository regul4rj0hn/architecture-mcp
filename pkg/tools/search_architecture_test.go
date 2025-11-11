package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/logging"
)

// TestNewSearchArchitectureTool tests the constructor
func TestNewSearchArchitectureTool(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")

	tool := NewSearchArchitectureTool(cache, logger)

	if tool == nil {
		t.Fatal("NewSearchArchitectureTool returned nil")
	}

	if tool.cache != cache {
		t.Error("Tool cache not set correctly")
	}

	if tool.logger != logger {
		t.Error("Tool logger not set correctly")
	}
}

// TestSearchArchitectureTool_Name tests the Name method
func TestSearchArchitectureTool_Name(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	name := tool.Name()
	expected := "search-architecture"

	if name != expected {
		t.Errorf("Expected name %s, got %s", expected, name)
	}
}

// TestSearchArchitectureTool_Description tests the Description method
func TestSearchArchitectureTool_Description(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	description := tool.Description()

	if description == "" {
		t.Error("Description should not be empty")
	}

	if !strings.Contains(strings.ToLower(description), "search") {
		t.Error("Description should mention search")
	}
}

// TestSearchArchitectureTool_InputSchema tests the InputSchema method
func TestSearchArchitectureTool_InputSchema(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	schema := tool.InputSchema()

	if schema == nil {
		t.Fatal("InputSchema returned nil")
	}

	// Check schema type
	schemaType, ok := schema["type"].(string)
	if !ok || schemaType != "object" {
		t.Error("Schema type should be 'object'")
	}

	// Check properties exist
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}

	// Check required fields
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("Schema should have required fields")
	}

	if len(required) != 1 || required[0] != "query" {
		t.Errorf("Expected 'query' as only required field, got %v", required)
	}

	// Verify query property
	queryProperty, ok := properties["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have 'query' property")
	}

	if queryProperty["type"] != "string" {
		t.Error("Query property should be string type")
	}

	if queryProperty["maxLength"] != 500 {
		t.Error("Query property should have maxLength of 500")
	}

	// Verify resource_type property
	resourceTypeProperty, ok := properties["resource_type"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have 'resource_type' property")
	}

	if resourceTypeProperty["type"] != "string" {
		t.Error("Resource_type property should be string type")
	}

	// Verify max_results property
	maxResultsProperty, ok := properties["max_results"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have 'max_results' property")
	}

	if maxResultsProperty["type"] != "integer" {
		t.Error("Max_results property should be integer type")
	}
}

// TestSearchArchitectureTool_Execute_BasicSearch tests search with various queries
func TestSearchArchitectureTool_Execute_BasicSearch(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test documents to cache
	setupTestDocuments(cache)

	tests := []struct {
		name          string
		query         string
		expectResults bool
		minResults    int
	}{
		{
			name:          "search for repository",
			query:         "repository",
			expectResults: true,
			minResults:    1,
		},
		{
			name:          "search for microservices",
			query:         "microservices",
			expectResults: true,
			minResults:    1,
		},
		{
			name:          "search for API design",
			query:         "API design",
			expectResults: true,
			minResults:    1,
		},
		{
			name:          "search for nonexistent term",
			query:         "xyznonexistent",
			expectResults: false,
			minResults:    0,
		},
		{
			name:          "search with multiple keywords",
			query:         "pattern implementation",
			expectResults: true,
			minResults:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]interface{}{
				"query": tt.query,
			}

			ctx := context.Background()
			result, err := tool.Execute(ctx, arguments)

			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatal("Result should be a map")
			}

			results, ok := resultMap["results"].([]map[string]interface{})
			if !ok {
				t.Fatal("Result should have 'results' field")
			}

			if tt.expectResults && len(results) < tt.minResults {
				t.Errorf("Expected at least %d results, got %d", tt.minResults, len(results))
			}

			if !tt.expectResults && len(results) > 0 {
				t.Errorf("Expected no results, got %d", len(results))
			}

			// Verify result structure
			for _, res := range results {
				if _, ok := res["uri"].(string); !ok {
					t.Error("Result should have 'uri' field")
				}
				if _, ok := res["title"].(string); !ok {
					t.Error("Result should have 'title' field")
				}
				if _, ok := res["resource_type"].(string); !ok {
					t.Error("Result should have 'resource_type' field")
				}
				if _, ok := res["relevance_score"].(float64); !ok {
					t.Error("Result should have 'relevance_score' field")
				}
				if _, ok := res["excerpt"].(string); !ok {
					t.Error("Result should have 'excerpt' field")
				}
			}
		})
	}
}

// TestSearchArchitectureTool_Execute_ResourceTypeFiltering tests filtering by resource type
func TestSearchArchitectureTool_Execute_ResourceTypeFiltering(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test documents to cache
	setupTestDocuments(cache)

	tests := []struct {
		name         string
		query        string
		resourceType string
		expectType   string
	}{
		{
			name:         "filter by guidelines",
			query:        "API",
			resourceType: config.CategoryGuideline,
			expectType:   config.CategoryGuideline,
		},
		{
			name:         "filter by patterns",
			query:        "repository",
			resourceType: config.CategoryPattern,
			expectType:   config.CategoryPattern,
		},
		{
			name:         "filter by adr",
			query:        "microservices",
			resourceType: config.CategoryADR,
			expectType:   config.CategoryADR,
		},
		{
			name:         "no filter (all)",
			query:        "architecture",
			resourceType: "all",
			expectType:   "", // Can be any type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]interface{}{
				"query":         tt.query,
				"resource_type": tt.resourceType,
			}

			ctx := context.Background()
			result, err := tool.Execute(ctx, arguments)

			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatal("Result should be a map")
			}

			results, ok := resultMap["results"].([]map[string]interface{})
			if !ok {
				t.Fatal("Result should have 'results' field")
			}

			// Verify all results match the expected type (if not "all")
			if tt.expectType != "" {
				for _, res := range results {
					resType, ok := res["resource_type"].(string)
					if !ok {
						t.Error("Result should have 'resource_type' field")
						continue
					}
					if resType != tt.expectType {
						t.Errorf("Expected resource_type %s, got %s", tt.expectType, resType)
					}
				}
			}
		})
	}
}

// TestSearchArchitectureTool_Execute_ResultLimiting tests max_results parameter
func TestSearchArchitectureTool_Execute_ResultLimiting(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test documents to cache
	setupTestDocuments(cache)

	tests := []struct {
		name       string
		query      string
		maxResults int
		wantMax    int
	}{
		{
			name:       "limit to 1 result",
			query:      "architecture",
			maxResults: 1,
			wantMax:    1,
		},
		{
			name:       "limit to 2 results",
			query:      "architecture",
			maxResults: 2,
			wantMax:    2,
		},
		{
			name:       "default limit (10)",
			query:      "architecture",
			maxResults: 0, // Will use default
			wantMax:    10,
		},
		{
			name:       "limit to 20 results (max)",
			query:      "architecture",
			maxResults: 20,
			wantMax:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]interface{}{
				"query": tt.query,
			}

			if tt.maxResults > 0 {
				arguments["max_results"] = tt.maxResults
			}

			ctx := context.Background()
			result, err := tool.Execute(ctx, arguments)

			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatal("Result should be a map")
			}

			results, ok := resultMap["results"].([]map[string]interface{})
			if !ok {
				t.Fatal("Result should have 'results' field")
			}

			if len(results) > tt.wantMax {
				t.Errorf("Expected at most %d results, got %d", tt.wantMax, len(results))
			}
		})
	}
}

// TestSearchArchitectureTool_Execute_RelevanceScoring tests relevance scoring
func TestSearchArchitectureTool_Execute_RelevanceScoring(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test documents to cache
	setupTestDocuments(cache)

	arguments := map[string]interface{}{
		"query": "repository pattern",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	results, ok := resultMap["results"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'results' field")
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Verify results are sorted by relevance score (descending)
	for i := 0; i < len(results)-1; i++ {
		score1, ok1 := results[i]["relevance_score"].(float64)
		score2, ok2 := results[i+1]["relevance_score"].(float64)

		if !ok1 || !ok2 {
			t.Error("Results should have relevance_score field")
			continue
		}

		if score1 < score2 {
			t.Errorf("Results not sorted by relevance: result[%d] score %.2f < result[%d] score %.2f",
				i, score1, i+1, score2)
		}
	}

	// Verify all scores are positive
	for i, res := range results {
		score, ok := res["relevance_score"].(float64)
		if !ok {
			t.Errorf("Result[%d] should have relevance_score field", i)
			continue
		}
		if score <= 0 {
			t.Errorf("Result[%d] should have positive relevance score, got %.2f", i, score)
		}
	}
}

// TestSearchArchitectureTool_Execute_ExcerptExtraction tests excerpt extraction
func TestSearchArchitectureTool_Execute_ExcerptExtraction(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test documents to cache
	setupTestDocuments(cache)

	arguments := map[string]interface{}{
		"query": "repository",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	results, ok := resultMap["results"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'results' field")
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Verify excerpts are present and contain query terms
	for i, res := range results {
		excerpt, ok := res["excerpt"].(string)
		if !ok {
			t.Errorf("Result[%d] should have 'excerpt' field", i)
			continue
		}

		if excerpt == "" {
			t.Errorf("Result[%d] excerpt should not be empty", i)
		}

		// Verify excerpt length is reasonable (not too long)
		if len(excerpt) > 250 {
			t.Errorf("Result[%d] excerpt too long: %d characters", i, len(excerpt))
		}

		// Verify excerpt contains at least one query term
		excerptLower := strings.ToLower(excerpt)
		if !strings.Contains(excerptLower, "repository") {
			t.Logf("Result[%d] excerpt may not contain query term (acceptable for some results): %s", i, excerpt)
		}
	}
}

// TestSearchArchitectureTool_Execute_InvalidArguments tests input validation
func TestSearchArchitectureTool_Execute_InvalidArguments(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	ctx := context.Background()

	tests := []struct {
		name      string
		arguments map[string]interface{}
		wantError string
	}{
		{
			name:      "missing query argument",
			arguments: map[string]interface{}{},
			wantError: "query argument must be a string",
		},
		{
			name: "query not a string",
			arguments: map[string]interface{}{
				"query": 12345,
			},
			wantError: "query argument must be a string",
		},
		{
			name: "query too long",
			arguments: map[string]interface{}{
				"query": strings.Repeat("a", 501),
			},
			wantError: "exceeds maximum length",
		},
		{
			name: "invalid resource_type",
			arguments: map[string]interface{}{
				"query":         "test",
				"resource_type": "invalid",
			},
			wantError: "invalid resource_type",
		},
		{
			name: "max_results too small",
			arguments: map[string]interface{}{
				"query":       "test",
				"max_results": 0,
			},
			wantError: "must be between 1 and 20",
		},
		{
			name: "max_results too large",
			arguments: map[string]interface{}{
				"query":       "test",
				"max_results": 21,
			},
			wantError: "must be between 1 and 20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, tt.arguments)

			if err == nil {
				t.Fatal("Expected error for invalid arguments")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Expected error containing '%s', got: %v", tt.wantError, err)
			}
		})
	}
}

// TestSearchArchitectureTool_Execute_EmptyCache tests search with no documents
func TestSearchArchitectureTool_Execute_EmptyCache(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Don't add any documents to cache

	arguments := map[string]interface{}{
		"query": "test",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Execute should not fail with empty cache: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	results, ok := resultMap["results"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'results' field")
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results with empty cache, got %d", len(results))
	}

	totalMatches, ok := resultMap["total_matches"].(int)
	if !ok {
		t.Fatal("Result should have 'total_matches' field")
	}

	if totalMatches != 0 {
		t.Errorf("Expected 0 total_matches with empty cache, got %d", totalMatches)
	}
}

// TestSearchArchitectureTool_Execute_TitleMatching tests that title matches score higher
func TestSearchArchitectureTool_Execute_TitleMatching(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add documents with query term in title vs content only
	doc1 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Repository Pattern Guide",
			Category:     config.CategoryPattern,
			Path:         "mcp/resources/patterns/repository-pattern.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: "This is a guide about design patterns.",
		},
	}

	doc2 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Design Patterns Overview",
			Category:     config.CategoryGuideline,
			Path:         "mcp/resources/guidelines/patterns.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: "The repository pattern is a common design pattern used in software architecture.",
		},
	}

	cache.Set(doc1.Metadata.Path, doc1)
	cache.Set(doc2.Metadata.Path, doc2)

	arguments := map[string]interface{}{
		"query": "repository",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	results, ok := resultMap["results"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'results' field")
	}

	if len(results) < 2 {
		t.Fatal("Expected at least 2 results")
	}

	// First result should be the one with "repository" in the title
	firstTitle, ok := results[0]["title"].(string)
	if !ok {
		t.Fatal("First result should have title")
	}

	if !strings.Contains(strings.ToLower(firstTitle), "repository") {
		t.Errorf("Expected first result to have 'repository' in title, got: %s", firstTitle)
	}
}

// TestSearchArchitectureTool_Execute_CaseInsensitive tests case-insensitive search
func TestSearchArchitectureTool_Execute_CaseInsensitive(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test document
	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Repository Pattern",
			Category:     config.CategoryPattern,
			Path:         "mcp/resources/patterns/repository.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: "The REPOSITORY pattern is used for data access.",
		},
	}
	cache.Set(doc.Metadata.Path, doc)

	tests := []struct {
		name  string
		query string
	}{
		{"lowercase", "repository"},
		{"uppercase", "REPOSITORY"},
		{"mixed case", "RePoSiToRy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]interface{}{
				"query": tt.query,
			}

			ctx := context.Background()
			result, err := tool.Execute(ctx, arguments)

			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatal("Result should be a map")
			}

			results, ok := resultMap["results"].([]map[string]interface{})
			if !ok {
				t.Fatal("Result should have 'results' field")
			}

			if len(results) == 0 {
				t.Error("Expected to find results regardless of case")
			}
		})
	}
}

// TestSearchArchitectureTool_Execute_URIGeneration tests proper URI generation
func TestSearchArchitectureTool_Execute_URIGeneration(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewSearchArchitectureTool(cache, logger)

	// Add test documents to cache
	setupTestDocuments(cache)

	arguments := map[string]interface{}{
		"query": "architecture",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	results, ok := resultMap["results"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'results' field")
	}

	// Verify URI format for each result
	for i, res := range results {
		uri, ok := res["uri"].(string)
		if !ok {
			t.Errorf("Result[%d] should have 'uri' field", i)
			continue
		}

		// Verify URI starts with architecture://
		if !strings.HasPrefix(uri, config.URIScheme) {
			t.Errorf("Result[%d] URI should start with %s, got: %s", i, config.URIScheme, uri)
		}

		// Verify URI contains valid category
		resourceType, _ := res["resource_type"].(string)
		var expectedCategory string
		switch resourceType {
		case config.CategoryGuideline:
			expectedCategory = config.URIGuidelines
		case config.CategoryPattern:
			expectedCategory = config.URIPatterns
		case config.CategoryADR:
			expectedCategory = config.URIADR
		}

		if expectedCategory != "" && !strings.Contains(uri, expectedCategory) {
			t.Errorf("Result[%d] URI should contain category %s, got: %s", i, expectedCategory, uri)
		}
	}
}

// setupTestDocuments adds test documents to the cache
func setupTestDocuments(cache *cache.DocumentCache) {
	// Add guideline document
	guideline := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "API Design Guidelines",
			Category:     config.CategoryGuideline,
			Path:         "mcp/resources/guidelines/api-design.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: `# API Design Guidelines

## Overview

This document provides guidelines for designing RESTful APIs in our architecture.

## Principles

- Use consistent naming conventions
- Follow REST principles
- Provide clear documentation
- Handle errors gracefully

## Best Practices

Design your APIs with the consumer in mind. Use standard HTTP methods and status codes.
`,
		},
	}

	// Add pattern document
	pattern := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Repository Pattern",
			Category:     config.CategoryPattern,
			Path:         "mcp/resources/patterns/repository-pattern.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: `# Repository Pattern

## Overview

The Repository pattern encapsulates the logic needed to access data sources.

## Implementation

Define a clear interface that abstracts the implementation details.

### Repository Interface

Create an interface for your repository operations.

### Concrete Implementation

Provide a concrete implementation of the pattern interface.

## Best Practices

- Keep interfaces focused and cohesive
- Use domain-specific method names
- Return domain objects, not data transfer objects
`,
		},
	}

	// Add ADR document
	adr := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Microservices Architecture",
			Category:     config.CategoryADR,
			Path:         "mcp/resources/adr/001-microservices-architecture.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: `# ADR 001: Microservices Architecture

## Status

Accepted

## Context

We need to decide on the overall architecture pattern for our system.

## Decision

We will adopt a microservices architecture to enable independent deployment and scaling.

## Consequences

- Increased operational complexity
- Better scalability and resilience
- Independent team autonomy
`,
		},
	}

	cache.Set(guideline.Metadata.Path, guideline)
	cache.Set(pattern.Metadata.Path, pattern)
	cache.Set(adr.Metadata.Path, adr)
}
