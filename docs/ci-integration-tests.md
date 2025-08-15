# CI Integration Tests Guide

This guide explains how to run Rune integration tests in various CI environments, including GitHub Actions, GitLab CI, and local Docker setups.

## Overview

Rune integration tests require:
- **Go 1.21+** for building and running tests
- **Docker** for container-based testing
- **Storage backend** (BadgerDB or Memory store)

## GitHub Actions

### Automatic Workflow

The integration tests run automatically on:
- Push to main/master/develop branches
- Pull requests to main/master/develop branches
- Daily at 2 AM UTC (scheduled)
- Manual trigger via workflow_dispatch

### Workflow Features

- **Matrix testing**: Tests both memory and BadgerDB storage
- **Docker support**: Full Docker daemon setup
- **Go version matrix**: Tests against Go 1.21, 1.22, 1.23
- **Artifact upload**: Test results and coverage reports

### Manual Trigger

```yaml
# In GitHub Actions UI:
# 1. Go to Actions tab
# 2. Select "Integration Tests" workflow
# 3. Click "Run workflow"
# 4. Choose storage type (memory/badger)
# 5. Click "Run workflow"
```

### Workflow File

```yaml
# .github/workflows/integration-tests-matrix.yml
name: Integration Tests Matrix
on:
  push:
    branches: [ main, master, develop ]
  pull_request:
    branches: [ main, master, develop ]
  schedule:
    - cron: '0 2 * * *'
  workflow_dispatch:
    inputs:
      storage_type:
        description: 'Storage type for tests'
        required: false
        default: 'badger'
        type: choice
        options:
        - memory
        - badger
```

## GitLab CI

### .gitlab-ci.yml Example

```yaml
stages:
  - test

variables:
  RUNE_TEST_STORE_TYPE: "badger"
  RUNE_INTEGRATION_TESTS: "1"

integration-tests:
  stage: test
  image: docker:24.0.5-dind
  services:
    - docker:24.0.5-dind
  variables:
    DOCKER_TLS_CERTDIR: ""
  before_script:
    - apk add --no-cache go git bash
    - go version
    - docker --version
  script:
    - go mod download
    - go build -o bin/rune-test ./cmd/rune-test
    - bash scripts/run_integration_tests.sh
  artifacts:
    paths:
      - coverage_integration.out
    expire_in: 1 week
```

## Local Docker Testing

### Docker Compose

```bash
# Run tests in Docker environment
make test-integration-docker

# Or directly
docker-compose -f docker-compose.test.yml --profile test up --abort-on-container-exit
```

### Docker-in-Docker

```bash
# Run tests with Docker-in-Docker
make test-integration-dind

# Or directly
docker run --rm --privileged \
  -v $(pwd):/workspace \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e RUNE_TEST_STORE_TYPE=badger \
  -e RUNE_INTEGRATION_TESTS=1 \
  -w /workspace \
  docker:24.0.5-dind \
  sh -c "apk add --no-cache go git bash && go build -o bin/rune-test ./cmd/rune-test && bash scripts/run_integration_tests.sh"
```

## Environment Variables

### Required Variables

```bash
RUNE_INTEGRATION_TESTS=1          # Enable integration tests
DOCKER_HOST=unix:///var/run/docker.sock  # Docker socket path
```

### Optional Variables

```bash
RUNE_TEST_STORE_TYPE=badger       # Storage type (badger/memory)
RUNE_BINARY=/path/to/rune-test    # Custom test binary path
```

## Storage Types

### BadgerDB (Default - Production-like)

```bash
# Default behavior
go test -v -tags=integration

# Explicit BadgerDB
RUNE_TEST_STORE_TYPE=badger go test -v -tags=integration
make test-integration-badger
```

### Memory Store (Fast Development)

```bash
# Fast memory store
RUNE_TEST_STORE_TYPE=memory go test -v -tags=integration
make test-integration-memory
```

## Troubleshooting

### Docker Issues

```bash
# Check Docker daemon
docker info

# Test Docker functionality
docker run --rm hello-world

# Check Docker socket permissions
ls -la /var/run/docker.sock
```

### Permission Issues

```bash
# Fix Docker socket permissions (Linux)
sudo chmod 666 /var/run/docker.sock

# Add user to docker group
sudo usermod -aG docker $USER
```

### Test Timeouts

```bash
# Increase test timeout
go test -v -tags=integration -timeout=10m

# Check for hanging containers
docker ps -a
docker system prune -f
```

## Best Practices

### CI Environment

1. **Use Docker-in-Docker** for isolated testing
2. **Set appropriate timeouts** (10m+ for integration tests)
3. **Clean up resources** after tests
4. **Upload artifacts** for debugging

### Local Development

1. **Use memory store** for fast feedback
2. **Use BadgerDB** for production confidence
3. **Clean up test data** regularly
4. **Monitor Docker resource usage**

### Test Data

1. **Use temporary directories** for test storage
2. **Clean up after tests** complete
3. **Avoid persistent state** between test runs
4. **Use unique names** for test resources

## Examples

### GitHub Actions Matrix

```yaml
strategy:
  matrix:
    storage: [memory, badger]
    go-version: [1.21, 1.22, 1.23]
```

### Local Testing Commands

```bash
# Quick development
make test-integration-memory

# Production confidence
make test-integration-badger

# Docker environment
make test-integration-docker

# Custom storage type
make test-integration-store STORE=memory
```

### Environment Override

```bash
# Override default storage type
RUNE_TEST_STORE_TYPE=memory make test-integration

# Run specific test with custom storage
RUNE_TEST_STORE_TYPE=badger go test -v -tags=integration -run TestGetCommand
```
