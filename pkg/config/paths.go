package config

// Path configuration constants for MCP resources and prompts
const (
	// Base paths for MCP assets
	ResourcesBasePath = "mcp/resources"
	PromptsBasePath   = "mcp/prompts"

	// Resource subdirectory paths
	GuidelinesPath = ResourcesBasePath + "/guidelines"
	PatternsPath   = ResourcesBasePath + "/patterns"
	ADRPath        = ResourcesBasePath + "/adr"
)

// Category constants for resource classification
const (
	CategoryGuideline = "guideline"
	CategoryPattern   = "pattern"
	CategoryADR       = "adr"
	CategoryUnknown   = "unknown"
)

// URI scheme and format constants
const (
	URIScheme      = "architecture://"
	URIGuidelines  = "guidelines"
	URIPatterns    = "patterns"
	URIADR         = "adr"
	URIUnknown     = "unknown"
	URIFormatError = "Invalid URI format, expected 'architecture://{category}/{path}'"
)

// File extension constants
const (
	MimeTypeMarkdown  = "text/markdown"
	MarkdownExtension = ".md"
)
