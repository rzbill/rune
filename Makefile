.PHONY: build test lint clean setup dev generate docs docker install

# Project variables
BINARY_NAME=rune
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-X github.com/rzbill/rune/pkg/version.Version=$(VERSION) \
       -X github.com/rzbill/rune/pkg/version.BuildTime=$(BUILD_TIME)

# Build targets
build:
	@echo "Building $(BINARY_NAME)..."
	@echo "LDFLAGS: $(LDFLAGS)"
	@go build -x -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/rune
	@go build -x -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)d ./cmd/runed
	@echo "Build completed!"

# Install target
install: build
	@echo "Installing $(BINARY_NAME)..."
	@install -m 755 bin/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@install -m 755 bin/$(BINARY_NAME)d $(GOPATH)/bin/$(BINARY_NAME)d
	@echo "Installation completed! Binaries installed to $(GOPATH)/bin"

# Test targets
test:
	@echo "Running tests..."
	@go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

# Lint targets
lint:
	@echo "Running linters..."
	@golangci-lint run ./...

# Clean targets
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out
	@echo "Clean completed!"

# Setup targets
setup:
	@echo "Setting up development environment..."
	@go mod tidy
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/golang/mock/mockgen@latest
	@go install golang.org/x/tools/cmd/godoc@latest
	@echo "Setup completed!"

# Development targets
dev:
	@echo "Starting development environment..."
	@docker-compose up -d
	@echo "Development environment started!"

# Generate targets
generate:
	@echo "Running code generation..."
	@go generate ./...
	@echo "Code generation completed!"

# Documentation targets
docs:
	@echo "Generating documentation..."
	@godoc -http=:6060 &
	@echo "Documentation server started at http://localhost:6060"

# Docker targets
docker:
	@echo "Building Docker image..."
	@docker build -t razorbill/$(BINARY_NAME):$(VERSION) .
	@echo "Docker image built!"

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build the project binaries"
	@echo "  install        - Build and install binaries to GOPATH/bin"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run linters"
	@echo "  clean          - Clean build artifacts"
	@echo "  setup          - Set up development environment"
	@echo "  dev            - Start development environment"
	@echo "  generate       - Run code generation"
	@echo "  docs           - Generate and serve documentation"
	@echo "  docker         - Build Docker image"
	@echo "  help           - Show this help message"

# Default target
.DEFAULT_GOAL := build 