# Prompt Definition Guide

## Overview

Prompts are interactive templates that combine instructions with architectural documentation to guide AI-assisted workflows. This guide explains how to create custom prompts for the MCP Architecture Service.

## Prompt Definition Format

Prompts are defined as JSON files in the `prompts/` directory. Each prompt file must follow this structure:

```json
{
  "name": "prompt-name",
  "description": "Brief description of what this prompt does",
  "arguments": [
    {
      "name": "argumentName",
      "description": "Description of this argument",
      "required": true,
      "maxLength": 10000
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Template text with {{argumentName}} and {{resource:architecture://patterns/*}}"
      }
    }
  ]
}
```

## JSON Schema Reference

### Root Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier for the prompt (lowercase, alphanumeric, hyphens only) |
| `description` | string | No | Human-readable description of the prompt's purpose |
| `arguments` | array | No | List of arguments the prompt accepts |
| `messages` | array | Yes | Template messages that form the prompt content |

### Argument Definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Argument identifier (used in templates as `{{name}}`) |
| `description` | string | No | Human-readable description of the argument |
| `required` | boolean | Yes | Whether this argument must be provided |
| `maxLength` | integer | No | Maximum character length for the argument value |

### Message Template

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `role` | string | Yes | Message role (`"user"` or `"assistant"`) |
| `content` | object | Yes | Message content definition |

### Content Template

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Content type (currently only `"text"` supported) |
| `text` | string | Yes | Template text with variable and resource placeholders |

## Template Syntax

### Variable Substitution

Use double curly braces to insert argument values:

```
{{argumentName}}
```

Example:
```json
{
  "text": "Review the following {{language}} code:\n\n```{{language}}\n{{code}}\n```"
}
```

When invoked with `{"language": "go", "code": "func main() {}"}`, this renders as:
```
Review the following go code:

```go
func main() {}
```
```

### Resource Embedding

Embed architectural documentation using the resource pattern:

```
{{resource:architecture://path/pattern}}
```

#### Specific Resource

Embed a single resource:
```
{{resource:architecture://patterns/repository-pattern}}
```

#### Wildcard Pattern

Embed all matching resources:
```
{{resource:architecture://patterns/*}}
```

This will embed all files from `docs/patterns/` directory.

#### Resource Embedding Behavior

- Resources are retrieved from the document cache
- Content is inserted at the placeholder location
- Markdown formatting is preserved
- Multiple resources are concatenated with separators
- Missing resources cause an error response

## Validation Rules

### Prompt Name

- Pattern: `^[a-z0-9-]+$`
- Must be lowercase
- Only alphanumeric characters and hyphens
- No spaces or special characters

Examples:
- ✅ `review-code`
- ✅ `suggest-patterns-v2`
- ❌ `Review_Code` (uppercase, underscore)
- ❌ `suggest patterns` (space)

### Argument Constraints

- Required arguments must be provided when invoking the prompt
- `maxLength` enforces character limits (default: no limit)
- Argument names must be valid identifiers

### Resource Limits

To prevent denial-of-service:
- Maximum 50 resources per prompt
- Maximum 1MB total embedded content per prompt
- Exceeding limits returns an error

## Complete Example

Here's a complete prompt definition for reviewing code against patterns:

```json
{
  "name": "review-code-against-patterns",
  "description": "Review code against documented architectural patterns",
  "arguments": [
    {
      "name": "code",
      "description": "The code snippet to review",
      "required": true,
      "maxLength": 10000
    },
    {
      "name": "language",
      "description": "Programming language of the code",
      "required": false
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Review the following {{language}} code against our architectural patterns:\n\n```{{language}}\n{{code}}\n```\n\nConsider the following patterns from our documentation:\n\n{{resource:architecture://patterns/*}}\n\nProvide feedback on:\n1. Which patterns are being followed correctly\n2. Which patterns are violated or could be better applied\n3. Specific recommendations for improvement"
      }
    }
  ]
}
```

## Creating Custom Prompts

### Step 1: Define Your Use Case

Identify what workflow you want to support:
- Code review against specific guidelines
- Architecture decision assistance
- Pattern recommendation
- Documentation generation

### Step 2: Determine Required Inputs

What information does the AI need from the user?
- Code snippets
- Problem descriptions
- Decision topics
- Configuration details

### Step 3: Identify Relevant Resources

Which documentation should be embedded?
- Specific guidelines: `{{resource:architecture://guidelines/api-design}}`
- All patterns: `{{resource:architecture://patterns/*}}`
- Specific ADRs: `{{resource:architecture://adr/001-microservices-architecture}}`

### Step 4: Write the Template

Combine instructions, argument placeholders, and resource embeddings:

```json
{
  "name": "my-custom-prompt",
  "description": "Description of what this does",
  "arguments": [
    {
      "name": "input",
      "description": "User input description",
      "required": true,
      "maxLength": 5000
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Instructions for the AI...\n\nUser input: {{input}}\n\nRelevant documentation:\n{{resource:architecture://guidelines/*}}"
      }
    }
  ]
}
```

### Step 5: Save and Test

1. Save the file as `prompts/my-custom-prompt.json`
2. The server will automatically reload within 2 seconds
3. Test with `prompts/list` to verify it appears
4. Test with `prompts/get` to verify rendering

## Hot Reload

The server monitors the `prompts/` directory for changes:

- **Add**: New prompt files are automatically loaded
- **Modify**: Updated prompts are reloaded
- **Delete**: Removed prompts are unregistered
- **Timing**: Changes detected within 2 seconds

No server restart required.

## Error Handling

### Common Errors

**Prompt not found**
```json
{
  "error": {
    "code": -32602,
    "message": "Prompt not found: unknown-prompt"
  }
}
```

**Missing required argument**
```json
{
  "error": {
    "code": -32602,
    "message": "Missing required arguments: code"
  }
}
```

**Argument too long**
```json
{
  "error": {
    "code": -32602,
    "message": "Argument 'code' exceeds maximum length of 10000 characters"
  }
}
```

**Resource not found**
```json
{
  "error": {
    "code": -32602,
    "message": "Resource not found: architecture://patterns/nonexistent"
  }
}
```

### Validation During Development

If a prompt definition is malformed:
- The server logs the error
- The prompt is excluded from the registry
- Other prompts continue to work
- Fix the JSON and the server will reload automatically

## Best Practices

### Naming

- Use descriptive, action-oriented names: `review-code`, `suggest-patterns`
- Keep names concise but clear
- Use hyphens to separate words

### Arguments

- Mark arguments as required only if truly necessary
- Set reasonable `maxLength` limits to prevent abuse
- Provide clear descriptions for user guidance

### Templates

- Write clear, specific instructions for the AI
- Structure output requests (numbered lists, sections)
- Include context about what the AI should focus on
- Use markdown formatting for readability

### Resource Embedding

- Be specific when possible: `architecture://patterns/repository-pattern`
- Use wildcards judiciously: `architecture://patterns/*` embeds everything
- Consider the total size of embedded content
- Test with actual documentation to verify output

### Documentation

- Write clear descriptions for each prompt
- Document expected argument formats
- Provide examples in comments or separate docs
- Keep prompts focused on a single workflow

## Advanced Patterns

### Conditional Instructions

Use argument values to guide AI behavior:

```json
{
  "text": "Review this {{language}} code. {{#if language}}Focus on {{language}}-specific best practices.{{/if}}"
}
```

Note: Conditional syntax not yet supported, but you can structure instructions to work with any input.

### Multi-step Workflows

Create separate prompts for each step:
- `analyze-requirements` - Initial analysis
- `suggest-architecture` - Architecture recommendations
- `create-adr` - Document the decision

### Combining Multiple Resources

```json
{
  "text": "Guidelines:\n{{resource:architecture://guidelines/*}}\n\nPatterns:\n{{resource:architecture://patterns/*}}\n\nPrevious Decisions:\n{{resource:architecture://adr/*}}"
}
```

Be mindful of the 1MB total content limit.

## Troubleshooting

### Prompt Not Appearing

1. Check filename matches pattern: `*.json`
2. Verify JSON syntax is valid
3. Check server logs for parsing errors
4. Ensure `name` field matches filename convention

### Template Not Rendering

1. Verify argument names match exactly (case-sensitive)
2. Check resource URIs are valid
3. Ensure resources exist in the cache
4. Review server logs for rendering errors

### Performance Issues

1. Reduce number of embedded resources
2. Use specific resources instead of wildcards
3. Set appropriate `maxLength` limits
4. Monitor total embedded content size

## Reference

### Built-in Prompts

The server includes three built-in prompts as examples:

1. **review-code-against-patterns** - Code review workflow
2. **suggest-patterns** - Pattern recommendation
3. **create-adr** - ADR creation assistance

Review these files in `prompts/` for working examples.

### Related Documentation

- MCP Protocol Specification: https://modelcontextprotocol.io
- Architecture Service README: `../README.md`
- Security Guidelines: `../SECURITY.md`
