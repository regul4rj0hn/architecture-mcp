package tools

import (
	"context"
	"fmt"
	"strings"

	"mcp-architecture-service/pkg/cache"
	"mcp-architecture-service/pkg/logging"
)

// ValidatePatternTool validates code against documented architectural patterns
type ValidatePatternTool struct {
	cache  *cache.DocumentCache
	logger *logging.StructuredLogger
}

// NewValidatePatternTool creates a new ValidatePatternTool instance
func NewValidatePatternTool(cache *cache.DocumentCache, logger *logging.StructuredLogger) *ValidatePatternTool {
	return &ValidatePatternTool{
		cache:  cache,
		logger: logger,
	}
}

// Name returns the unique identifier for the tool
func (vpt *ValidatePatternTool) Name() string {
	return "validate-against-pattern"
}

// Description returns a human-readable description
func (vpt *ValidatePatternTool) Description() string {
	return "Validates code against documented architectural patterns to ensure compliance with established guidelines"
}

// InputSchema returns JSON schema for tool parameters
func (vpt *ValidatePatternTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"code": map[string]interface{}{
				"type":        "string",
				"description": "Code to validate",
				"maxLength":   50000,
			},
			"pattern_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of pattern to validate against (e.g., 'repository-pattern')",
			},
			"language": map[string]interface{}{
				"type":        "string",
				"description": "Programming language (optional)",
			},
		},
		"required": []string{"code", "pattern_name"},
	}
}

// Execute runs the tool with validated arguments
func (vpt *ValidatePatternTool) Execute(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	// Extract arguments
	code, ok := arguments["code"].(string)
	if !ok {
		return nil, fmt.Errorf("code argument must be a string")
	}

	patternName, ok := arguments["pattern_name"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern_name argument must be a string")
	}

	language, _ := arguments["language"].(string)

	// Validate code length
	if len(code) > 50000 {
		return nil, fmt.Errorf("code exceeds maximum length of 50000 characters")
	}

	// Construct and validate pattern path
	patternPath := fmt.Sprintf("mcp/resources/patterns/%s.md", patternName)

	// Validate path to prevent directory traversal
	if err := ValidateResourcePath(patternPath); err != nil {
		return nil, fmt.Errorf("invalid pattern path: %w", err)
	}

	// Load pattern document from cache
	patternDoc, err := vpt.cache.Get(patternPath)
	if err != nil {
		return nil, fmt.Errorf("pattern not found: %s", patternName)
	}

	vpt.logger.WithContext("pattern", patternName).
		WithContext("language", language).
		WithContext("code_length", len(code)).
		Info("Validating code against pattern")

	// Perform validation
	result := vpt.validateCode(code, patternDoc.Content.RawContent, patternName, language)

	return result, nil
}

// validateCode performs the actual validation logic
func (vpt *ValidatePatternTool) validateCode(code, patternContent, patternName, language string) map[string]interface{} {
	violations := []map[string]interface{}{}
	suggestions := []string{}

	// Parse pattern document for validation rules
	rules := vpt.extractValidationRules(patternContent)

	// Analyze code structure against pattern expectations
	for _, rule := range rules {
		if violation := vpt.checkRule(code, rule, language); violation != nil {
			violations = append(violations, violation)
		}
	}

	// Generate suggestions based on violations
	if len(violations) > 0 {
		suggestions = vpt.generateSuggestions(violations, patternContent)
	}

	// Determine compliance status
	compliant := len(violations) == 0

	return map[string]interface{}{
		"compliant":   compliant,
		"pattern":     patternName,
		"violations":  violations,
		"suggestions": suggestions,
	}
}

// validationRule represents a rule extracted from the pattern document
type validationRule struct {
	name        string
	description string
	keywords    []string
	severity    string
}

// extractValidationRules parses the pattern document to extract validation rules
func (vpt *ValidatePatternTool) extractValidationRules(patternContent string) []validationRule {
	rules := []validationRule{}

	// Extract rules from "Best Practices" section
	if strings.Contains(patternContent, "## Best Practices") {
		bestPractices := vpt.extractSection(patternContent, "## Best Practices")
		rules = append(rules, vpt.parseBestPractices(bestPractices)...)
	}

	// Extract rules from "Common Pitfalls" section
	if strings.Contains(patternContent, "## Common Pitfalls") {
		pitfalls := vpt.extractSection(patternContent, "## Common Pitfalls")
		rules = append(rules, vpt.parsePitfalls(pitfalls)...)
	}

	// Extract rules from "Implementation" section
	if strings.Contains(patternContent, "## Implementation") {
		implementation := vpt.extractSection(patternContent, "## Implementation")
		rules = append(rules, vpt.parseImplementation(implementation)...)
	}

	return rules
}

// extractSection extracts content between a section header and the next section
func (vpt *ValidatePatternTool) extractSection(content, sectionHeader string) string {
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

// parseBestPractices extracts validation rules from best practices section
func (vpt *ValidatePatternTool) parseBestPractices(content string) []validationRule {
	rules := []validationRule{}
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "### ") {
			ruleName := strings.TrimPrefix(line, "### ")
			description := ""
			keywords := []string{}

			// Look ahead for description
			if i+1 < len(lines) {
				for j := i + 1; j < len(lines) && j < i+5; j++ {
					nextLine := strings.TrimSpace(lines[j])
					if strings.HasPrefix(nextLine, "- ") {
						keywords = append(keywords, strings.TrimPrefix(nextLine, "- "))
					} else if nextLine != "" && !strings.HasPrefix(nextLine, "###") {
						if description == "" {
							description = nextLine
						}
					}
				}
			}

			rules = append(rules, validationRule{
				name:        ruleName,
				description: description,
				keywords:    keywords,
				severity:    "warning",
			})
		}
	}

	return rules
}

// parsePitfalls extracts validation rules from common pitfalls section
func (vpt *ValidatePatternTool) parsePitfalls(content string) []validationRule {
	rules := []validationRule{}
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "### ") {
			ruleName := strings.TrimPrefix(line, "### ")
			description := ""
			keywords := []string{}

			// Look ahead for description
			if i+1 < len(lines) {
				for j := i + 1; j < len(lines) && j < i+5; j++ {
					nextLine := strings.TrimSpace(lines[j])
					if strings.HasPrefix(nextLine, "- ") {
						keywords = append(keywords, strings.TrimPrefix(nextLine, "- "))
					} else if nextLine != "" && !strings.HasPrefix(nextLine, "###") {
						if description == "" {
							description = nextLine
						}
					}
				}
			}

			rules = append(rules, validationRule{
				name:        ruleName,
				description: description,
				keywords:    keywords,
				severity:    "error",
			})
		}
	}

	return rules
}

// parseImplementation extracts validation rules from implementation section
func (vpt *ValidatePatternTool) parseImplementation(content string) []validationRule {
	rules := []validationRule{}

	// Check for interface definition
	if strings.Contains(content, "interface") || strings.Contains(content, "Interface") {
		rules = append(rules, validationRule{
			name:        "Interface Definition",
			description: "Pattern requires interface definition",
			keywords:    []string{"interface", "Interface"},
			severity:    "error",
		})
	}

	// Check for concrete implementation
	if strings.Contains(content, "Implementation") || strings.Contains(content, "Concrete") {
		rules = append(rules, validationRule{
			name:        "Concrete Implementation",
			description: "Pattern requires concrete implementation",
			keywords:    []string{"struct", "class", "implementation"},
			severity:    "error",
		})
	}

	return rules
}

// checkRule checks if code violates a specific rule
func (vpt *ValidatePatternTool) checkRule(code string, rule validationRule, _language string) map[string]interface{} {
	codeLower := strings.ToLower(code)

	// Check for required keywords
	if len(rule.keywords) > 0 {
		hasKeyword := false
		for _, keyword := range rule.keywords {
			keywordLower := strings.ToLower(keyword)
			if strings.Contains(codeLower, keywordLower) {
				hasKeyword = true
				break
			}
		}

		// For implementation rules, missing keywords indicate violation
		if rule.name == "Interface Definition" || rule.name == "Concrete Implementation" {
			if !hasKeyword {
				return map[string]interface{}{
					"rule":        rule.name,
					"description": rule.description,
					"severity":    rule.severity,
				}
			}
		}
	}

	// Check for anti-patterns based on rule name
	if strings.Contains(strings.ToLower(rule.name), "anemic") {
		// Check if code has only CRUD methods
		hasCRUD := strings.Contains(codeLower, "create") ||
			strings.Contains(codeLower, "read") ||
			strings.Contains(codeLower, "update") ||
			strings.Contains(codeLower, "delete")

		hasDomainLogic := strings.Contains(codeLower, "validate") ||
			strings.Contains(codeLower, "calculate") ||
			strings.Contains(codeLower, "process")

		if hasCRUD && !hasDomainLogic {
			return map[string]interface{}{
				"rule":        rule.name,
				"description": "Code appears to contain only CRUD operations without domain-specific logic",
				"severity":    rule.severity,
			}
		}
	}

	if strings.Contains(strings.ToLower(rule.name), "leaky") {
		// Check for data source-specific types
		leakyKeywords := []string{"sql.", "*sql.", "database/sql", "mongo.", "redis."}
		for _, keyword := range leakyKeywords {
			if strings.Contains(codeLower, keyword) {
				return map[string]interface{}{
					"rule":        rule.name,
					"description": "Code may expose data source implementation details",
					"severity":    rule.severity,
				}
			}
		}
	}

	return nil
}

// generateSuggestions creates actionable suggestions based on violations
func (vpt *ValidatePatternTool) generateSuggestions(violations []map[string]interface{}, _patternContent string) []string {
	suggestions := []string{}
	suggestionSet := make(map[string]bool)

	for _, violation := range violations {
		rule, _ := violation["rule"].(string)

		var suggestion string
		switch {
		case strings.Contains(strings.ToLower(rule), "interface"):
			suggestion = "Define a clear interface that abstracts the implementation details"
		case strings.Contains(strings.ToLower(rule), "implementation"):
			suggestion = "Provide a concrete implementation of the pattern interface"
		case strings.Contains(strings.ToLower(rule), "anemic"):
			suggestion = "Add domain-specific query methods beyond basic CRUD operations"
		case strings.Contains(strings.ToLower(rule), "leaky"):
			suggestion = "Hide data source implementation details behind the interface abstraction"
		case strings.Contains(strings.ToLower(rule), "error"):
			suggestion = "Implement proper error handling with domain-specific error types"
		default:
			suggestion = fmt.Sprintf("Review the pattern documentation for guidance on: %s", rule)
		}

		// Avoid duplicate suggestions
		if !suggestionSet[suggestion] {
			suggestions = append(suggestions, suggestion)
			suggestionSet[suggestion] = true
		}
	}

	// Add general suggestion to review pattern
	if len(suggestions) > 0 {
		suggestions = append(suggestions, "Review the complete pattern documentation for detailed implementation guidance")
	}

	return suggestions
}
