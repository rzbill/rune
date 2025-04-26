.PHONY: build test lint clean setup dev generate proto proto-tools docs docker install coverage-report coverage-summary test test-unit test-integration

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

test-unit:
	@echo "Running unit tests..."
	@go test -tags=unit -v ./...

test-integration:
	@echo "Running integration tests..."
	@scripts/integration/run_tests.sh

test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

# Display coverage report from existing coverage.out file
coverage-report:
	@if [ ! -f coverage.out ]; then \
		echo "Error: coverage.out file not found. Run 'make test-coverage' first."; \
		exit 1; \
	fi
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Just show coverage summary without regenerating the report
coverage-summary:
	@if [ ! -f coverage.out ]; then \
		echo "Error: coverage.out file not found. Run 'make test-coverage' first."; \
		exit 1; \
	fi
	@echo "───────────────────────────────────────────────────"
	@$(GO) tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3}'
	@echo "───────────────────────────────────────────────────"

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

# Protocol Buffer targets
proto:
	@echo "Generating Protocol Buffer code..."
	@bash scripts/generate-proto.sh
	@echo "Protocol Buffer code generation completed!"

proto-tools:
	@echo "Installing Protocol Buffer tools..."
	@bash scripts/install-proto-tools.sh
	@echo "Protocol Buffer tools installation completed!"

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
	@echo "  coverage-report    Generate HTML coverage report from existing coverage.out"
	@echo "  coverage-summary   Display coverage summary from existing coverage.out"
	@echo "  lint           - Run linters"
	@echo "  clean          - Clean build artifacts"
	@echo "  setup          - Set up development environment"
	@echo "  dev            - Start development environment"
	@echo "  generate       - Run code generation"
	@echo "  proto          - Generate Protocol Buffer code"
	@echo "  proto-tools    - Install Protocol Buffer tools"
	@echo "  docs           - Generate and serve documentation"
	@echo "  docker         - Build Docker image"
	@echo "  help           - Show this help message"

# Default target
.DEFAULT_GOAL := build 