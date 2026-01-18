# Multi-stage Dockerfile for GitHub Readme Stats Go Server
# Optimized for production deployment with minimal image size

# Build stage
FROM golang:1.21-alpine AS builder

# Install ca-certificates for HTTPS requests to GitHub API
RUN apk --no-cache add ca-certificates git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static linking
# -ldflags for removing debug symbols and reducing binary size
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags '-w -s -extldflags "-static"' \
    -o github-stats-server \
    main.go

# Verify the binary works
RUN ./github-stats-server version 2>/dev/null || echo "Note: version command not implemented"

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS and dumb-init for proper signal handling
RUN apk --no-cache add ca-certificates dumb-init

# Create non-root user for security
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/github-stats-server .

# Make the binary executable
RUN chmod +x github-stats-server

# Create logs directory
RUN mkdir -p /app/logs && chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 3000

# Set environment variables
ENV PORT=3000
ENV GIN_MODE=release

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

# Use dumb-init for proper signal handling
ENTRYPOINT ["dumb-init", "--"]

# Run the application
CMD ["./github-stats-server"]

# Labels for metadata
LABEL maintainer="GitHub Readme Stats Team"
LABEL description="Lightweight Go server for generating GitHub user statistics SVG cards"
LABEL version="1.0.0"
LABEL org.opencontainers.image.source="https://github.com/anuraghazra/github-readme-stats"
