package validation

import (
	"mcp-architecture-service/pkg/config"
	"os"
	"testing"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "valid relative path",
			input:       config.GuidelinesPath + "/api-design.md",
			expected:    config.GuidelinesPath + "/api-design.md",
			expectError: false,
		},
		{
			name:        "path with dots",
			input:       config.ResourcesBasePath + "/../guidelines/api-design.md",
			expected:    "",
			expectError: true,
		},
		{
			name:        "absolute path",
			input:       "/etc/passwd",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty path",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "path with null bytes",
			input:       config.ResourcesBasePath + "/test\x00.md",
			expected:    "",
			expectError: true,
		},
		{
			name:        "path with invalid characters",
			input:       config.ResourcesBasePath + "/test<script>.md",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizePath(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateResourceURI(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "valid guidelines URI",
			input:       "architecture://guidelines/api-design.md",
			expected:    "api-design.md",
			expectError: false,
		},
		{
			name:        "valid patterns URI",
			input:       "architecture://patterns/microservices.md",
			expected:    "microservices.md",
			expectError: false,
		},
		{
			name:        "valid ADR URI",
			input:       "architecture://adr/001-use-microservices.md",
			expected:    "001-use-microservices.md",
			expectError: false,
		},
		{
			name:        "invalid URI scheme",
			input:       "http://example.com/doc.md",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty URI",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "URI with directory traversal",
			input:       "architecture://guidelines/../../../etc/passwd",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateResourceURI(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestDetermineCategoryFromPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "guidelines path",
			input:       config.GuidelinesPath + "/api-design.md",
			expected:    config.CategoryGuideline,
			expectError: false,
		},
		{
			name:        "patterns path",
			input:       config.PatternsPath + "/microservices.md",
			expected:    config.CategoryPattern,
			expectError: false,
		},
		{
			name:        "ADR path",
			input:       config.ADRPath + "/001-use-microservices.md",
			expected:    config.CategoryADR,
			expectError: false,
		},
		{
			name:        "invalid path",
			input:       "src/main.go",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DetermineCategoryFromPath(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateMarkdownStructure(t *testing.T) {
	validator := NewDocumentValidator()

	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name: "valid markdown with proper hierarchy",
			content: `# Main Title

## Section 1

### Subsection 1.1

## Section 2`,
			expectError: false,
		},
		{
			name: "markdown without headings",
			content: `This is just plain text
with no headings at all.`,
			expectError: true,
		},
		{
			name: "markdown with skipped heading levels",
			content: `# Main Title

### Subsection (skipped H2)`,
			expectError: true,
		},
		{
			name: "markdown with valid single heading",
			content: `# Single Heading

Some content here.`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMarkdownStructure([]byte(tt.content))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	validator := NewDocumentValidator()

	tests := []struct {
		name        string
		content     string
		expected    string
		expectError bool
	}{
		{
			name: "markdown with H1 title",
			content: `# API Design Guidelines

This document describes...`,
			expected:    "API Design Guidelines",
			expectError: false,
		},
		{
			name: "markdown without H1",
			content: `## Section Title

Some content here.`,
			expected:    "",
			expectError: true,
		},
		{
			name: "markdown with multiple H1s",
			content: `# First Title

# Second Title`,
			expected:    "First Title",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ExtractTitle([]byte(tt.content))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateAndExtractMetadata(t *testing.T) {
	validator := NewDocumentValidator()

	// Create a temporary test file in current directory
	testFile := config.GuidelinesPath + "/test.md"

	// Create directory structure
	err := os.MkdirAll(config.GuidelinesPath, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Clean up after test
	defer os.RemoveAll("docs")

	// Write test content
	testContent := `# Test Document

This is a test document for validation.

## Section 1

Some content here.`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test metadata extraction
	metadata, err := validator.ValidateAndExtractMetadata(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metadata.Title != "Test Document" {
		t.Errorf("expected title 'Test Document', got %q", metadata.Title)
	}

	if metadata.Category != config.CategoryGuideline {
		t.Errorf("expected category %q, got %q", config.CategoryGuideline, metadata.Category)
	}

	if metadata.Size == 0 {
		t.Errorf("expected non-zero file size")
	}

	if metadata.Checksum == "" {
		t.Errorf("expected non-empty checksum")
	}

	if metadata.LastModified.IsZero() {
		t.Errorf("expected non-zero last modified time")
	}
}
