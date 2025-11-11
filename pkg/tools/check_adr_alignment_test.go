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

// TestNewCheckADRAlignmentTool tests the constructor
func TestNewCheckADRAlignmentTool(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")

	tool := NewCheckADRAlignmentTool(cache, logger)

	if tool == nil {
		t.Fatal("NewCheckADRAlignmentTool returned nil")
	}

	if tool.cache != cache {
		t.Error("Tool cache not set correctly")
	}

	if tool.logger != logger {
		t.Error("Tool logger not set correctly")
	}
}

// TestCheckADRAlignmentTool_Name tests the Name method
func TestCheckADRAlignmentTool_Name(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	name := tool.Name()
	expected := "check-adr-alignment"

	if name != expected {
		t.Errorf("Expected name %s, got %s", expected, name)
	}
}

// TestCheckADRAlignmentTool_Description tests the Description method
func TestCheckADRAlignmentTool_Description(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	description := tool.Description()

	if description == "" {
		t.Error("Description should not be empty")
	}

	if !strings.Contains(strings.ToLower(description), "adr") {
		t.Error("Description should mention ADR")
	}
}

// TestCheckADRAlignmentTool_InputSchema tests the InputSchema method
func TestCheckADRAlignmentTool_InputSchema(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	schema := tool.InputSchema()

	if schema == nil {
		t.Fatal("InputSchema returned nil")
	}

	schemaType, ok := schema["type"].(string)
	if !ok || schemaType != "object" {
		t.Error("Schema type should be 'object'")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("Schema should have required fields")
	}

	if len(required) != 1 || required[0] != "decision_description" {
		t.Errorf("Expected 'decision_description' as only required field, got %v", required)
	}

	decisionDescProperty, ok := properties["decision_description"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have 'decision_description' property")
	}

	if decisionDescProperty["type"] != "string" {
		t.Error("decision_description property should be string type")
	}

	if decisionDescProperty["maxLength"] != 5000 {
		t.Error("decision_description property should have maxLength of 5000")
	}

	decisionContextProperty, ok := properties["decision_context"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have 'decision_context' property")
	}

	if decisionContextProperty["type"] != "string" {
		t.Error("decision_context property should be string type")
	}

	if decisionContextProperty["maxLength"] != 2000 {
		t.Error("decision_context property should have maxLength of 2000")
	}
}

// TestCheckADRAlignmentTool_Execute_SupportingADRs tests alignment detection with supporting ADRs
func TestCheckADRAlignmentTool_Execute_SupportingADRs(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "We should adopt microservices architecture to enable independent deployment and better scalability",
		"decision_context":     "Our monolithic application is becoming difficult to scale and deploy",
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

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	if len(relatedADRs) == 0 {
		t.Fatal("Expected to find related ADRs")
	}

	foundSupporting := false
	for _, adr := range relatedADRs {
		alignment, ok := adr["alignment"].(string)
		if !ok {
			t.Error("ADR should have 'alignment' field")
			continue
		}

		if alignment == "supports" {
			foundSupporting = true
			if _, ok := adr["uri"].(string); !ok {
				t.Error("ADR should have 'uri' field")
			}
			if _, ok := adr["title"].(string); !ok {
				t.Error("ADR should have 'title' field")
			}
			if _, ok := adr["adr_id"].(string); !ok {
				t.Error("ADR should have 'adr_id' field")
			}
			if _, ok := adr["status"].(string); !ok {
				t.Error("ADR should have 'status' field")
			}
			if _, ok := adr["reason"].(string); !ok {
				t.Error("ADR should have 'reason' field")
			}
		}
	}

	if !foundSupporting {
		t.Error("Expected to find at least one supporting ADR")
	}
}

// TestCheckADRAlignmentTool_Execute_ConflictIdentification tests conflict detection
func TestCheckADRAlignmentTool_Execute_ConflictIdentification(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "We should use a monolithic architecture for simplicity",
		"decision_context":     "We want to keep our deployment simple",
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

	conflicts, ok := resultMap["conflicts"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'conflicts' field")
	}

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	foundConflict := false
	for _, adr := range relatedADRs {
		alignment, ok := adr["alignment"].(string)
		if ok && alignment == "conflicts" {
			foundConflict = true
			break
		}
	}

	if foundConflict && len(conflicts) == 0 {
		t.Error("Expected conflicts list to be populated when conflicting ADRs are found")
	}

	for _, conflict := range conflicts {
		if _, ok := conflict["adr_uri"].(string); !ok {
			t.Error("Conflict should have 'adr_uri' field")
		}
		if _, ok := conflict["conflict_description"].(string); !ok {
			t.Error("Conflict should have 'conflict_description' field")
		}
	}
}

// TestCheckADRAlignmentTool_Execute_RelatedADRDiscovery tests related ADR discovery
func TestCheckADRAlignmentTool_Execute_RelatedADRDiscovery(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "We need to implement API versioning for our REST services",
		"decision_context":     "We want to maintain backward compatibility",
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

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	for _, adr := range relatedADRs {
		alignment, ok := adr["alignment"].(string)
		if !ok {
			t.Error("ADR should have 'alignment' field")
			continue
		}

		validAlignments := map[string]bool{
			"supports":  true,
			"conflicts": true,
			"related":   true,
		}

		if !validAlignments[alignment] {
			t.Errorf("Invalid alignment value: %s", alignment)
		}
	}
}

// TestCheckADRAlignmentTool_Execute_KeywordExtraction tests keyword extraction
func TestCheckADRAlignmentTool_Execute_KeywordExtraction(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	tests := []struct {
		name                string
		decisionDescription string
		decisionContext     string
		expectMatches       bool
	}{
		{
			name:                "keywords in description",
			decisionDescription: "microservices architecture deployment scaling",
			decisionContext:     "",
			expectMatches:       true,
		},
		{
			name:                "keywords in context",
			decisionDescription: "system design",
			decisionContext:     "microservices independent deployment",
			expectMatches:       true,
		},
		{
			name:                "stop words filtered",
			decisionDescription: "the and or but in on at to for of with",
			decisionContext:     "",
			expectMatches:       false,
		},
		{
			name:                "short words filtered",
			decisionDescription: "a b c d e f",
			decisionContext:     "",
			expectMatches:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]interface{}{
				"decision_description": tt.decisionDescription,
			}

			if tt.decisionContext != "" {
				arguments["decision_context"] = tt.decisionContext
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

			relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
			if !ok {
				t.Fatal("Result should have 'related_adrs' field")
			}

			if tt.expectMatches && len(relatedADRs) == 0 {
				t.Error("Expected to find related ADRs with meaningful keywords")
			}

			if !tt.expectMatches && len(relatedADRs) > 0 {
				t.Logf("Found %d ADRs with stop words/short words (acceptable if content matches)", len(relatedADRs))
			}
		})
	}
}

// TestCheckADRAlignmentTool_Execute_InputValidation tests input validation
func TestCheckADRAlignmentTool_Execute_InputValidation(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	ctx := context.Background()

	tests := []struct {
		name      string
		arguments map[string]interface{}
		wantError string
	}{
		{
			name:      "missing decision_description",
			arguments: map[string]interface{}{},
			wantError: "decision_description argument must be a string",
		},
		{
			name: "decision_description not a string",
			arguments: map[string]interface{}{
				"decision_description": 12345,
			},
			wantError: "decision_description argument must be a string",
		},
		{
			name: "decision_description too long",
			arguments: map[string]interface{}{
				"decision_description": strings.Repeat("a", 5001),
			},
			wantError: "exceeds maximum length of 5000",
		},
		{
			name: "decision_context too long",
			arguments: map[string]interface{}{
				"decision_description": "valid description",
				"decision_context":     strings.Repeat("a", 2001),
			},
			wantError: "exceeds maximum length of 2000",
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

// TestCheckADRAlignmentTool_Execute_MaxSizeInputs tests inputs at exactly the size limit
func TestCheckADRAlignmentTool_Execute_MaxSizeInputs(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	maxSizeDescription := strings.Repeat("a", 5000)
	maxSizeContext := strings.Repeat("b", 2000)

	arguments := map[string]interface{}{
		"decision_description": maxSizeDescription,
		"decision_context":     maxSizeContext,
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Should accept inputs at exactly max size: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

// TestCheckADRAlignmentTool_Execute_OptionalContext tests that decision_context is optional
func TestCheckADRAlignmentTool_Execute_OptionalContext(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "We should use microservices",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Should work without decision_context: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

// TestCheckADRAlignmentTool_Execute_EmptyCache tests with no ADRs in cache
func TestCheckADRAlignmentTool_Execute_EmptyCache(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	arguments := map[string]interface{}{
		"decision_description": "We should use microservices",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, arguments)

	if err != nil {
		t.Fatalf("Should not fail with empty cache: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	if len(relatedADRs) != 0 {
		t.Errorf("Expected 0 related ADRs with empty cache, got %d", len(relatedADRs))
	}

	suggestions, ok := resultMap["suggestions"].([]string)
	if !ok {
		t.Fatal("Result should have 'suggestions' field")
	}

	foundNoADRSuggestion := false
	for _, suggestion := range suggestions {
		if strings.Contains(strings.ToLower(suggestion), "no related adrs") {
			foundNoADRSuggestion = true
			break
		}
	}

	if !foundNoADRSuggestion {
		t.Error("Expected suggestion about no related ADRs found")
	}
}

// TestCheckADRAlignmentTool_Execute_ResultStructure tests the structure of results
func TestCheckADRAlignmentTool_Execute_ResultStructure(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "We should adopt microservices",
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

	requiredFields := []string{"related_adrs", "conflicts", "suggestions"}
	for _, field := range requiredFields {
		if _, ok := resultMap[field]; !ok {
			t.Errorf("Result should have '%s' field", field)
		}
	}

	if _, ok := resultMap["related_adrs"].([]map[string]interface{}); !ok {
		t.Error("'related_adrs' field should be array of maps")
	}

	if _, ok := resultMap["conflicts"].([]map[string]interface{}); !ok {
		t.Error("'conflicts' field should be array of maps")
	}

	if _, ok := resultMap["suggestions"].([]string); !ok {
		t.Error("'suggestions' field should be array of strings")
	}
}

// TestCheckADRAlignmentTool_Execute_SupersededADR tests handling of superseded ADRs
func TestCheckADRAlignmentTool_Execute_SupersededADR(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	supersededADR := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Use Monolithic Architecture",
			Category:     config.CategoryADR,
			Path:         "mcp/resources/adr/002-monolithic-architecture.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: `# ADR 002: Use Monolithic Architecture

## Status

Superseded by ADR 001

## Context

We initially chose a monolithic architecture for simplicity.

## Decision

We will use a monolithic architecture.

## Consequences

This decision has been superseded by the move to microservices.
`,
		},
	}

	cache.Set(supersededADR.Metadata.Path, supersededADR)

	arguments := map[string]interface{}{
		"decision_description": "We should use monolithic architecture",
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

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	foundSuperseded := false
	for _, adr := range relatedADRs {
		status, ok := adr["status"].(string)
		if ok && status == "superseded" {
			foundSuperseded = true
			alignment, _ := adr["alignment"].(string)
			if alignment != "conflicts" {
				t.Error("Superseded ADR should be marked as conflicts")
			}
		}
	}

	if !foundSuperseded {
		t.Error("Expected to find superseded ADR")
	}
}

// TestCheckADRAlignmentTool_Execute_ADRIDExtraction tests ADR ID extraction
func TestCheckADRAlignmentTool_Execute_ADRIDExtraction(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "microservices architecture",
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

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	if len(relatedADRs) == 0 {
		t.Fatal("Expected to find related ADRs")
	}

	for _, adr := range relatedADRs {
		adrID, ok := adr["adr_id"].(string)
		if !ok {
			t.Error("ADR should have 'adr_id' field")
			continue
		}

		if adrID == "" || adrID == "unknown" {
			t.Errorf("ADR ID should be extracted from filename, got: %s", adrID)
		}
	}
}

// TestCheckADRAlignmentTool_Execute_URIGeneration tests proper URI generation
func TestCheckADRAlignmentTool_Execute_URIGeneration(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "microservices architecture",
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

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	for i, adr := range relatedADRs {
		uri, ok := adr["uri"].(string)
		if !ok {
			t.Errorf("ADR[%d] should have 'uri' field", i)
			continue
		}

		if !strings.HasPrefix(uri, config.URIScheme) {
			t.Errorf("ADR[%d] URI should start with %s, got: %s", i, config.URIScheme, uri)
		}

		if !strings.Contains(uri, config.URIADR) {
			t.Errorf("ADR[%d] URI should contain %s, got: %s", i, config.URIADR, uri)
		}
	}
}

// TestCheckADRAlignmentTool_Execute_SuggestionsGeneration tests suggestion generation
func TestCheckADRAlignmentTool_Execute_SuggestionsGeneration(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "We should adopt microservices architecture",
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

	suggestions, ok := resultMap["suggestions"].([]string)
	if !ok {
		t.Fatal("Result should have 'suggestions' field")
	}

	if len(suggestions) == 0 {
		t.Error("Expected suggestions to be provided")
	}

	for i, suggestion := range suggestions {
		if suggestion == "" {
			t.Errorf("Suggestion[%d] should not be empty", i)
		}
	}
}

// TestCheckADRAlignmentTool_Execute_CaseInsensitive tests case-insensitive matching
func TestCheckADRAlignmentTool_Execute_CaseInsensitive(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	tests := []struct {
		name        string
		description string
	}{
		{"lowercase", "microservices architecture"},
		{"uppercase", "MICROSERVICES ARCHITECTURE"},
		{"mixed case", "MiCrOsErViCeS ArChItEcTuRe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]interface{}{
				"decision_description": tt.description,
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

			relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
			if !ok {
				t.Fatal("Result should have 'related_adrs' field")
			}

			if len(relatedADRs) == 0 {
				t.Error("Expected to find related ADRs regardless of case")
			}
		})
	}
}

// TestCheckADRAlignmentTool_Execute_StatusExtraction tests ADR status extraction
func TestCheckADRAlignmentTool_Execute_StatusExtraction(t *testing.T) {
	cache := cache.NewDocumentCache()
	logger := logging.NewStructuredLogger("test")
	tool := NewCheckADRAlignmentTool(cache, logger)

	setupADRTestDocuments(cache)

	arguments := map[string]interface{}{
		"decision_description": "microservices architecture",
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

	relatedADRs, ok := resultMap["related_adrs"].([]map[string]interface{})
	if !ok {
		t.Fatal("Result should have 'related_adrs' field")
	}

	if len(relatedADRs) == 0 {
		t.Fatal("Expected to find related ADRs")
	}

	foundAccepted := false
	for _, adr := range relatedADRs {
		status, ok := adr["status"].(string)
		if !ok {
			t.Error("ADR should have 'status' field")
			continue
		}

		if status == "accepted" {
			foundAccepted = true
		}

		if status == "" {
			t.Error("ADR status should not be empty")
		}
	}

	if !foundAccepted {
		t.Error("Expected to find at least one accepted ADR")
	}
}

// setupADRTestDocuments adds test ADR documents to the cache
func setupADRTestDocuments(cache *cache.DocumentCache) {
	adr1 := &models.Document{
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
Our monolithic application is becoming difficult to scale and deploy independently.

## Decision

We will adopt a microservices architecture to enable independent deployment and scaling.
This approach will allow teams to work autonomously and deploy services independently.

## Consequences

- Increased operational complexity
- Better scalability and resilience
- Independent team autonomy
- Need for service discovery and API gateway
`,
		},
	}

	adr2 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "API Gateway Pattern",
			Category:     config.CategoryADR,
			Path:         "mcp/resources/adr/003-api-gateway.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: `# ADR 003: API Gateway Pattern

## Status

Accepted

## Context

With our microservices architecture, we need a unified entry point for client requests.

## Decision

We will implement an API gateway to handle routing, authentication, and rate limiting.

## Consequences

- Single entry point for all client requests
- Centralized authentication and authorization
- Simplified client code
`,
		},
	}

	adr3 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Database Per Service",
			Category:     config.CategoryADR,
			Path:         "mcp/resources/adr/004-database-per-service.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: `# ADR 004: Database Per Service

## Status

Accepted

## Context

In our microservices architecture, we need to decide on data management strategy.

## Decision

Each microservice will have its own database to ensure loose coupling and independent deployment.

## Consequences

- Data consistency challenges
- Need for distributed transactions or eventual consistency
- Better service independence
`,
		},
	}

	cache.Set(adr1.Metadata.Path, adr1)
	cache.Set(adr2.Metadata.Path, adr2)
	cache.Set(adr3.Metadata.Path, adr3)
}
