# Multi-stage build for Goldpath IDP Tool

# Stage 1: Build
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o goldpath ./cmd/goldpath/

# Stage 2: Runtime
FROM alpine:3.19

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' goldpath

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/goldpath .

# Create directories for config and data
RUN mkdir -p /app/config /app/data && \
    chown -R goldpath:goldpath /app

# Switch to non-root user
USER goldpath

# Expose port
EXPOSE 8080

# Environment variables with defaults
ENV GOLDPATH_PORT=8080
ENV GOLDPATH_HOST=0.0.0.0
ENV GOLDPATH_LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["./goldpath"]
