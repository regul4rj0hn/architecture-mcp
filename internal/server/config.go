package server

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
	categoryGuideline = "guideline"
	categoryPattern   = "pattern"
	categoryADR       = "adr"
	categoryUnknown   = "unknown"
)
