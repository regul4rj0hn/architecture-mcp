package scanner

import (
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
		{"/docs/guidelines/api.md", "guideline"},
		{"/docs/patterns/singleton.md", "pattern"},
		{"/docs/adr/001-database.md", "adr"},
		{"/docs/other/readme.md", "unknown"},
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

	// Test empty path
	_, err := scanner.ScanDirectory("")
	if err == nil {
		t.Error("Expected error for empty path, got nil")
	}

	// Test non-existent directory
	_, err = scanner.ScanDirectory("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestParseMarkdownFileErrors(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	// Test empty path
	_, err := scanner.ParseMarkdownFile("")
	if err == nil {
		t.Error("Expected error for empty file path, got nil")
	}

	// Test non-existent file
	_, err = scanner.ParseMarkdownFile("/non/existent/file.md")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

// Integration test with temporary files
func TestScanDirectoryIntegration(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "scanner_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directories
	guidelinesDir := filepath.Join(tempDir, "docs", "guidelines")
	patternsDir := filepath.Join(tempDir, "docs", "patterns")
	adrDir := filepath.Join(tempDir, "docs", "adr")

	err = os.MkdirAll(guidelinesDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create guidelines dir: %v", err)
	}
	err = os.MkdirAll(patternsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create patterns dir: %v", err)
	}
	err = os.MkdirAll(adrDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create adr dir: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		filepath.Join(guidelinesDir, "api.md"):     "# API Guidelines\n\nThis is a guideline.",
		filepath.Join(patternsDir, "singleton.md"): "# Singleton Pattern\n\nThis is a pattern.",
		filepath.Join(adrDir, "001-database.md"):   "# Database Choice\n\nThis is an ADR.",
	}

	for filePath, content := range testFiles {
		err = os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	scanner := NewDocumentationScanner(tempDir)

	// Test scanning guidelines directory
	index, err := scanner.ScanDirectory(guidelinesDir)
	if err != nil {
		t.Fatalf("Failed to scan guidelines directory: %v", err)
	}

	if index.Category != "guideline" {
		t.Errorf("Expected category 'guideline', got '%s'", index.Category)
	}
	if index.Count != 1 {
		t.Errorf("Expected 1 document, got %d", index.Count)
	}
	if len(index.Documents) != 1 {
		t.Errorf("Expected 1 document in slice, got %d", len(index.Documents))
	}

	doc := index.Documents[0]
	if doc.Title != "API Guidelines" {
		t.Errorf("Expected title 'API Guidelines', got '%s'", doc.Title)
	}
	if doc.Category != "guideline" {
		t.Errorf("Expected category 'guideline', got '%s'", doc.Category)
	}

	// Test BuildIndex with multiple directories
	directories := []string{guidelinesDir, patternsDir, adrDir}
	indexes, err := scanner.BuildIndex(directories)
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	if len(indexes) != 3 {
		t.Errorf("Expected 3 categories, got %d", len(indexes))
	}

	expectedCategories := []string{"guideline", "pattern", "adr"}
	for _, category := range expectedCategories {
		if _, exists := indexes[category]; !exists {
			t.Errorf("Expected category '%s' to exist in indexes", category)
		}
	}
}

func TestParseMarkdownFileIntegration(t *testing.T) {
	// Create temporary file
	tempDir, err := os.MkdirTemp("", "scanner_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test Document\n\nThis is test content with **bold** text."

	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	scanner := NewDocumentationScanner(tempDir)
	metadata, err := scanner.ParseMarkdownFile(testFile)
	if err != nil {
		t.Fatalf("Failed to parse markdown file: %v", err)
	}

	if metadata.Title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got '%s'", metadata.Title)
	}
	if metadata.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), metadata.Size)
	}
	if metadata.Checksum == "" {
		t.Error("Expected checksum to be calculated")
	}
	if metadata.Path == "" {
		t.Error("Expected path to be set")
	}
}

func TestBuildIndexErrors(t *testing.T) {
	scanner := NewDocumentationScanner("/test")

	// Test with empty directories list
	_, err := scanner.BuildIndex([]string{})
	if err == nil {
		t.Error("Expected error for empty directories list, got nil")
	}

	// Test with non-existent directories
	indexes, err := scanner.BuildIndex([]string{"/non/existent/path"})
	if err != nil {
		t.Errorf("BuildIndex should not fail completely, got error: %v", err)
	}
	if len(indexes) != 0 {
		t.Errorf("Expected empty indexes for non-existent directories, got %d", len(indexes))
	}
}

func TestScanDirectoryWithMalformedFiles(t *testing.T) {
	// Create temporary directory with malformed files
	tempDir, err := os.MkdirTemp("", "scanner_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a binary file with .md extension (should be rejected)
	binaryFile := filepath.Join(tempDir, "binary.md")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
	err = os.WriteFile(binaryFile, binaryContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	// Create an empty file
	emptyFile := filepath.Join(tempDir, "empty.md")
	err = os.WriteFile(emptyFile, []byte{}, 0644)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Create a valid file
	validFile := filepath.Join(tempDir, "valid.md")
	err = os.WriteFile(validFile, []byte("# Valid Document\n\nContent here."), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid file: %v", err)
	}

	scanner := NewDocumentationScanner(tempDir)
	index, err := scanner.ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Should have 1 valid document and some errors
	if index.Count != 1 {
		t.Errorf("Expected 1 valid document, got %d", index.Count)
	}
	if len(index.Errors) == 0 {
		t.Error("Expected some parsing errors for malformed files")
	}
}
