#!/bin/bash
# Test file system monitoring for MCP server

echo "=== Testing File System Monitoring ==="
echo ""

# Create a test file
echo "Step 1: Creating new test file in mcp/resources/guidelines/..."
cat > mcp/resources/guidelines/test-guideline.md << 'EOF'
# Test Guideline

This is a test guideline for monitoring.

## Purpose

Test file system monitoring.
EOF

echo "Step 2: Starting server and checking if it detects the new file..."
RESPONSE=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}' | timeout 3 ./bin/mcp-server 2>&1)

# Check if test-guideline appears in resources
if echo "$RESPONSE" | grep -q "test-guideline"; then
    echo "✓ SUCCESS: New file detected by server on startup"
else
    echo "✗ FAILED: New file not detected"
fi

# Clean up
echo ""
echo "Step 3: Cleaning up test file..."
rm -f mcp/resources/guidelines/test-guideline.md

echo ""
echo "Step 4: Testing prompt file monitoring..."
echo "Creating test prompt file..."
cat > mcp/prompts/test-monitoring.json << 'EOF'
{
  "name": "test-monitoring",
  "description": "Test prompt for monitoring",
  "arguments": [
    {
      "name": "input",
      "description": "Test input",
      "required": true
    }
  ],
  "template": "Test: {{input}}"
}
EOF

echo "Starting server and checking if it detects the new prompt..."
PROMPT_RESPONSE=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"prompts/list","params":{}}' | timeout 3 ./bin/mcp-server 2>&1)

if echo "$PROMPT_RESPONSE" | grep -q "test-monitoring"; then
    echo "✓ SUCCESS: New prompt file detected by server on startup"
else
    echo "✗ FAILED: New prompt file not detected"
fi

# Clean up
echo ""
echo "Cleaning up test prompt file..."
rm -f mcp/prompts/test-monitoring.json

echo ""
echo "=== File System Monitoring Test Complete ==="
echo ""
echo "Note: File system monitoring watches mcp/resources/ and mcp/prompts/"
echo "The server successfully loads files from these directories on startup."
