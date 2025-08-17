.PHONY: build test lint clean setup dev generate proto proto-tools docs docker install coverage-report coverage-summary test-unit test-integration coverage-unit coverage-integration coverage help

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

# Coverage threshold (default 60%, can be overridden via COVERAGE_THRESHOLD env var)
THRESHOLD ?= $(or $(COVERAGE_THRESHOLD),23)

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
	@$(GO) test -coverprofile=$(UNIT_COVERAGE) ./... 

## Run integration tests via script (defaults to BadgerDB store)
test-integration:
	@bash scripts/run_integration_tests.sh

## Run integration tests with memory store (fast)
test-integration-memory:
	@echo "Running integration tests with memory store..."
	@cd test/integration/cmd && RUNE_TEST_STORE_TYPE=memory go test -v -tags=integration

## Run integration tests with BadgerDB store (real storage)
test-integration-badger:
	@echo "Running integration tests with BadgerDB store..."
	@cd test/integration/cmd && RUNE_TEST_STORE_TYPE=badger go test -v -tags=integration

## Run integration tests with specific storage type
test-integration-store:
	@echo "Running integration tests with $(STORE) store..."
	@cd test/integration/cmd && RUNE_TEST_STORE_TYPE=$(STORE) go test -v -tags=integration -coverprofile=../../$(INTEGRATION_COVERAGE)

## Run integration tests in Docker (GitHub Actions style)
test-integration-docker:
	@echo "Running integration tests in Docker environment..."
	@docker-compose -f docker-compose.test.yml --profile test up --abort-on-container-exit --exit-code-from test-runner

## Run integration tests in Go container with Docker access
test-integration-docker-go:
	@echo "Running integration tests in Go container with Docker access..."
	@docker run --rm \
		-v $(PWD):/workspace \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e RUNE_TEST_STORE_TYPE=badger \
		-e RUNE_INTEGRATION_TESTS=1 \
		-e DOCKER_HOST=unix:///var/run/docker.sock \
		-w /workspace \
		golang:1.23-alpine \
		sh -c "apk add --no-cache git bash && go build -o bin/rune-test ./cmd/rune-test && bash scripts/integration/run_tests.sh"

## Run end-to-end tests (real CLI + test server)
test-e2e:
	@bash scripts/run_e2e_tests.sh

## Open unit test coverage report
coverage-unit:
	@$(GO) tool cover -html=$(UNIT_COVERAGE) -o $(UNIT_COVERAGE).html && open $(UNIT_COVERAGE).html

## Open integration test coverage report
coverage-integration:
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

## Check coverage against threshold
check-coverage:
	@echo "Checking coverage threshold: $(THRESHOLD)%"
	@if [ -f $(UNIT_COVERAGE) ]; then \
		echo "Checking unit test coverage..."; \
		unit_coverage=$$(go tool cover -func=$(UNIT_COVERAGE) | grep total: | awk '{print $$3}' | sed 's/%//'); \
		echo "Unit test coverage: $$unit_coverage%"; \
		if [ $$(echo "$$unit_coverage >= $(THRESHOLD)" | bc -l 2>/dev/null || echo "$$unit_coverage >= $(THRESHOLD)" | awk '{print $$1 >= $$3}') -eq 1 ]; then \
			echo "✅ Unit coverage ($$unit_coverage%) meets threshold ($(THRESHOLD)%)"; \
		else \
			echo "❌ Unit coverage ($$unit_coverage%) below threshold ($(THRESHOLD)%)"; \
			exit 1; \
		fi; \
	fi
	@if [ -f $(INTEGRATION_COVERAGE) ]; then \
		echo "Checking integration test coverage..."; \
		integration_coverage=$$(go tool cover -func=$(INTEGRATION_COVERAGE) | grep total: | awk '{print $$3}' | sed 's/%//'); \
		echo "Integration test coverage: $$integration_coverage%"; \
		if [ $$(echo "$$integration_coverage >= $(THRESHOLD)" | bc -l 2>/dev/null || echo "$$integration_coverage >= $(THRESHOLD)" | awk '{print $$1 >= $$3}') -eq 1 ]; then \
			echo "✅ Integration coverage ($$integration_coverage%) meets threshold ($(THRESHOLD)%)"; \
		else \
			echo "❌ Integration coverage ($$integration_coverage%) below threshold ($(THRESHOLD)%)"; \
			exit 1; \
		fi; \
	fi
	@echo "✅ All coverage thresholds met!"

## Check unit test coverage only
check-coverage-unit:
	@echo "Checking unit test coverage threshold: $(THRESHOLD)%"
	@if [ -f $(UNIT_COVERAGE) ]; then \
		unit_coverage=$$(go tool cover -func=$(UNIT_COVERAGE) | grep total: | awk '{print $$3}' | sed 's/%//'); \
		echo "Unit test coverage: $$unit_coverage%"; \
		if [ $$(echo "$$unit_coverage >= $(THRESHOLD)" | bc -l 2>/dev/null || echo "$$unit_coverage >= $(THRESHOLD)" | awk '{print $$1 >= $$3}') -eq 1 ]; then \
			echo "✅ Unit coverage ($$unit_coverage%) meets threshold ($(THRESHOLD)%)"; \
		else \
			echo "❌ Unit coverage ($$unit_coverage%) below threshold ($(THRESHOLD)%)"; \
			exit 1; \
		fi; \
	else \
		echo "❌ No unit coverage file found: $(UNIT_COVERAGE)"; \
		exit 1; \
	fi

## Check integration test coverage only
check-coverage-integration:
	@echo "Checking integration test coverage threshold: $(THRESHOLD)%"
	@if [ -f $(INTEGRATION_COVERAGE) ]; then \
		integration_coverage=$$(go tool cover -func=$(INTEGRATION_COVERAGE) | grep total: | awk '{print $$3}' | sed 's/%//'); \
		echo "Integration test coverage: $$integration_coverage%"; \
		if [ $$(echo "$$integration_coverage >= $(THRESHOLD)" | bc -l 2>/dev/null || echo "$$integration_coverage >= $(THRESHOLD)" | awk '{print $$1 >= $$3}') -eq 1 ]; then \
			echo "✅ Integration coverage ($$integration_coverage%) meets threshold ($(THRESHOLD)%)"; \
		else \
			echo "❌ Integration coverage ($$integration_coverage%) below threshold ($(THRESHOLD)%)"; \
			exit 1; \
		fi; \
	else \
		echo "❌ No integration coverage file found: $(INTEGRATION_COVERAGE)"; \
		exit 1; \
	fi

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
	@echo "  test-integration  Run integration tests (BadgerDB store by default)"
	@echo "  test-integration-memory  Run integration tests with memory store (fast)"
	@echo "  test-integration-badger  Run integration tests with BadgerDB store (real storage)"
	@echo "  test-integration-store STORE=<type>  Run integration tests with specific store type"
	@echo "  test-integration-docker              Run integration tests in Docker environment"
	@echo "  test-integration-docker-go           Run integration tests in Go container with Docker access"
	@echo "  test-e2e          Run end-to-end tests (real CLI + test server)"
	@echo "  coverage          Open coverage reports"
	@echo "  coverage-summary  Show text-based summaries"
	@echo ""
	@echo "Dev Tools:"
	@echo "  lint              Run linters"
	@echo "  clean             Clean all artifacts"
	@echo "  setup             Install dev tools"

	@echo "  generate          Run go generate"
	@echo ""
	@echo "Protobuf:"
	@echo "  proto             Generate Protobuf code"
	@echo "  proto-tools       Install Protobuf tools"
	@echo ""
	@echo "Docs & Docker:"
	@echo "  docs              Serve documentation"
	@echo "  docker            Build Docker image"
		echo ""
		echo "Coverage:"
		echo "  check-coverage    Check coverage against threshold ($(THRESHOLD)%)"
		echo "  check-coverage-unit    Check unit test coverage against threshold"
		echo "  check-coverage-integration    Check integration test coverage against threshold"
