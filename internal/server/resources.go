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
		MimeType:    "text/markdown",
		Annotations: annotations,
	}
}

// generateResourceURI creates an MCP resource URI based on category and path
func (s *MCPServer) generateResourceURI(category, path string) string {
	// Remove file extension and normalize path
	cleanPath := strings.TrimSuffix(path, ".md")
	cleanPath = filepath.ToSlash(cleanPath)

	// Remove category prefix from path if present
	switch category {
	case "guideline":
		cleanPath = strings.TrimPrefix(cleanPath, config.GuidelinesPath+"/")
		return fmt.Sprintf("architecture://guidelines/%s", cleanPath)
	case "pattern":
		cleanPath = strings.TrimPrefix(cleanPath, config.PatternsPath+"/")
		return fmt.Sprintf("architecture://patterns/%s", cleanPath)
	case "adr":
		cleanPath = strings.TrimPrefix(cleanPath, config.ADRPath+"/")
		// For ADRs, extract ADR ID from filename if possible
		adrId := s.extractADRId(cleanPath)
		return fmt.Sprintf("architecture://adr/%s", adrId)
	default:
		return fmt.Sprintf("architecture://unknown/%s", cleanPath)
	}
}

// extractADRId extracts ADR ID from filename or path
func (s *MCPServer) extractADRId(path string) string {
	// Get the base filename
	filename := filepath.Base(path)

	// Try to extract ADR number from common patterns like "001-api-design" or "adr-001"
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

	// If no pattern matches, use the filename without extension
	return filename
}

// parseResourceURI parses an MCP resource URI and returns category and path
func (s *MCPServer) parseResourceURI(uri string) (category, path string, err error) {
	// Expected URI patterns:
	// architecture://guidelines/{path}
	// architecture://patterns/{path}
	// architecture://adr/{adr_id}

	if !strings.HasPrefix(uri, "architecture://") {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid resource URI", nil).
			WithContext("uri", uri)
	}

	// Remove the scheme prefix
	remainder := strings.TrimPrefix(uri, "architecture://")

	// Split into category and path
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) < 2 {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid URI format, expected 'architecture://{category}/{path}'", nil).
			WithContext("uri", uri)
	}

	category = parts[0]
	path = parts[1]

	// Validate path is not empty and doesn't start with slash
	if path == "" || strings.HasPrefix(path, "/") {
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidURI,
			"Invalid URI format, path cannot be empty or start with '/'", nil).
			WithContext("uri", uri).
			WithContext("path", path)
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return "", "", errors.NewValidationError(errors.ErrCodePathTraversal,
			"Path traversal attempt detected", nil).
			WithContext("uri", uri).
			WithContext("path", path)
	}

	// Validate category
	switch category {
	case "guidelines":
		return "guideline", path, nil
	case "patterns":
		return "pattern", path, nil
	case "adr":
		return "adr", path, nil
	default:
		return "", "", errors.NewValidationError(errors.ErrCodeInvalidCategory,
			"unsupported resource category", nil).
			WithContext("uri", uri).
			WithContext("category", category)
	}
}

// findDocumentByResourcePath finds a document in the cache by category and resource path
func (s *MCPServer) findDocumentByResourcePath(category, resourcePath string) (*models.Document, error) {
	documents := s.cache.GetByCategory(category)

	// Check if we have documents for this category
	if len(documents) == 0 {
		return nil, errors.NewFileSystemError(errors.ErrCodeFileNotFound,
			"Resource not found", nil).
			WithContext("category", category).
			WithContext("resourcePath", resourcePath)
	}

	// For each document, generate its resource URI and compare with the requested path
	for _, doc := range documents {
		docResourceURI := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)

		// Extract the path part from the generated URI for comparison
		_, docResourcePath, err := s.parseResourceURI(docResourceURI)
		if err != nil {
			continue // Skip malformed URIs
		}

		// Compare paths (case-insensitive)
		if strings.EqualFold(docResourcePath, resourcePath) {
			return doc, nil
		}
	}

	// If no exact match found, try direct path lookup in cache
	// This handles cases where the resource path might be a direct file path
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
func (s *MCPServer) generatePossibleFilePaths(category, resourcePath string) []string {
	var paths []string

	// Add .md extension if not present
	if !strings.HasSuffix(resourcePath, ".md") {
		resourcePath += ".md"
	}

	switch category {
	case "guideline":
		paths = append(paths, filepath.Join(config.GuidelinesPath, resourcePath))
	case "pattern":
		paths = append(paths, filepath.Join(config.PatternsPath, resourcePath))
	case "adr":
		// For ADRs, try different naming patterns
		adrId := strings.TrimSuffix(resourcePath, ".md")

		// Try various ADR naming patterns
		patterns := []string{
			fmt.Sprintf("%s.md", adrId),
			fmt.Sprintf("adr-%s.md", adrId),
			fmt.Sprintf("ADR-%s.md", adrId),
			fmt.Sprintf("%03s.md", adrId), // Zero-padded numbers
		}

		for _, pattern := range patterns {
			paths = append(paths, filepath.Join(config.ADRPath, pattern))
		}

		// Also try to find by ADR ID in existing documents
		allDocs := s.cache.GetByCategory("adr")
		for _, doc := range allDocs {
			docURI := s.generateResourceURI(doc.Metadata.Category, doc.Metadata.Path)
			if strings.Contains(docURI, adrId) {
				paths = append(paths, doc.Metadata.Path)
			}
		}
	}

	return paths
}
