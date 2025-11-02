package scanner

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mcp-architecture-service/internal/models"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// DocumentationScanner handles scanning and parsing of documentation files
type DocumentationScanner struct {
	rootPath string
	parser   goldmark.Markdown
}

// NewDocumentationScanner creates a new documentation scanner
func NewDocumentationScanner(rootPath string) *DocumentationScanner {
	return &DocumentationScanner{
		rootPath: rootPath,
		parser:   goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID())),
	}
}

// ScanDirectory recursively scans a directory for documentation files
func (ds *DocumentationScanner) ScanDirectory(path string) (*models.DocumentIndex, error) {
	// Validate input path
	if path == "" {
		return nil, fmt.Errorf("scan path cannot be empty")
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", path)
	}

	var documents []models.DocumentMetadata
	var parseErrors []string
	category := ds.getCategoryFromPath(path)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			// Log but continue processing
			parseErrors = append(parseErrors, fmt.Sprintf("access error for %s: %v", filePath, err))
			return nil
		}

		// Skip directories and non-markdown files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		metadata, err := ds.ParseMarkdownFile(filePath)
		if err != nil {
			// Collect parse errors but continue processing
			parseErrors = append(parseErrors, fmt.Sprintf("parse error for %s: %v", filePath, err))
			return nil
		}

		metadata.Category = category
		documents = append(documents, *metadata)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %w", path, err)
	}

	// Log parse errors if any occurred
	if len(parseErrors) > 0 {
		fmt.Printf("Warning: %d files had parsing errors:\n", len(parseErrors))
		for _, errMsg := range parseErrors {
			fmt.Printf("  - %s\n", errMsg)
		}
	}

	return &models.DocumentIndex{
		Category:  category,
		Documents: documents,
		Count:     len(documents),
		Errors:    parseErrors,
	}, nil
}

// ParseMarkdownFile parses a markdown file and extracts metadata using goldmark
func (ds *DocumentationScanner) ParseMarkdownFile(filePath string) (*models.DocumentMetadata, error) {
	// Validate file path
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Validate that content is not empty
	if len(content) == 0 {
		return nil, fmt.Errorf("file %s is empty", filePath)
	}

	// Validate that content appears to be valid text
	if !ds.isValidMarkdown(content) {
		return nil, fmt.Errorf("file %s does not appear to contain valid markdown", filePath)
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum(content))

	// Parse markdown using goldmark to extract structured metadata
	metadata, err := ds.ExtractMetadata(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata from %s: %w", filePath, err)
	}

	// Get relative path from root
	relPath, err := filepath.Rel(ds.rootPath, filePath)
	if err != nil {
		relPath = filePath
	}

	// Use filename as fallback title if no title found
	if metadata.Title == "" {
		metadata.Title = strings.TrimSuffix(filepath.Base(filePath), ".md")
	}

	// Set file system metadata
	metadata.Path = relPath
	metadata.LastModified = info.ModTime()
	metadata.Size = info.Size()
	metadata.Checksum = checksum

	return metadata, nil
}

// ExtractMetadata extracts structured metadata from markdown content using goldmark
func (ds *DocumentationScanner) ExtractMetadata(content string) (*models.DocumentMetadata, error) {
	// Parse the markdown document
	source := []byte(content)
	doc := ds.parser.Parser().Parse(text.NewReader(source))

	metadata := &models.DocumentMetadata{}

	// Walk the AST to extract metadata
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Heading:
			// Extract title from first H1 heading
			if n.Level == 1 && metadata.Title == "" {
				title := ds.extractTextFromNode(n, source)
				metadata.Title = strings.TrimSpace(title)
			}
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown AST: %w", err)
	}

	return metadata, nil
}

// extractTextFromNode extracts text content from an AST node
func (ds *DocumentationScanner) extractTextFromNode(node ast.Node, source []byte) string {
	var buf bytes.Buffer

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if text, ok := child.(*ast.Text); ok {
			buf.Write(text.Segment.Value(source))
		}
	}

	return buf.String()
}

// isValidMarkdown performs basic validation to check if content appears to be markdown
func (ds *DocumentationScanner) isValidMarkdown(content []byte) bool {
	// Empty content is considered valid
	if len(content) == 0 {
		return true
	}

	// Check for null bytes or other binary indicators
	for _, b := range content {
		if b == 0 {
			return false
		}
	}

	// Convert to string for text-based checks
	text := string(content)

	// Check for common markdown patterns or at least readable text
	markdownPatterns := []string{
		`^#\s+`,      // Headers
		`^\*\s+`,     // Bullet lists
		`^\d+\.\s+`,  // Numbered lists
		`\*\*.*\*\*`, // Bold text
		`_.*_`,       // Italic text
		"`.*`",       // Code spans
	}

	for _, pattern := range markdownPatterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}

	// If no markdown patterns found, check if it's at least readable text
	// Allow if it contains mostly printable characters
	printableCount := 0
	for _, r := range text {
		if r >= 32 && r <= 126 || r == '\n' || r == '\r' || r == '\t' {
			printableCount++
		}
	}

	// Consider valid if at least 90% of characters are printable
	return float64(printableCount)/float64(len(text)) >= 0.9
}

// BuildIndex scans multiple directories and builds a comprehensive index
func (ds *DocumentationScanner) BuildIndex(directories []string) (map[string]*models.DocumentIndex, error) {
	if len(directories) == 0 {
		return nil, fmt.Errorf("no directories provided for indexing")
	}

	indexes := make(map[string]*models.DocumentIndex)
	var allErrors []string

	for _, dir := range directories {
		index, err := ds.ScanDirectory(dir)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("failed to scan %s: %v", dir, err))
			continue
		}

		// Add any parsing errors to the overall error list
		if len(index.Errors) > 0 {
			allErrors = append(allErrors, index.Errors...)
		}

		indexes[index.Category] = index
	}

	// Log overall indexing results
	totalDocs := 0
	for category, index := range indexes {
		totalDocs += index.Count
		fmt.Printf("Indexed %d documents in category '%s'\n", index.Count, category)
	}

	if len(allErrors) > 0 {
		fmt.Printf("Warning: %d total errors occurred during indexing:\n", len(allErrors))
		for _, errMsg := range allErrors {
			fmt.Printf("  - %s\n", errMsg)
		}
	}

	fmt.Printf("Successfully built index with %d total documents across %d categories\n", totalDocs, len(indexes))

	return indexes, nil
}

// getCategoryFromPath determines the document category based on the file path
func (ds *DocumentationScanner) getCategoryFromPath(path string) string {
	// Normalize path separators for cross-platform compatibility
	normalizedPath := filepath.ToSlash(strings.ToLower(path))

	if strings.Contains(normalizedPath, "guidelines") {
		return "guideline"
	}
	if strings.Contains(normalizedPath, "patterns") {
		return "pattern"
	}
	if strings.Contains(normalizedPath, "adr") {
		return "adr"
	}
	return "unknown"
}
