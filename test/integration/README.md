# Integration Testing

This directory contains integration tests for the Rune platform. The tests are designed to work with different storage backends to ensure compatibility and reliability.

## Storage Types

The integration tests support two storage types:

- **Memory Store** (`memory`): Fast, in-memory storage for quick development feedback
- **BadgerDB Store** (`badger`): Real persistent storage that matches production behavior

## Running Tests

### Default (BadgerDB Store - Production-like)
```bash
# Run all integration tests with BadgerDB store (production-like)
make test-integration

# Or run directly
cd test/integration/cmd && go test -v -tags=integration
```

### Specific Storage Types

#### Memory Store (Fast)
```bash
make test-integration-memory
```

#### BadgerDB Store (Real Storage)
```bash
make test-integration-badger
```

#### Custom Storage Type
```bash
make test-integration-store STORE=memory
make test-integration-store STORE=badger
```

### Environment Variable Control

You can also control the storage type directly via environment variable:

```bash
# Memory store (fast development)
RUNE_TEST_STORE_TYPE=memory go test -v -tags=integration

# BadgerDB store (default, production-like)
RUNE_TEST_STORE_TYPE=badger go test -v -tags=integration
# Or simply omit the variable - BadgerDB is the default
go test -v -tags=integration
```

## Test Structure

- **`get_test.go`**: Tests the `rune get` command functionality
- **`helpers/test_helper.go`**: Configurable test helper that supports both storage types

## Benefits

1. **Fast Development**: Use memory store for quick feedback during development
2. **Production Confidence**: Use BadgerDB store to test against real storage behavior
3. **Flexible**: Easy to switch between storage types via Makefile or environment variables
4. **Consistent**: Same test code works with both storage backends

## Example Workflow

```bash
# Quick development testing (fast)
make test-integration-memory

# Production confidence testing (slower but more realistic)
make test-integration-badger

# Run specific test with custom storage
RUNE_TEST_STORE_TYPE=badger go test -v -tags=integration -run TestGetCommand
``` 