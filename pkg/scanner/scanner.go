package scanner

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mcp-architecture-service/internal/models"
)

// DocumentationScanner handles scanning and parsing of documentation files
type DocumentationScanner struct {
	rootPath string
}

// NewDocumentationScanner creates a new documentation scanner
func NewDocumentationScanner(rootPath string) *DocumentationScanner {
	return &DocumentationScanner{
		rootPath: rootPath,
	}
}

// ScanDirectory recursively scans a directory for documentation files
func (ds *DocumentationScanner) ScanDirectory(path string) (*models.DocumentIndex, error) {
	var documents []models.DocumentMetadata
	category := ds.getCategoryFromPath(path)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		metadata, err := ds.ParseMarkdownFile(filePath)
		if err != nil {
			// Log error but continue processing other files
			fmt.Printf("Warning: Failed to parse %s: %v\n", filePath, err)
			return nil
		}

		metadata.Category = category
		documents = append(documents, *metadata)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %w", path, err)
	}

	return &models.DocumentIndex{
		Category:  category,
		Documents: documents,
	}, nil
}

// ParseMarkdownFile parses a markdown file and extracts metadata
func (ds *DocumentationScanner) ParseMarkdownFile(filePath string) (*models.DocumentMetadata, error) {
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

	// Read file content for checksum and title extraction
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum(content))

	// Extract title from content
	title := ds.ExtractTitle(string(content))
	if title == "" {
		// Use filename as fallback title
		title = strings.TrimSuffix(filepath.Base(filePath), ".md")
	}

	// Get relative path from root
	relPath, err := filepath.Rel(ds.rootPath, filePath)
	if err != nil {
		relPath = filePath
	}

	return &models.DocumentMetadata{
		Title:        title,
		Path:         relPath,
		LastModified: info.ModTime(),
		Size:         info.Size(),
		Checksum:     checksum,
	}, nil
}

// ExtractTitle extracts the title from markdown content (first H1 heading)
func (ds *DocumentationScanner) ExtractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	}
	return ""
}

// getCategoryFromPath determines the document category based on the file path
func (ds *DocumentationScanner) getCategoryFromPath(path string) string {
	if strings.Contains(path, "guidelines") {
		return "guideline"
	}
	if strings.Contains(path, "patterns") {
		return "pattern"
	}
	if strings.Contains(path, "adr") {
		return "adr"
	}
	return "unknown"
}
