# Rune Integration Tests

This directory contains integration tests for the Rune project. These tests interact with real dependencies like Docker to verify that Rune's components work correctly in a realistic environment.

## Prerequisites

- Docker must be installed and running
- Go 1.20 or later

## Running Integration Tests

### Via Makefile

```bash
# Run the integration tests
make test-integration
```

### Directly with Go

```bash
# Run all integration tests
go test -tags=integration ./test/integration/...

# Run specific integration tests
go test -tags=integration ./test/integration/docker/...
```

## Test Structure

The integration tests use build tags to separate them from unit tests:

- All integration test files should include `// +build integration` or `//go:build integration` at the top
- Integration tests are not run by default with `go test ./...`
- The continuous integration pipeline runs them separately from unit tests

## Test Helpers

The integration test suite includes several helpers to make writing tests easier:

- `DockerHelper`: A wrapper around the Docker client to create containers, pull images, etc.
- `IntegrationTest`: A function to set up and tear down test infrastructure

## Adding New Tests

1. Create a new test file in the appropriate package
2. Add the build tag `// +build integration` at the top
3. Use the helper functions to set up any required infrastructure
4. Implement the test cases, making sure to clean up resources

## CI/CD Integration

The integration tests are run in CI/CD only on:
- Scheduled runs
- When explicitly triggered via workflow_dispatch

This is to avoid long test runs on every push, while still ensuring that integration tests are run regularly. 