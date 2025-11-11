#!/bin/bash
# Integration test script for MCP server

SERVER="./bin/mcp-server"
TIMEOUT=5

echo "=== Starting MCP Server Integration Tests ==="
echo ""

# Function to send JSON-RPC request and get response
send_request() {
    local request="$1"
    echo "$request" | timeout $TIMEOUT $SERVER 2>/dev/null
}

# Test 1: Initialize
echo "Test 1: Initialize"
INIT_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'
INIT_RESPONSE=$(send_request "$INIT_REQUEST")
echo "Response: $INIT_RESPONSE"
echo ""

# Test 2: List Resources
echo "Test 2: List Resources"
LIST_REQUEST='{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}'
LIST_RESPONSE=$(send_request "$LIST_REQUEST")
echo "Response: $LIST_RESPONSE"
echo ""

# Test 3: Read a Resource (API Design guideline)
echo "Test 3: Read Resource (architecture://guidelines/api-design)"
READ_REQUEST='{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"architecture://guidelines/api-design"}}'
READ_RESPONSE=$(send_request "$READ_REQUEST")
echo "Response (truncated): ${READ_RESPONSE:0:200}..."
echo ""

# Test 4: List Prompts
echo "Test 4: List Prompts"
PROMPTS_LIST_REQUEST='{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}'
PROMPTS_LIST_RESPONSE=$(send_request "$PROMPTS_LIST_REQUEST")
echo "Response: $PROMPTS_LIST_RESPONSE"
echo ""

# Test 5: Get a Prompt
echo "Test 5: Get Prompt (create-adr)"
PROMPT_GET_REQUEST='{"jsonrpc":"2.0","id":5,"method":"prompts/get","params":{"name":"create-adr","arguments":{"topic":"test decision"}}}'
PROMPT_GET_RESPONSE=$(send_request "$PROMPT_GET_REQUEST")
echo "Response (truncated): ${PROMPT_GET_RESPONSE:0:200}..."
echo ""

echo "=== Integration Tests Complete ==="
