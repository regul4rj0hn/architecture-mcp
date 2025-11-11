package prompts

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
)

func TestRenderTemplate(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()
	renderer := NewTemplateRenderer(cache)

	tests := []struct {
		name     string
		template string
		args     map[string]interface{}
		want     string
	}{
		{
			name:     "simple variable substitution",
			template: "Hello {{name}}!",
			args:     map[string]interface{}{"name": "World"},
			want:     "Hello World!",
		},
		{
			name:     "multiple variables",
			template: "{{greeting}} {{name}}, welcome to {{place}}!",
			args: map[string]interface{}{
				"greeting": "Hello",
				"name":     "Alice",
				"place":    "Wonderland",
			},
			want: "Hello Alice, welcome to Wonderland!",
		},
		{
			name:     "repeated variable",
			template: "{{word}} {{word}} {{word}}",
			args:     map[string]interface{}{"word": "test"},
			want:     "test test test",
		},
		{
			name:     "no variables",
			template: "This is a plain template",
			args:     map[string]interface{}{},
			want:     "This is a plain template",
		},
		{
			name:     "variable with hyphens",
			template: "Language: {{programming-language}}",
			args:     map[string]interface{}{"programming-language": "Go"},
			want:     "Language: Go",
		},
		{
			name:     "variable with underscores",
			template: "Value: {{some_value}}",
			args:     map[string]interface{}{"some_value": "123"},
			want:     "Value: 123",
		},
		{
			name:     "numeric value",
			template: "Count: {{count}}",
			args:     map[string]interface{}{"count": 42},
			want:     "Count: 42",
		},
		{
			name:     "missing variable leaves placeholder",
			template: "Hello {{name}}, you have {{count}} messages",
			args:     map[string]interface{}{"name": "Bob"},
			want:     "Hello Bob, you have {{count}} messages",
		},
		{
			name:     "code block with variable",
			template: "```{{language}}\n{{code}}\n```",
			args: map[string]interface{}{
				"language": "go",
				"code":     "func main() {}",
			},
			want: "```go\nfunc main() {}\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderer.RenderTemplate(tt.template, tt.args)
			if err != nil {
				t.Errorf("RenderTemplate() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("RenderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmbedResources(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	// Populate cache with test documents
	doc1 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "API Design Guidelines",
			Category:     "guidelines",
			Path:         config.GuidelinesPath + "/api-design.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: "# API Design\n\nUse RESTful principles.",
		},
	}

	doc2 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Repository Pattern",
			Category:     "patterns",
			Path:         config.PatternsPath + "/repository-pattern.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: "# Repository Pattern\n\nSeparate data access logic.",
		},
	}

	doc3 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        "Factory Pattern",
			Category:     "patterns",
			Path:         config.PatternsPath + "/factory-pattern.md",
			LastModified: time.Now(),
		},
		Content: models.DocumentContent{
			RawContent: "# Factory Pattern\n\nCreate objects without specifying exact class.",
		},
	}

	cache.Set(config.GuidelinesPath+"/api-design.md", doc1)
	cache.Set(config.PatternsPath+"/repository-pattern.md", doc2)
	cache.Set(config.PatternsPath+"/factory-pattern.md", doc3)

	renderer := NewTemplateRenderer(cache)

	tests := []struct {
		name     string
		template string
		wantErr  bool
		validate func(*testing.T, string)
	}{
		{
			name:     "embed single resource",
			template: "Guidelines:\n\n{{resource:architecture://guidelines/api-design}}",
			wantErr:  false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "API Design Guidelines") {
					t.Error("Result should contain document title")
				}
				if !strings.Contains(result, "Use RESTful principles") {
					t.Error("Result should contain document content")
				}
				if !strings.Contains(result, config.GuidelinesPath+"/api-design.md") {
					t.Error("Result should contain source path")
				}
			},
		},
		{
			name:     "embed all patterns with wildcard",
			template: "Patterns:\n\n{{resource:architecture://patterns/*}}",
			wantErr:  false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "Repository Pattern") {
					t.Error("Result should contain Repository Pattern")
				}
				if !strings.Contains(result, "Factory Pattern") {
					t.Error("Result should contain Factory Pattern")
				}
				if !strings.Contains(result, "---") {
					t.Error("Result should contain separator between documents")
				}
			},
		},
		{
			name:     "no resource patterns",
			template: "This is a plain template without resources",
			wantErr:  false,
			validate: func(t *testing.T, result string) {
				if result != "This is a plain template without resources" {
					t.Errorf("Result should be unchanged, got: %s", result)
				}
			},
		},
		{
			name:     "invalid resource URI scheme",
			template: "{{resource:http://invalid/path}}",
			wantErr:  true,
		},
		{
			name:     "resource not found",
			template: "{{resource:architecture://guidelines/nonexistent}}",
			wantErr:  true,
		},
		{
			name:     "wildcard with no matches",
			template: "{{resource:architecture://nonexistent/*}}",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderer.EmbedResources(tt.template)
			if tt.wantErr {
				if err == nil {
					t.Error("EmbedResources() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("EmbedResources() unexpected error = %v", err)
				return
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestEmbedResourcesSizeLimits(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	// Test resource count limit
	t.Run("resource count limit", func(t *testing.T) {
		// Add more than MaxResourcesPerPrompt documents with unique paths
		for i := 0; i < MaxResourcesPerPrompt+5; i++ {
			doc := &models.Document{
				Metadata: models.DocumentMetadata{
					Title:    "Test Document",
					Category: "patterns",
					Path:     config.PatternsPath + "/test.md",
				},
				Content: models.DocumentContent{
					RawContent: "Small content",
				},
			}
			// Use unique path for each document
			path := fmt.Sprintf(config.PatternsPath+"/test-%d.md", i)
			doc.Metadata.Path = path
			cache.Set(path, doc)
		}

		renderer := NewTemplateRenderer(cache)
		template := "{{resource:architecture://patterns/*}}"

		_, err := renderer.EmbedResources(template)
		if err == nil {
			t.Error("EmbedResources() expected error for resource count limit, got nil")
		}
		if !strings.Contains(err.Error(), "resource limit exceeded") {
			t.Errorf("Expected 'resource limit exceeded' error, got: %v", err)
		}
	})

	// Test content size limit
	t.Run("content size limit", func(t *testing.T) {
		cache.Clear()

		// Create a document with content exceeding MaxTotalContentSize
		largeContent := strings.Repeat("x", MaxTotalContentSize+1000)
		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:    "Large Document",
				Category: "patterns",
				Path:     config.PatternsPath + "/large.md",
			},
			Content: models.DocumentContent{
				RawContent: largeContent,
			},
		}
		cache.Set(config.PatternsPath+"/large.md", doc)

		renderer := NewTemplateRenderer(cache)
		template := "{{resource:architecture://patterns/*}}"

		_, err := renderer.EmbedResources(template)
		if err == nil {
			t.Error("EmbedResources() expected error for content size limit, got nil")
		}
		if !strings.Contains(err.Error(), "content size limit exceeded") {
			t.Errorf("Expected 'content size limit exceeded' error, got: %v", err)
		}
	})
}

func TestResolveResourcePattern(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	// Populate cache with test documents
	doc1 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:    "API Design",
			Category: "guidelines",
			Path:     config.GuidelinesPath + "/api-design.md",
		},
		Content: models.DocumentContent{
			RawContent: "API content",
		},
	}

	doc2 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:    "Repository Pattern",
			Category: "patterns",
			Path:     config.PatternsPath + "/repository-pattern.md",
		},
		Content: models.DocumentContent{
			RawContent: "Repository content",
		},
	}

	doc3 := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:    "ADR 001",
			Category: "adr",
			Path:     config.ADRPath + "/001-microservices-architecture.md",
		},
		Content: models.DocumentContent{
			RawContent: "ADR content",
		},
	}

	cache.Set(config.GuidelinesPath+"/api-design.md", doc1)
	cache.Set(config.PatternsPath+"/repository-pattern.md", doc2)
	cache.Set(config.ADRPath+"/001-microservices-architecture.md", doc3)

	renderer := NewTemplateRenderer(cache)

	tests := []struct {
		name      string
		pattern   string
		wantErr   bool
		wantCount int
		validate  func(*testing.T, []*models.Document)
	}{
		{
			name:      "exact match",
			pattern:   "architecture://guidelines/api-design",
			wantErr:   false,
			wantCount: 1,
			validate: func(t *testing.T, docs []*models.Document) {
				if docs[0].Metadata.Title != "API Design" {
					t.Errorf("Expected 'API Design', got '%s'", docs[0].Metadata.Title)
				}
			},
		},
		{
			name:      "wildcard all in category",
			pattern:   "architecture://patterns/*",
			wantErr:   false,
			wantCount: 1,
			validate: func(t *testing.T, docs []*models.Document) {
				if docs[0].Metadata.Category != "patterns" {
					t.Errorf("Expected category 'patterns', got '%s'", docs[0].Metadata.Category)
				}
			},
		},
		{
			name:      "wildcard all ADRs",
			pattern:   "architecture://adr/*",
			wantErr:   false,
			wantCount: 1,
			validate: func(t *testing.T, docs []*models.Document) {
				if docs[0].Metadata.Category != "adr" {
					t.Errorf("Expected category 'adr', got '%s'", docs[0].Metadata.Category)
				}
			},
		},
		{
			name:    "invalid URI scheme",
			pattern: "http://invalid/path",
			wantErr: true,
		},
		{
			name:    "missing category",
			pattern: "architecture://",
			wantErr: true,
		},
		{
			name:    "nonexistent resource",
			pattern: "architecture://guidelines/nonexistent",
			wantErr: true,
		},
		{
			name:    "nonexistent category",
			pattern: "architecture://invalid/*",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs, err := renderer.ResolveResourcePattern(tt.pattern)
			if tt.wantErr {
				if err == nil {
					t.Error("ResolveResourcePattern() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveResourcePattern() unexpected error = %v", err)
				return
			}
			if tt.wantCount > 0 && len(docs) != tt.wantCount {
				t.Errorf("ResolveResourcePattern() returned %d documents, want %d", len(docs), tt.wantCount)
			}
			if tt.validate != nil {
				tt.validate(t, docs)
			}
		})
	}
}

func TestCombinedRenderingAndEmbedding(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	// Add test document
	doc := &models.Document{
		Metadata: models.DocumentMetadata{
			Title:    "Test Pattern",
			Category: "patterns",
			Path:     config.PatternsPath + "/test-pattern.md",
		},
		Content: models.DocumentContent{
			RawContent: "Pattern implementation details",
		},
	}
	cache.Set(config.PatternsPath+"/test-pattern.md", doc)

	renderer := NewTemplateRenderer(cache)

	// Template with both variables and resources
	template := `Review the following {{language}} code:

` + "```{{language}}\n{{code}}\n```" + `

Compare against our patterns:

{{resource:architecture://patterns/*}}`

	args := map[string]interface{}{
		"language": "go",
		"code":     "func main() { fmt.Println(\"Hello\") }",
	}

	// First render variables
	rendered, err := renderer.RenderTemplate(template, args)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	// Then embed resources
	final, err := renderer.EmbedResources(rendered)
	if err != nil {
		t.Fatalf("EmbedResources() error = %v", err)
	}

	// Validate final result
	if !strings.Contains(final, "go") {
		t.Error("Final result should contain language variable")
	}
	if !strings.Contains(final, "func main()") {
		t.Error("Final result should contain code variable")
	}
	if !strings.Contains(final, "Test Pattern") {
		t.Error("Final result should contain embedded resource")
	}
	if !strings.Contains(final, "Pattern implementation details") {
		t.Error("Final result should contain resource content")
	}
}

func TestNewTemplateRenderer(t *testing.T) {
	cache := cache.NewDocumentCache()
	defer cache.Close()

	renderer := NewTemplateRenderer(cache)

	if renderer == nil {
		t.Fatal("NewTemplateRenderer() returned nil")
	}

	if renderer.cache != cache {
		t.Error("NewTemplateRenderer() did not set cache correctly")
	}
}
