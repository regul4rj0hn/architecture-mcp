package scanner

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"

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

// ScanDirectory recursively scans a directory for documentation files using concurrent processing
func (ds *DocumentationScanner) ScanDirectory(path string) (*models.DocumentIndex, error) {
	// Validate input path
	if path == "" {
		return nil, errors.NewValidationError(errors.ErrCodeInvalidParams,
			"Scan path cannot be empty", nil)
	}

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, errors.NewFileSystemError(errors.ErrCodeDirectoryNotFound,
			"Directory does not exist", err).
			WithContext("path", path)
	}

	category := ds.getCategoryFromPath(path)

	// Use concurrent scanning for better performance
	return ds.scanDirectoryConcurrent(path, category)
}

// scanDirectoryConcurrent performs concurrent file scanning for improved performance
func (ds *DocumentationScanner) scanDirectoryConcurrent(path, category string) (*models.DocumentIndex, error) {
	// First, collect all markdown files
	var markdownFiles []string
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue processing, errors will be handled during parsing
		}

		// Skip directories and non-markdown files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		markdownFiles = append(markdownFiles, filePath)
		return nil
	})

	if err != nil {
		return nil, errors.NewFileSystemError(errors.ErrCodeFileSystemUnavailable,
			"Failed to scan directory", err).
			WithContext("path", path)
	}

	if len(markdownFiles) == 0 {
		return &models.DocumentIndex{
			Category:  category,
			Documents: []models.DocumentMetadata{},
			Count:     0,
			Errors:    []string{},
		}, nil
	}

	// Process files concurrently with worker pool
	numWorkers := ds.calculateOptimalWorkerCount(len(markdownFiles))

	// Channels for work distribution and result collection
	fileChan := make(chan string, len(markdownFiles))
	resultChan := make(chan parseResult, len(markdownFiles))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go ds.parseWorker(&wg, fileChan, resultChan, category)
	}

	// Send files to workers
	for _, file := range markdownFiles {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var documents []models.DocumentMetadata
	var parseErrors []string

	for result := range resultChan {
		if result.err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("parse error for %s: %v", result.filePath, result.err))
		} else {
			documents = append(documents, result.metadata)
		}
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

// parseResult holds the result of parsing a single file
type parseResult struct {
	filePath string
	metadata models.DocumentMetadata
	err      error
}

// parseWorker processes files from the work channel
func (ds *DocumentationScanner) parseWorker(wg *sync.WaitGroup, fileChan <-chan string, resultChan chan<- parseResult, category string) {
	defer wg.Done()

	for filePath := range fileChan {
		metadata, err := ds.ParseMarkdownFile(filePath)

		result := parseResult{
			filePath: filePath,
			err:      err,
		}

		if err == nil {
			metadata.Category = category
			result.metadata = *metadata
		}

		resultChan <- result
	}
}

// calculateOptimalWorkerCount determines the optimal number of workers based on file count and system resources
func (ds *DocumentationScanner) calculateOptimalWorkerCount(fileCount int) int {
	// Get number of CPU cores
	numCPU := runtime.NumCPU()

	// For small file counts, use fewer workers to avoid overhead
	if fileCount <= 10 {
		return min(2, numCPU)
	}

	// For larger file counts, use more workers but cap at reasonable limits
	if fileCount <= 100 {
		return min(4, numCPU)
	}

	// For very large file counts, use more workers up to CPU count
	return min(numCPU, 8) // Cap at 8 to avoid excessive goroutine overhead
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseMarkdownFile parses a markdown file and extracts metadata using goldmark
func (ds *DocumentationScanner) ParseMarkdownFile(filePath string) (*models.DocumentMetadata, error) {
	// Validate file path
	if filePath == "" {
		return nil, errors.NewValidationError(errors.ErrCodeInvalidParams,
			"File path cannot be empty", nil)
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NewFileSystemError(errors.ErrCodeFileNotFound,
				"File not found", err).WithContext("path", filePath)
		}
		return nil, errors.NewFileSystemError(errors.ErrCodePermissionDenied,
			"Failed to open file", err).WithContext("path", filePath)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return nil, errors.NewFileSystemError(errors.ErrCodeFileSystemUnavailable,
			"Failed to get file info", err).WithContext("path", filePath)
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.NewFileSystemError(errors.ErrCodeFileSystemUnavailable,
			"Failed to read file", err).WithContext("path", filePath)
	}

	// Validate that content is not empty
	if len(content) == 0 {
		return nil, errors.NewParsingError(errors.ErrCodeMalformedMarkdown,
			"File is empty", nil).WithContext("path", filePath)
	}

	// Validate that content appears to be valid text
	if !ds.isValidMarkdown(content) {
		return nil, errors.NewParsingError(errors.ErrCodeEncodingIssue,
			"File does not appear to contain valid markdown", nil).
			WithContext("path", filePath)
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum(content))

	// Parse markdown using goldmark to extract structured metadata
	metadata, err := ds.ExtractMetadata(string(content))
	if err != nil {
		return nil, errors.NewParsingError(errors.ErrCodeInvalidMetadata,
			"Failed to extract metadata", err).WithContext("path", filePath)
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
		return nil, errors.NewParsingError(errors.ErrCodeMalformedMarkdown,
			"Failed to parse markdown AST", err)
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

// BuildIndex scans multiple directories concurrently and builds a comprehensive index
func (ds *DocumentationScanner) BuildIndex(directories []string) (map[string]*models.DocumentIndex, error) {
	if len(directories) == 0 {
		return nil, errors.NewValidationError(errors.ErrCodeInvalidParams,
			"No directories provided for indexing", nil)
	}

	// Use concurrent scanning for multiple directories
	return ds.buildIndexConcurrent(directories)
}

// buildIndexConcurrent processes multiple directories concurrently for faster indexing
func (ds *DocumentationScanner) buildIndexConcurrent(directories []string) (map[string]*models.DocumentIndex, error) {
	type indexResult struct {
		index *models.DocumentIndex
		err   error
	}

	// Channel for collecting results
	resultChan := make(chan indexResult, len(directories))

	// Start goroutines for each directory
	var wg sync.WaitGroup
	for _, dir := range directories {
		wg.Add(1)
		go func(directory string) {
			defer wg.Done()

			index, err := ds.ScanDirectory(directory)
			resultChan <- indexResult{
				index: index,
				err:   err,
			}
		}(dir)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	indexes := make(map[string]*models.DocumentIndex)
	var allErrors []string

	for result := range resultChan {
		if result.err != nil {
			allErrors = append(allErrors, fmt.Sprintf("failed to scan directory: %v", result.err))
			continue
		}

		// Add any parsing errors to the overall error list
		if len(result.index.Errors) > 0 {
			allErrors = append(allErrors, result.index.Errors...)
		}

		indexes[result.index.Category] = result.index
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
