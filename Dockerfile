# Multi-stage Docker build for production web crawler
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o web-crawler .

# Final stage - minimal production image
FROM python:3.11-alpine

# Install system dependencies for AI processing
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# Install Python AI dependencies
RUN pip install --no-cache-dir \
    openai \
    anthropic \
    google-genai

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup

# Set working directory
WORKDIR /app

# Copy built binary from builder stage
COPY --from=builder /app/web-crawler .

# Copy templates and static files
COPY --from=builder /app/templates ./templates/

# Create directories with proper permissions
RUN mkdir -p /app/cache /app/logs /app/baselines && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8081/ || exit 1

# Run the application
CMD ["./web-crawler"]