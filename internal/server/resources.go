package server

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/errors"
)

// createMCPResourceFromDocument converts a Document to an MCPResource
func (s *MCPServer) createMCPResourceFromDocument(doc *models.Document) models.MCPResource {
	// Generate MCP resource URI based on category
	uri := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)

	// Create description from title and category
	description := fmt.Sprintf("%s document", strings.Title(doc.Metadata.Category))
	if doc.Metadata.Title != "" {
		description = fmt.Sprintf("%s: %s", strings.Title(doc.Metadata.Category), doc.Metadata.Title)
	}

	// Create annotations with metadata
	annotations := map[string]string{
		"category":     doc.Metadata.Category,
		"path":         doc.Metadata.Path,
		"lastModified": doc.Metadata.LastModified.Format(time.RFC3339),
		"size":         fmt.Sprintf("%d", doc.Metadata.Size),
		"checksum":     doc.Metadata.Checksum,
	}

	return models.MCPResource{
		URI:         uri,
		Name:        doc.Metadata.Title,
		Description: description,
		MimeType:    config.MimeTypeMarkdown,
		Annotations: annotations,
	}
}

// generateResourceURI creates an MCP resource URI based on category and path
// Normalizes filesystem paths to consistent URI format for MCP protocol
func (s *MCPServer) generateResourceURI(category, path string) string {
	cleanPath := strings.TrimSuffix(path, config.MarkdownExtension)
	cleanPath = filepath.ToSlash(cleanPath)

	switch category {
	case config.CategoryGuideline:
		cleanPath = strings.TrimPrefix(cleanPath, config.GuidelinesPath+"/")
		return fmt.Sprintf("%s%s/%s", config.URIScheme, config.URIGuidelines, cleanPath)
	case config.CategoryPattern:
		cleanPath = strings.TrimPrefix(cleanPath, config.PatternsPath+"/")
		return fmt.Sprintf("%s%s/%s", config.URIScheme, config.URIPatterns, cleanPath)
	case config.CategoryADR:
		cleanPath = strings.TrimPrefix(cleanPath, config.ADRPath+"/")
		// ADRs use numeric IDs for cleaner URIs (e.g., "001" instead of "001-api-design")
		adrId := s.extractADRId(cleanPath)
		return fmt.Sprintf("%s%s/%s", config.URIScheme, config.URIADR, adrId)
	default:
		return fmt.Sprintf("%s%s/%s", config.URIScheme, config.URIUnknown, cleanPath)
	}
}

// extractADRId extracts ADR ID from filename or path
// Supports multiple ADR naming conventions for flexibility
func (s *MCPServer) extractADRId(path string) string {
	filename := filepath.Base(path)

	patterns := []string{
		`^(\d+)-`,    // "001-api-design"
		`^adr-(\d+)`, // "adr-001"
		`^ADR-(\d+)`, // "ADR-001"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(filename); len(matches) > 1 {
			return matches[1]
		}
	}

	return filename
}

// parseResourceURI parses an MCP resource URI and returns category and path
func (s *MCPServer) parseResourceURI(uri string) (category, path string, err error) {
	if !strings.HasPrefix(uri, config.URIScheme) {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid resource URI", nil).
			WithContext("uri", uri)
	}

	remainder := strings.TrimPrefix(uri, config.URIScheme)

	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) < 2 {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			config.URIFormatError, nil).
			WithContext("uri", uri)
	}

	category = parts[0]
	path = parts[1]

	if path == "" || strings.HasPrefix(path, "/") {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid URI format, path cannot be empty or start with '/'", nil).
			WithContext("uri", uri).
			WithContext("path", path)
	}

	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return "", "", errors.NewValidationError(errors.ErrCodePathTraversal,
			"Path traversal attempt detected", nil).
			WithContext("uri", uri).
			WithContext("path", path)
	}

	switch category {
	case config.URIGuidelines:
		return config.CategoryGuideline, path, nil
	case config.URIPatterns:
		return config.CategoryPattern, path, nil
	case config.URIADR:
		return config.CategoryADR, path, nil
	default:
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidCategory,
			"unsupported resource category", nil).
			WithContext("uri", uri).
			WithContext("category", category)
	}
}

// findDocumentByResourcePath finds a document in the cache by category and resource path
// Uses two-phase lookup: URI matching first, then filesystem path fallback
func (s *MCPServer) findDocumentByResourcePath(category, resourcePath string) (*models.Document, error) {
	documents := s.cache.GetByCategory(category)

	if len(documents) == 0 {
		return nil, errors.NewFileSystemError(errors.ErrCodeFileNotFound,
			"Resource not found", nil).
			WithContext("category", category).
			WithContext("resourcePath", resourcePath)
	}

	// Phase 1: Match by generated URI (handles normalized paths)
	for _, doc := range documents {
		docResourceURI := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)

		_, docResourcePath, err := s.parseResourceURI(docResourceURI)
		if err != nil {
			continue
		}

		if strings.EqualFold(docResourcePath, resourcePath) {
			return doc, nil
		}
	}

	// Phase 2: Try direct filesystem path lookup (handles edge cases)
	possiblePaths := s.generatePossibleFilePaths(category, resourcePath)

	for _, possiblePath := range possiblePaths {
		if doc, err := s.cache.Get(possiblePath); err == nil {
			return doc, nil
		}
	}

	return nil, errors.NewFileSystemError(errors.ErrCodeFileNotFound,
		"Resource not found", nil).
		WithContext("category", category).
		WithContext("resourcePath", resourcePath)
}

// generatePossibleFilePaths generates possible file paths for a given category and resource path
// ADRs support multiple naming conventions to accommodate different team preferences
func (s *MCPServer) generatePossibleFilePaths(category, resourcePath string) []string {
	var paths []string

	if !strings.HasSuffix(resourcePath, config.MarkdownExtension) {
		resourcePath += config.MarkdownExtension
	}

	switch category {
	case config.CategoryGuideline:
		paths = append(paths, filepath.Join(config.GuidelinesPath, resourcePath))
	case config.CategoryPattern:
		paths = append(paths, filepath.Join(config.PatternsPath, resourcePath))
	case config.CategoryADR:
		adrId := strings.TrimSuffix(resourcePath, config.MarkdownExtension)

		patterns := []string{
			fmt.Sprintf("%s%s", adrId, config.MarkdownExtension),
			fmt.Sprintf("adr-%s%s", adrId, config.MarkdownExtension),
			fmt.Sprintf("ADR-%s%s", adrId, config.MarkdownExtension),
			fmt.Sprintf("%03s%s", adrId, config.MarkdownExtension),
		}

		for _, pattern := range patterns {
			paths = append(paths, filepath.Join(config.ADRPath, pattern))
		}

		// Fallback: search existing documents for partial ID matches
		allDocs := s.cache.GetByCategory(config.CategoryADR)
		for _, doc := range allDocs {
			docURI := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)
			if strings.Contains(docURI, adrId) {
				paths = append(paths, doc.Metadata.Path)
			}
		}
	}

	return paths
}
