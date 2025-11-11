# Multi-stage build for MCP Architecture Service
# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o mcp-server \
    ./cmd/mcp-server

# Stage 2: Create the runtime image
FROM alpine:3.19

# Install runtime dependencies including process monitoring tools
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    procps \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1001 -S mcpuser && \
    adduser -u 1001 -S mcpuser -G mcpuser

# Set working directory
WORKDIR /app

# Create necessary directories with proper permissions
RUN mkdir -p /app/tmp /app/logs && \
    chown -R mcpuser:mcpuser /app

# Copy the binary from builder stage
COPY --from=builder /build/mcp-server /app/mcp-server

# Copy documentation files (required for the service to function)
COPY --chown=mcpuser:mcpuser docs/ /app/docs/

# Copy prompt definitions (required for prompts capability)
COPY --chown=mcpuser:mcpuser prompts/ /app/prompts/

# Ensure the binary is executable
RUN chmod +x /app/mcp-server

# Create health check script
RUN echo '#!/bin/sh' > /app/healthcheck.sh && \
    echo 'if pgrep -f "mcp-server" > /dev/null; then' >> /app/healthcheck.sh && \
    echo '  exit 0' >> /app/healthcheck.sh && \
    echo 'else' >> /app/healthcheck.sh && \
    echo '  exit 1' >> /app/healthcheck.sh && \
    echo 'fi' >> /app/healthcheck.sh && \
    chmod +x /app/healthcheck.sh && \
    chown mcpuser:mcpuser /app/healthcheck.sh

# Switch to non-root user
USER mcpuser

# Declare volume for prompts directory to support customization
VOLUME ["/app/prompts"]

# Set environment variables
ENV DOCS_PATH=/app/docs
ENV TMPDIR=/app/tmp

# Security labels and metadata
LABEL security.non-root="true" \
      security.readonly-rootfs="true" \
      security.no-new-privileges="true"

# Health check configuration
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /app/healthcheck.sh

# The MCP server communicates via stdio (no network ports needed)
# It will wait for JSON-RPC messages on stdin and respond on stdout
# Set the entrypoint to the MCP server binary
ENTRYPOINT ["/app/mcp-server"]