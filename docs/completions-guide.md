# Completions Guide

The MCP Architecture Service provides intelligent autocomplete suggestions for prompt arguments through the `completion/complete` endpoint.

## Overview

Completions help users discover and select valid values for prompt arguments by providing context-aware suggestions based on your documentation catalog.

## Supported Arguments

The service automatically provides completions for these argument types:

### Pattern Names (`pattern_name`)
Suggests pattern names from `mcp/resources/patterns/`:
- Converts filenames to human-readable format (e.g., `repository-pattern.md` → `repository pattern`)
- Includes pattern title as description
- Supports prefix-based filtering

### Guideline Names (`guideline_name`)
Suggests guideline names from `mcp/resources/guidelines/`:
- Converts filenames to human-readable format (e.g., `api-design.md` → `api design`)
- Includes guideline title as description
- Supports prefix-based filtering

### ADR IDs (`adr_id`)
Suggests ADR identifiers from `mcp/resources/adr/`:
- Extracts ADR ID from filename (e.g., `001-microservices-architecture.md` → `001-microservices-architecture`)
- Includes ADR title as description
- Supports prefix-based filtering

## How It Works

1. **Client Request**: IDE or AI agent sends a `completion/complete` request with:
   - Prompt name reference
   - Argument name being completed
   - Current partial value (prefix)

2. **Server Processing**:
   - Validates the prompt exists
   - Determines completion type based on argument name
   - Queries the documentation cache
   - Filters results by prefix (case-insensitive)
   - Returns formatted completion items

3. **Client Display**: IDE shows suggestions to the user with:
   - Value (what gets inserted)
   - Label (display text)
   - Description (document title)

## Request Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "completion/complete",
  "params": {
    "ref": {
      "type": "ref/prompt",
      "name": "review-code-against-patterns"
    },
    "argument": {
      "name": "pattern_name",
      "value": "repo"
    }
  }
}
```

## Response Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "completion": {
      "values": [
        {
          "value": "repository pattern",
          "label": "repository pattern",
          "description": "Repository Pattern"
        }
      ],
      "total": 1,
      "hasMore": false
    }
  }
}
```

## Prefix Filtering

Completions support case-insensitive prefix matching:

- Empty prefix (`""`) returns all available options
- Partial prefix (`"repo"`) returns matching items (`"repository pattern"`)
- Case is ignored (`"REPO"` matches `"repository pattern"`)

## Adding Completion Support to Custom Prompts

To enable completions for your custom prompts:

1. Use standard argument names in your prompt definition:
   ```json
   {
     "name": "my-custom-prompt",
     "arguments": [
       {
         "name": "pattern_name",
         "description": "Pattern to apply",
         "required": true
       }
     ]
   }
   ```

2. The service automatically provides completions for `pattern_name`, `guideline_name`, and `adr_id` arguments

3. Other argument names will return empty completion lists (no suggestions)

## Error Handling

The service returns structured errors for:

- Invalid reference type (must be `"ref/prompt"`)
- Prompt not found
- Missing required parameters
- Internal processing errors

## Performance

- Completions are served from in-memory cache (fast)
- Results are filtered on-demand (no pre-computation)
- Typical response time: < 10ms for hundreds of documents

## Best Practices

1. **Use descriptive filenames**: They become completion values
2. **Add clear titles**: They appear as descriptions
3. **Keep catalogs organized**: Easier to discover relevant items
4. **Use consistent naming**: Helps with prefix matching

## Example Use Cases

### IDE Integration
```typescript
// User types "rep" in pattern_name field
// IDE requests completions
// Server returns: ["repository pattern", "replication pattern"]
// User selects from dropdown
```

### AI Agent Workflow
```typescript
// Agent needs to validate code against a pattern
// Agent requests completions for pattern_name
// Server returns all available patterns
// Agent selects most relevant pattern
// Agent calls validate-against-pattern tool
```

## See Also

- [Prompts Guide](prompts-guide.md) - Creating custom prompts
- [Tools Guide](tools-guide.md) - Using tools with prompts
- [Architecture Overview](architecture.md) - System design
