package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/config"
	"mcp-architecture-service/pkg/logging"
)

// CheckADRAlignmentTool checks if a decision aligns with existing ADRs
type CheckADRAlignmentTool struct {
	cache  *cache.DocumentCache
	logger *logging.StructuredLogger
}

// NewCheckADRAlignmentTool creates a new CheckADRAlignmentTool instance
func NewCheckADRAlignmentTool(cache *cache.DocumentCache, logger *logging.StructuredLogger) *CheckADRAlignmentTool {
	return &CheckADRAlignmentTool{
		cache:  cache,
		logger: logger,
	}
}

// Name returns the unique identifier for the tool
func (cat *CheckADRAlignmentTool) Name() string {
	return "check-adr-alignment"
}

// Description returns a human-readable description
func (cat *CheckADRAlignmentTool) Description() string {
	return "Checks if an architectural decision aligns with existing ADRs to identify conflicts or redundancies"
}

// InputSchema returns JSON schema for tool parameters
func (cat *CheckADRAlignmentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"decision_description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the proposed decision",
				"maxLength":   5000,
			},
			"decision_context": map[string]interface{}{
				"type":        "string",
				"description": "Context or problem being addressed (optional)",
				"maxLength":   2000,
			},
		},
		"required": []string{"decision_description"},
	}
}

// Execute runs the tool with validated arguments
func (cat *CheckADRAlignmentTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	// Extract arguments
	decisionDescription, ok := arguments["decision_description"].(string)
	if !ok {
		return nil, fmt.Errorf("decision_description argument must be a string")
	}

	// Validate decision_description length
	if len(decisionDescription) > 5000 {
		return nil, fmt.Errorf("decision_description exceeds maximum length of 5000 characters")
	}

	// Extract optional decision_context
	decisionContext := ""
	if dc, ok := arguments["decision_context"].(string); ok {
		decisionContext = dc
		// Validate decision_context length
		if len(decisionContext) > 2000 {
			return nil, fmt.Errorf("decision_context exceeds maximum length of 2000 characters")
		}
	}

	cat.logger.WithContext("decision_length", len(decisionDescription)).
		WithContext("has_context", decisionContext != "").
		Info("Checking ADR alignment")

	// Perform alignment analysis
	result := cat.analyzeAlignment(decisionDescription, decisionContext)

	return result, nil
}

// adrAlignment represents alignment information for a single ADR
type adrAlignment struct {
	URI       string
	Title     string
	ADRID     string
	Status    string
	Alignment string // "supports", "conflicts", "related"
	Reason    string
	Score     float64
}

// analyzeAlignment performs the ADR alignment analysis
func (cat *CheckADRAlignmentTool) analyzeAlignment(decisionDescription, decisionContext string) map[string]interface{} {
	// Extract keywords from decision description and context
	keywords := cat.extractKeywords(decisionDescription, decisionContext)

	// Get all ADR documents from cache
	allDocs := cat.cache.GetAllDocuments()
	var adrDocs []struct {
		path string
		doc  *models.Document
	}
	for path, doc := range allDocs {
		if doc.Metadata.Category == config.CategoryADR {
			adrDocs = append(adrDocs, struct {
				path string
				doc  *models.Document
			}{path, doc})
		}
	}

	// Analyze each ADR for alignment
	var alignments []adrAlignment
	for _, adrDoc := range adrDocs {
		alignment := cat.analyzeADR(adrDoc.doc, adrDoc.path, keywords, decisionDescription)
		if alignment != nil {
			alignments = append(alignments, *alignment)
		}
	}

	// Sort by score (descending)
	cat.sortAlignments(alignments)

	// Identify conflicts
	conflicts := cat.identifyConflicts(alignments, decisionDescription)

	// Generate suggestions
	suggestions := cat.generateSuggestions(alignments, conflicts)

	// Convert to output format
	relatedADRs := make([]map[string]interface{}, 0, len(alignments))
	for _, alignment := range alignments {
		relatedADRs = append(relatedADRs, map[string]interface{}{
			"uri":       alignment.URI,
			"title":     alignment.Title,
			"adr_id":    alignment.ADRID,
			"status":    alignment.Status,
			"alignment": alignment.Alignment,
			"reason":    alignment.Reason,
		})
	}

	conflictList := make([]map[string]interface{}, 0, len(conflicts))
	for _, conflict := range conflicts {
		conflictList = append(conflictList, map[string]interface{}{
			"adr_uri":              conflict.URI,
			"conflict_description": conflict.Description,
		})
	}

	return map[string]interface{}{
		"related_adrs": relatedADRs,
		"conflicts":    conflictList,
		"suggestions":  suggestions,
	}
}

// extractKeywords extracts important keywords from decision text
func (cat *CheckADRAlignmentTool) extractKeywords(decisionDescription, decisionContext string) []string {
	text := cat.combineText(decisionDescription, decisionContext)
	tokens := cat.tokenizeText(text)
	return cat.filterKeywords(tokens)
}

func (cat *CheckADRAlignmentTool) combineText(description, context string) string {
	text := description
	if context != "" {
		text = text + " " + context
	}
	return strings.ToLower(text)
}

func (cat *CheckADRAlignmentTool) tokenizeText(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '.' ||
			r == ';' || r == ':' || r == '!' || r == '?' || r == '(' || r == ')'
	})
}

func (cat *CheckADRAlignmentTool) filterKeywords(tokens []string) []string {
	stopWords := cat.getStopWords()
	keywordSet := make(map[string]bool)
	var keywords []string

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if cat.isValidKeyword(token, stopWords, keywordSet) {
			keywords = append(keywords, token)
			keywordSet[token] = true
		}
	}

	return keywords
}

func (cat *CheckADRAlignmentTool) getStopWords() map[string]bool {
	return map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
		"are": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "should": true, "could": true, "may": true,
		"might": true, "must": true, "can": true, "this": true, "that": true,
		"these": true, "those": true, "we": true, "our": true, "us": true,
	}
}

func (cat *CheckADRAlignmentTool) isValidKeyword(token string, stopWords, keywordSet map[string]bool) bool {
	return len(token) >= 3 && !stopWords[token] && !keywordSet[token]
}

// analyzeADR analyzes a single ADR for alignment with the decision
func (cat *CheckADRAlignmentTool) analyzeADR(doc *models.Document, path string, keywords []string, decisionDescription string) *adrAlignment {
	content := doc.Content.RawContent
	contentLower := strings.ToLower(content)
	decisionLower := strings.ToLower(decisionDescription)

	// Calculate relevance score based on keyword matches
	score := 0.0
	matchedKeywords := 0
	for _, keyword := range keywords {
		if strings.Contains(contentLower, keyword) {
			matchedKeywords++
			// Count occurrences
			count := strings.Count(contentLower, keyword)
			score += float64(count)
		}
	}

	// Only consider ADRs with at least some keyword matches
	if matchedKeywords == 0 {
		return nil
	}

	// Boost score for title matches
	titleLower := strings.ToLower(doc.Metadata.Title)
	for _, keyword := range keywords {
		if strings.Contains(titleLower, keyword) {
			score += 5.0
		}
	}

	// Extract ADR ID from path
	adrID := cat.extractADRID(path)

	// Extract ADR status
	status := cat.extractADRStatus(content)

	// Determine alignment type
	alignment, reason := cat.determineAlignment(content, decisionLower, status, keywords)

	// Generate URI
	uri := cat.generateURI(path)

	return &adrAlignment{
		URI:       uri,
		Title:     doc.Metadata.Title,
		ADRID:     adrID,
		Status:    status,
		Alignment: alignment,
		Reason:    reason,
		Score:     score,
	}
}

// extractADRID extracts the ADR ID from the file path
func (cat *CheckADRAlignmentTool) extractADRID(path string) string {
	// Extract filename from path
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	// Extract ID (e.g., "001" from "001-microservices-architecture.md")
	re := regexp.MustCompile(`^(\d+)-`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) > 1 {
		return matches[1]
	}

	return "unknown"
}

// extractADRStatus extracts the status from ADR content
func (cat *CheckADRAlignmentTool) extractADRStatus(content string) string {
	// Look for status in common ADR formats
	statusPatterns := []string{
		`(?i)status:\s*(\w+)`,
		`(?i)## status\s+(\w+)`,
		`(?i)\*\*status\*\*:\s*(\w+)`,
	}

	for _, pattern := range statusPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return strings.ToLower(matches[1])
		}
	}

	// Default to "unknown" if status not found
	return "unknown"
}

// determineAlignment determines the alignment type and reason
func (cat *CheckADRAlignmentTool) determineAlignment(adrContent, decisionLower string, status string, keywords []string) (string, string) {
	adrContentLower := strings.ToLower(adrContent)

	// Check for conflicts first
	if alignment, reason := cat.checkForConflicts(adrContentLower, status); alignment != "" {
		return alignment, reason
	}

	// Check for opposing patterns
	if alignment, reason := cat.checkOpposingPatterns(adrContentLower, keywords); alignment != "" {
		return alignment, reason
	}

	// Check for decision alignment
	if alignment, reason := cat.checkDecisionAlignment(adrContent, status, keywords); alignment != "" {
		return alignment, reason
	}

	// Check for supporting patterns
	if cat.hasSupportingPatterns(adrContentLower) {
		return "supports", "ADR provides relevant guidance for this decision"
	}

	return "related", "ADR discusses related architectural concerns"
}

func (cat *CheckADRAlignmentTool) checkForConflicts(adrContentLower, status string) (string, string) {
	conflictKeywords := []string{"deprecated", "superseded", "rejected", "obsolete"}
	for _, keyword := range conflictKeywords {
		if strings.Contains(adrContentLower, keyword) && status != "accepted" {
			return "conflicts", fmt.Sprintf("ADR is %s and may conflict with new decisions", status)
		}
	}

	if status == "superseded" || status == "deprecated" {
		return "conflicts", "ADR has been superseded or deprecated"
	}

	return "", ""
}

func (cat *CheckADRAlignmentTool) checkOpposingPatterns(adrContentLower string, keywords []string) (string, string) {
	opposingKeywords := []string{"avoid", "do not", "should not", "must not", "anti-pattern"}

	for _, keyword := range opposingKeywords {
		if !strings.Contains(adrContentLower, keyword) {
			continue
		}

		if cat.hasNearbyOpposingKeywords(adrContentLower, keywords, opposingKeywords) {
			return "conflicts", "ADR recommends avoiding this approach"
		}
	}

	return "", ""
}

func (cat *CheckADRAlignmentTool) hasNearbyOpposingKeywords(adrContentLower string, keywords, opposingKeywords []string) bool {
	for _, decisionKeyword := range keywords {
		keywordPos := strings.Index(adrContentLower, decisionKeyword)
		if keywordPos == -1 {
			continue
		}

		for _, opposing := range opposingKeywords {
			opposingPos := strings.Index(adrContentLower, opposing)
			if opposingPos != -1 && abs(keywordPos-opposingPos) < 100 {
				return true
			}
		}
	}
	return false
}

func (cat *CheckADRAlignmentTool) checkDecisionAlignment(adrContent, status string, keywords []string) (string, string) {
	decisionSection := cat.extractSection(adrContent, "## Decision")
	if decisionSection == "" {
		return "", ""
	}

	decisionSectionLower := strings.ToLower(decisionSection)
	matchCount := 0
	for _, keyword := range keywords {
		if strings.Contains(decisionSectionLower, keyword) {
			matchCount++
		}
	}

	if matchCount >= len(keywords)/2 && len(keywords) > 0 {
		if status == "accepted" {
			return "supports", "ADR decision aligns with proposed approach"
		}
		return "related", "ADR addresses similar concerns"
	}

	return "", ""
}

func (cat *CheckADRAlignmentTool) hasSupportingPatterns(adrContentLower string) bool {
	supportKeywords := []string{"recommend", "should use", "best practice", "guideline"}
	for _, keyword := range supportKeywords {
		if strings.Contains(adrContentLower, keyword) {
			return true
		}
	}
	return false
}

// extractSection extracts content from a markdown section
func (cat *CheckADRAlignmentTool) extractSection(content, sectionHeader string) string {
	lines := strings.Split(content, "\n")
	inSection := false
	sectionContent := []string{}

	for _, line := range lines {
		if strings.HasPrefix(line, sectionHeader) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if inSection {
			sectionContent = append(sectionContent, line)
		}
	}

	return strings.Join(sectionContent, "\n")
}

// sortAlignments sorts alignments by score in descending order
func (cat *CheckADRAlignmentTool) sortAlignments(alignments []adrAlignment) {
	// Simple bubble sort for small lists
	for i := 0; i < len(alignments); i++ {
		for j := i + 1; j < len(alignments); j++ {
			if alignments[j].Score > alignments[i].Score {
				alignments[i], alignments[j] = alignments[j], alignments[i]
			}
		}
	}
}

// conflict represents a potential conflict with an ADR
type conflict struct {
	URI         string
	Description string
}

// identifyConflicts identifies potential conflicts from alignments
func (cat *CheckADRAlignmentTool) identifyConflicts(alignments []adrAlignment, _decisionDescription string) []conflict {
	var conflicts []conflict

	for _, alignment := range alignments {
		if alignment.Alignment == "conflicts" {
			conflicts = append(conflicts, conflict{
				URI:         alignment.URI,
				Description: fmt.Sprintf("%s: %s", alignment.Title, alignment.Reason),
			})
		}
	}

	return conflicts
}

// generateSuggestions creates actionable suggestions based on analysis
func (cat *CheckADRAlignmentTool) generateSuggestions(alignments []adrAlignment, conflicts []conflict) []string {
	var suggestions []string

	// Suggest reviewing conflicting ADRs
	if len(conflicts) > 0 {
		suggestions = append(suggestions, "Review conflicting ADRs to understand why previous decisions were made differently")
		suggestions = append(suggestions, "Consider updating or superseding conflicting ADRs if the new decision is justified")
	}

	// Suggest referencing supporting ADRs
	supportingCount := 0
	for _, alignment := range alignments {
		if alignment.Alignment == "supports" {
			supportingCount++
		}
	}
	if supportingCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Reference %d supporting ADR(s) in your decision documentation", supportingCount))
	}

	// Suggest reviewing related ADRs
	relatedCount := 0
	for _, alignment := range alignments {
		if alignment.Alignment == "related" {
			relatedCount++
		}
	}
	if relatedCount > 0 {
		suggestions = append(suggestions, "Review related ADRs to ensure consistency with existing architectural direction")
	}

	// General suggestions
	if len(alignments) == 0 {
		suggestions = append(suggestions, "No related ADRs found - this may be a new architectural area")
		suggestions = append(suggestions, "Consider creating a new ADR to document this decision")
	} else {
		suggestions = append(suggestions, "Document how this decision relates to existing ADRs")
	}

	return suggestions
}

// generateURI creates a proper architecture:// URI for an ADR
func (cat *CheckADRAlignmentTool) generateURI(path string) string {
	// Extract filename from path
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	// Remove .md extension
	filename = strings.TrimSuffix(filename, config.MarkdownExtension)

	// Generate URI
	return fmt.Sprintf("%s%s/%s", config.URIScheme, config.URIADR, filename)
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
