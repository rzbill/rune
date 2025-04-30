.PHONY: build test lint clean setup dev generate proto proto-tools docs docker install coverage-report coverage-summary test-unit test-integration unit-coverage integration-coverage coverage help

# Tools and paths
GO ?= go
BIN_DIR ?= bin

# Project metadata
BINARY_NAME = rune
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS = -X github.com/rzbill/rune/pkg/version.Version=$(VERSION) \
          -X github.com/rzbill/rune/pkg/version.BuildTime=$(BUILD_TIME)

# Coverage files
UNIT_COVERAGE = coverage_unit.out
INTEGRATION_COVERAGE = coverage_integration.out

# Default goal
.DEFAULT_GOAL := build

## Build binaries
build:
	@echo "Building $(BINARY_NAME)..."
	@echo "LDFLAGS: $(LDFLAGS)"
	@$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/rune
	@$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)d ./cmd/runed
	@echo "Build completed!"

## Install binaries to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	@install -m 755 $(BIN_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@install -m 755 $(BIN_DIR)/$(BINARY_NAME)d $(GOPATH)/bin/
	@echo "Installation completed!"

## Run all tests
test: test-unit test-integration

## Run unit tests via script
test-unit:
	@bash scripts/run_unit_tests.sh

## Run integration tests via script
test-integration:
	@bash scripts/integration/run_tests.sh

## Open unit test coverage report
unit-coverage:
	@$(GO) tool cover -html=$(UNIT_COVERAGE) -o $(UNIT_COVERAGE).html && open $(UNIT_COVERAGE).html

## Open integration test coverage report
integration-coverage:
	@$(GO) tool cover -html=$(INTEGRATION_COVERAGE) -o $(INTEGRATION_COVERAGE).html && open $(INTEGRATION_COVERAGE).html

## View combined coverage report if coverage.out exists
coverage-report:
	@if [ -f $(UNIT_COVERAGE) ]; then $(GO) tool cover -html=$(UNIT_COVERAGE) -o unit_coverage.html && echo "Opened unit coverage report."; fi
	@if [ -f $(INTEGRATION_COVERAGE) ]; then $(GO) tool cover -html=$(INTEGRATION_COVERAGE) -o integration_coverage.html && echo "Opened integration coverage report."; fi

## Show coverage summary from available reports
coverage-summary:
	@if [ -f $(UNIT_COVERAGE) ]; then \
		echo "─ Unit Test Coverage ─"; \
		$(GO) tool cover -func=$(UNIT_COVERAGE) | grep total; fi
	@if [ -f $(INTEGRATION_COVERAGE) ]; then \
		echo "─ Integration Test Coverage ─"; \
		$(GO) tool cover -func=$(INTEGRATION_COVERAGE) | grep total; fi

coverage: unit-coverage integration-coverage

## Lint the project
lint:
	@echo "Running linters..."
	@golangci-lint run ./...

## Clean build and coverage artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR) *.out *.html
	@echo "Clean completed!"

## Setup development environment
setup:
	@echo "Setting up development tools..."
	@$(GO) mod tidy
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/golang/mock/mockgen@latest
	@$(GO) install golang.org/x/tools/cmd/godoc@latest
	@echo "Setup completed!"

## Start development environment
dev:
	@echo "Starting dev environment..."
	@docker-compose up -d

## Run code generation
generate:
	@echo "Running go generate..."
	@$(GO) generate ./...

## Generate protobuf files
proto:
	@bash scripts/generate-proto.sh

## Install protobuf tools
proto-tools:
	@bash scripts/install-proto-tools.sh

## Run documentation server
docs:
	@godoc -http=:6060 &
	@echo "Docs available at http://localhost:6060"

## Build docker image
docker:
	@docker build -t razorbill/$(BINARY_NAME):$(VERSION) .

## Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  build             Build binaries"
	@echo "  install           Build and install to GOPATH/bin"
	@echo ""
	@echo "Testing:"
	@echo "  test              Run all tests"
	@echo "  test-unit         Run unit tests"
	@echo "  test-integration  Run integration tests"
	@echo "  coverage          Open coverage reports"
	@echo "  coverage-summary  Show text-based summaries"
	@echo ""
	@echo "Dev Tools:"
	@echo "  lint              Run linters"
	@echo "  clean             Clean all artifacts"
	@echo "  setup             Install dev tools"
	@echo "  dev               Start dev environment (Docker)"
	@echo "  generate          Run go generate"
	@echo ""
	@echo "Protobuf:"
	@echo "  proto             Generate Protobuf code"
	@echo "  proto-tools       Install Protobuf tools"
	@echo ""
	@echo "Docs & Docker:"
	@echo "  docs              Serve documentation"
	@echo "  docker            Build Docker image"
