# Makefile for Goldpath IDP Tool

.PHONY: build test clean run docker-build docker-run lint coverage help

# Build variables
BINARY_NAME=goldpath
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# Default target
help:
	@echo "Goldpath IDP Tool - Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Run the application"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  lint         - Run linters"
	@echo "  coverage     - Run tests with coverage"

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	go build ${GO_LDFLAGS} -o ${BINARY_NAME} ./cmd/goldpath/
	@echo "Built ${BINARY_NAME}"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f ${BINARY_NAME}
	rm -rf coverage.out
	@echo "Cleaned"

# Run the application
run:
	@echo "Running ${BINARY_NAME}..."
	./${BINARY_NAME}

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t goldpath:latest .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 goldpath:latest

# Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate code (if needed)
generate:
	@echo "Generating code..."
	# Add code generation commands here as needed
