package validation

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
)

// DocumentValidator provides validation utilities for documents
type DocumentValidator struct {
	markdown goldmark.Markdown
}

// NewDocumentValidator creates a new document validator
func NewDocumentValidator() *DocumentValidator {
	return &DocumentValidator{
		markdown: goldmark.New(),
	}
}

// ValidateAndExtractMetadata validates a document file and extracts metadata
func (dv *DocumentValidator) ValidateAndExtractMetadata(filePath string) (*models.DocumentMetadata, error) {
	// Sanitize the file path
	sanitizedPath, err := SanitizePath(filePath)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	// Check if file exists and get file info
	fileInfo, err := os.Stat(sanitizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	// Read file content
	content, err := os.ReadFile(sanitizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Validate Markdown structure
	if err := dv.ValidateMarkdownStructure(content); err != nil {
		return nil, fmt.Errorf("invalid markdown structure: %w", err)
	}

	// Extract metadata
	metadata := &models.DocumentMetadata{
		Path:         sanitizedPath,
		LastModified: fileInfo.ModTime(),
		Size:         fileInfo.Size(),
		Checksum:     calculateChecksum(content),
	}

	// Extract title from content
	title, err := dv.ExtractTitle(content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract title: %w", err)
	}
	metadata.Title = title

	// Determine category based on path
	category, err := DetermineCategoryFromPath(sanitizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine category: %w", err)
	}
	metadata.Category = category

	return metadata, nil
}

// ValidateMarkdownStructure validates the structure of a Markdown document
func (dv *DocumentValidator) ValidateMarkdownStructure(content []byte) error {
	doc := dv.markdown.Parser().Parse(text.NewReader(content))

	// Check if document has at least one heading
	hasHeading := false
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			hasHeading = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})

	if !hasHeading {
		return fmt.Errorf("document must contain at least one heading")
	}

	// Validate heading hierarchy (no skipping levels)
	var headingLevels []int
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			headingLevels = append(headingLevels, heading.Level)
		}
		return ast.WalkContinue, nil
	})

	if err := validateHeadingHierarchy(headingLevels); err != nil {
		return fmt.Errorf("invalid heading hierarchy: %w", err)
	}

	return nil
}

// ExtractTitle extracts the title from a Markdown document (first H1 heading)
func (dv *DocumentValidator) ExtractTitle(content []byte) (string, error) {
	doc := dv.markdown.Parser().Parse(text.NewReader(content))

	var title string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			if heading.Level == 1 {
				// Extract text content from heading
				var buf strings.Builder
				for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
					if child.Kind() == ast.KindText {
						text := child.(*ast.Text)
						buf.Write(text.Segment.Value(content))
					}
				}
				title = strings.TrimSpace(buf.String())
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	})

	if title == "" {
		return "", fmt.Errorf("no H1 heading found in document")
	}

	return title, nil
}

// ParseMarkdownSections parses a Markdown document into structured sections
func (dv *DocumentValidator) ParseMarkdownSections(content []byte) ([]models.DocumentSection, error) {
	doc := dv.markdown.Parser().Parse(text.NewReader(content))

	var sections []models.DocumentSection
	var currentSection *models.DocumentSection
	var sectionStack []*models.DocumentSection

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)

			// Extract heading text
			var buf strings.Builder
			for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
				if child.Kind() == ast.KindText {
					text := child.(*ast.Text)
					buf.Write(text.Segment.Value(content))
				}
			}

			section := models.DocumentSection{
				Heading: strings.TrimSpace(buf.String()),
				Level:   heading.Level,
				Content: "",
			}

			// Handle section hierarchy
			if heading.Level == 1 {
				sections = append(sections, section)
				currentSection = &sections[len(sections)-1]
				sectionStack = []*models.DocumentSection{currentSection}
			} else {
				// Find the appropriate parent section
				for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= heading.Level {
					sectionStack = sectionStack[:len(sectionStack)-1]
				}

				if len(sectionStack) > 0 {
					parent := sectionStack[len(sectionStack)-1]
					parent.Subsections = append(parent.Subsections, section)
					currentSection = &parent.Subsections[len(parent.Subsections)-1]
					sectionStack = append(sectionStack, currentSection)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	return sections, nil
}

// SanitizePath sanitizes a file path to prevent directory traversal attacks
func SanitizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Check for directory traversal attempts before cleaning
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains directory traversal sequence")
	}

	// Ensure path doesn't start with / (absolute path)
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	// Clean the path to resolve any . components
	cleanPath := filepath.Clean(path)

	// Check for null bytes
	if strings.Contains(cleanPath, "\x00") {
		return "", fmt.Errorf("path contains null bytes")
	}

	// Validate allowed characters (alphanumeric, dash, underscore, dot, slash)
	validPath := regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	if !validPath.MatchString(cleanPath) {
		return "", fmt.Errorf("path contains invalid characters")
	}

	return cleanPath, nil
}

// ValidateResourceURI validates an MCP resource URI and extracts the path component
func ValidateResourceURI(uri string) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("URI cannot be empty")
	}

	// Define valid URI patterns
	validPatterns := []string{
		`^architecture://guidelines/(.+)$`,
		`^architecture://patterns/(.+)$`,
		`^architecture://adr/(.+)$`,
	}

	for _, pattern := range validPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(uri)
		if len(matches) == 2 {
			// Extract and sanitize the path component
			return SanitizePath(matches[1])
		}
	}

	return "", fmt.Errorf("invalid resource URI format")
}

// DetermineCategoryFromPath determines the document category based on file path
func DetermineCategoryFromPath(path string) (string, error) {
	cleanPath := filepath.Clean(path)

	if strings.HasPrefix(cleanPath, config.GuidelinesPath+"/") {
		return config.CategoryGuideline, nil
	}
	if strings.HasPrefix(cleanPath, config.PatternsPath+"/") {
		return config.CategoryPattern, nil
	}
	if strings.HasPrefix(cleanPath, config.ADRPath+"/") {
		return config.CategoryADR, nil
	}

	return "", fmt.Errorf("unable to determine category from path: %s", path)
}

// validateHeadingHierarchy ensures headings follow proper hierarchy (no skipping levels)
func validateHeadingHierarchy(levels []int) error {
	if len(levels) == 0 {
		return nil
	}

	for i := 1; i < len(levels); i++ {
		prevLevel := levels[i-1]
		currentLevel := levels[i]

		// Allow same level, one level deeper, or any level shallower
		if currentLevel > prevLevel+1 {
			return fmt.Errorf("heading level %d follows level %d, skipping intermediate levels", currentLevel, prevLevel)
		}
	}

	return nil
}

// calculateChecksum calculates MD5 checksum of content
func calculateChecksum(content []byte) string {
	hash := md5.Sum(content)
	return fmt.Sprintf("%x", hash)
}
