package scanner

import (
	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDocumentationScanner(t *testing.T) {
	scanner := NewDocumentationScanner("/test/path")
	if scanner == nil {
		t.Fatal("Expected scanner to be created, got nil")
	}
	if scanner.rootPath != "/test/path" {
		t.Errorf("Expected rootPath to be '/test/path', got '%s'", scanner.rootPath)
	}
	if scanner.parser == nil {
		t.Fatal("Expected parser to be initialized, got nil")
	}
}

func TestGetCategoryFromPath(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	tests := []struct {
		path     string
		expected string
	}{
		{"/mcp/resources/guidelines/api.md", "guideline"},
		{"/mcp/resources/patterns/singleton.md", "pattern"},
		{"/mcp/resources/adr/001-database.md", "adr"},
		{"/mcp/resources/other/readme.md", "unknown"},
		{"guidelines/test.md", "guideline"},
		{"PATTERNS/Test.md", "pattern"}, // Test case insensitive
		{"", "unknown"},
	}

	for _, test := range tests {
		result := scanner.getCategoryFromPath(test.path)
		if result != test.expected {
			t.Errorf("getCategoryFromPath(%s) = %s, expected %s", test.path, result, test.expected)
		}
	}
}

func TestExtractMetadata(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Simple H1 title",
			content:  "# Test Title\n\nSome content here.",
			expected: "Test Title",
		},
		{
			name:     "H1 with extra spaces",
			content:  "#   Spaced Title   \n\nContent.",
			expected: "Spaced Title",
		},
		{
			name:     "No H1 heading",
			content:  "## H2 Title\n\nSome content.",
			expected: "",
		},
		{
			name:     "Multiple H1 headings",
			content:  "# First Title\n\n# Second Title\n\nContent.",
			expected: "First Title",
		},
		{
			name:     "Empty content",
			content:  "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := scanner.ExtractMetadata(test.content)
			if err != nil {
				t.Fatalf("ExtractMetadata failed: %v", err)
			}
			if metadata.Title != test.expected {
				t.Errorf("Expected title '%s', got '%s'", test.expected, metadata.Title)
			}
		})
	}
}

func TestIsValidMarkdown(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "Valid markdown with headers",
			content:  []byte("# Title\n\nSome content."),
			expected: true,
		},
		{
			name:     "Valid markdown with lists",
			content:  []byte("* Item 1\n* Item 2"),
			expected: true,
		},
		{
			name:     "Valid markdown with bold",
			content:  []byte("This is **bold** text."),
			expected: true,
		},
		{
			name:     "Plain text (valid)",
			content:  []byte("This is just plain text without markdown."),
			expected: true,
		},
		{
			name:     "Binary content (invalid)",
			content:  []byte{0x00, 0x01, 0x02, 0x03},
			expected: false,
		},
		{
			name:     "Empty content",
			content:  []byte(""),
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := scanner.isValidMarkdown(test.content)
			if result != test.expected {
				t.Errorf("isValidMarkdown() = %v, expected %v", result, test.expected)
			}
		})
	}
}

func TestScanDirectoryErrors(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	tests := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"non-existent directory", "/non/existent/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := scanner.ScanDirectory(tt.path)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestParseMarkdownFileErrors(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	tests := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"non-existent file", "/non/existent/file.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := scanner.ParseMarkdownFile(tt.path)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
		})
	}
}

// setupScanTestDirectories creates a temporary directory structure with test files
func setupScanTestDirectories(t *testing.T) (tempDir, guidelinesDir, patternsDir, adrDir string) {
	t.Helper()

	tempDir = t.TempDir()

	guidelinesDir = filepath.Join(tempDir, config.GuidelinesPath)
	patternsDir = filepath.Join(tempDir, config.PatternsPath)
	adrDir = filepath.Join(tempDir, config.ADRPath)

	dirs := []string{guidelinesDir, patternsDir, adrDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	testFiles := map[string]string{
		filepath.Join(guidelinesDir, "api.md"):     "# API Guidelines\n\nThis is a guideline.",
		filepath.Join(patternsDir, "singleton.md"): "# Singleton Pattern\n\nThis is a pattern.",
		filepath.Join(adrDir, "001-database.md"):   "# Database Choice\n\nThis is an ADR.",
	}

	for filePath, content := range testFiles {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	return tempDir, guidelinesDir, patternsDir, adrDir
}

// validateScannedDocuments validates the scanned documents in an index
func validateScannedDocuments(t *testing.T, index *models.DocumentIndex, expectedCategory string, expectedCount int, expectedTitle string) {
	t.Helper()

	if index.Category != expectedCategory {
		t.Errorf("Expected category '%s', got '%s'", expectedCategory, index.Category)
	}
	if index.Count != expectedCount {
		t.Errorf("Expected %d document(s), got %d", expectedCount, index.Count)
	}
	if len(index.Documents) != expectedCount {
		t.Errorf("Expected %d document(s) in slice, got %d", expectedCount, len(index.Documents))
	}

	if expectedCount > 0 && expectedTitle != "" {
		doc := index.Documents[0]
		if doc.Title != expectedTitle {
			t.Errorf("Expected title '%s', got '%s'", expectedTitle, doc.Title)
		}
		if doc.Category != expectedCategory {
			t.Errorf("Expected category '%s', got '%s'", expectedCategory, doc.Category)
		}
	}
}

// validateBuildIndex validates the results of BuildIndex
func validateBuildIndex(t *testing.T, indexes map[string]*models.DocumentIndex, expectedCategories []string) {
	t.Helper()

	if len(indexes) != len(expectedCategories) {
		t.Errorf("Expected %d categories, got %d", len(expectedCategories), len(indexes))
	}

	for _, category := range expectedCategories {
		if _, exists := indexes[category]; !exists {
			t.Errorf("Expected category '%s' to exist in indexes", category)
		}
	}
}

func TestScanDirectoryIntegration(t *testing.T) {
	tempDir, guidelinesDir, patternsDir, adrDir := setupScanTestDirectories(t)

	scanner := NewDocumentationScanner(tempDir)

	t.Run("scan single directory", func(t *testing.T) {
		index, err := scanner.ScanDirectory(guidelinesDir)
		if err != nil {
			t.Fatalf("Failed to scan guidelines directory: %v", err)
		}
		validateScannedDocuments(t, index, "guideline", 1, "API Guidelines")
	})

	t.Run("build index from multiple directories", func(t *testing.T) {
		directories := []string{guidelinesDir, patternsDir, adrDir}
		indexes, err := scanner.BuildIndex(directories)
		if err != nil {
			t.Fatalf("Failed to build index: %v", err)
		}

		expectedCategories := []string{"guideline", "pattern", "adr"}
		validateBuildIndex(t, indexes, expectedCategories)
	})
}

func TestParseMarkdownFileIntegration(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		filename      string
		content       string
		expectedTitle string
	}{
		{
			name:          "valid markdown with formatting",
			filename:      "test.md",
			content:       "# Test Document\n\nThis is test content with **bold** text.",
			expectedTitle: "Test Document",
		},
		{
			name:          "simple markdown",
			filename:      "simple.md",
			content:       "# Simple Title\n\nPlain content.",
			expectedTitle: "Simple Title",
		},
	}

	scanner := NewDocumentationScanner(tempDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, tt.filename)
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			metadata, err := scanner.ParseMarkdownFile(testFile)
			if err != nil {
				t.Fatalf("Failed to parse markdown file: %v", err)
			}

			if metadata.Title != tt.expectedTitle {
				t.Errorf("Expected title '%s', got '%s'", tt.expectedTitle, metadata.Title)
			}
			if metadata.Size != int64(len(tt.content)) {
				t.Errorf("Expected size %d, got %d", len(tt.content), metadata.Size)
			}
			if metadata.Checksum == "" {
				t.Error("Expected checksum to be calculated")
			}
			if metadata.Path == "" {
				t.Error("Expected path to be set")
			}
		})
	}
}

func TestBuildIndexErrors(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	tests := []struct {
		name        string
		directories []string
		expectError bool
		expectEmpty bool
	}{
		{
			name:        "empty directories list",
			directories: []string{},
			expectError: true,
			expectEmpty: false,
		},
		{
			name:        "non-existent directories",
			directories: []string{"/non/existent/path"},
			expectError: false,
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexes, err := scanner.BuildIndex(tt.directories)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectEmpty && len(indexes) != 0 {
				t.Errorf("Expected empty indexes, got %d", len(indexes))
			}
		})
	}
}

func TestScanDirectoryWithMalformedFiles(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := []struct {
		filename string
		content  []byte
	}{
		{"binary.md", []byte{0x00, 0x01, 0x02, 0x03, 0xFF}},
		{"empty.md", []byte{}},
		{"valid.md", []byte("# Valid Document\n\nContent here.")},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tempDir, tf.filename)
		if err := os.WriteFile(filePath, tf.content, 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.filename, err)
		}
	}

	scanner := NewDocumentationScanner(tempDir)
	index, err := scanner.ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	if index.Count != 1 {
		t.Errorf("Expected 1 valid document, got %d", index.Count)
	}
	if len(index.Errors) == 0 {
		t.Error("Expected some parsing errors for malformed files")
	}
}
