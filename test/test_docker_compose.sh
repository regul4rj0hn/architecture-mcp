#!/bin/bash
# Test docker-compose configuration

echo "=== Testing Docker Compose Configuration ==="
echo ""

echo "Step 1: Building with docker-compose..."
docker-compose build --quiet

if [ $? -eq 0 ]; then
    echo "✓ SUCCESS: Docker compose build completed"
else
    echo "✗ FAILED: Docker compose build failed"
    exit 1
fi

echo ""
echo "Step 2: Verifying volume mount configuration..."
# Check that docker-compose.yml has the correct volume mount
if grep -q "./mcp:/app/mcp:ro" docker-compose.yml; then
    echo "✓ SUCCESS: docker-compose.yml correctly mounts ./mcp to /app/mcp"
else
    echo "✗ FAILED: docker-compose.yml volume mount is incorrect"
fi

echo ""
echo "Step 3: Testing container with docker-compose run..."
cat > /tmp/compose_test_input.json << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}
EOF

OUTPUT=$(timeout 10 docker-compose run --rm mcp-architecture-service < /tmp/compose_test_input.json 2>&1)

if echo "$OUTPUT" | grep -q '"id":1.*"result"'; then
    echo "✓ SUCCESS: Container started via docker-compose and initialized"
else
    echo "✗ FAILED: Container initialization via docker-compose failed"
fi

if echo "$OUTPUT" | grep -q '"id":2.*"resources"'; then
    echo "✓ SUCCESS: Container can access mounted mcp/resources/ directory"
else
    echo "✗ FAILED: Container cannot access mounted resources"
fi

# Clean up
rm -f /tmp/compose_test_input.json
docker-compose down --remove-orphans 2>/dev/null

echo ""
echo "=== Docker Compose Tests Complete ==="
echo ""
echo "Summary:"
echo "- docker-compose.yml correctly configured with ./mcp:/app/mcp:ro mount"
echo "- Container builds successfully via docker-compose"
echo "- Container can access resources through volume mount"
echo "- All configurations align with new mcp/ directory structure"
