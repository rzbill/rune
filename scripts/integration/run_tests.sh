#!/bin/bash
set -e

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Print header
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}   Rune Integration Tests Runner        ${NC}"
echo -e "${GREEN}=========================================${NC}"

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed or not in PATH. Integration tests require Docker.${NC}"
    exit 1
fi

# Check Docker daemon is running
if ! docker info &> /dev/null; then
    echo -e "${RED}Docker daemon is not running. Please start Docker and try again.${NC}"
    exit 1
fi

echo -e "${GREEN}Docker is available. Proceeding with tests.${NC}"

# Define images required for tests
REQUIRED_IMAGES=(
    "alpine:latest"
    # Add more images as needed
)

# Pull required Docker images
echo -e "${YELLOW}Pulling required Docker images...${NC}"
for IMG in "${REQUIRED_IMAGES[@]}"; do
    echo -e "  - Pulling ${IMG}..."
    docker pull "${IMG}" > /dev/null
    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to pull ${IMG}. Tests might fail.${NC}"
    else
        echo -e "${GREEN}  - Successfully pulled ${IMG}${NC}"
    fi
done

# Set environment variables for tests
export RUNE_INTEGRATION_TESTS=1
export RUNE_INTEGRATION_DOCKER_HOST=${DOCKER_HOST:-}

# Display testing environment
echo -e "${YELLOW}Testing environment:${NC}"
echo -e "  - RUNE_INTEGRATION_TESTS: ${RUNE_INTEGRATION_TESTS}"
echo -e "  - RUNE_INTEGRATION_DOCKER_HOST: ${RUNE_INTEGRATION_DOCKER_HOST:-default}"
echo -e "  - Go version: $(go version)"

# Run integration tests with the integration build tag
echo -e "${YELLOW}Running integration tests...${NC}"
go test -tags=integration ./test/integration/... -v

TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}All integration tests passed!${NC}"
else
    echo -e "${RED}Some integration tests failed.${NC}"
fi

exit $TEST_EXIT_CODE 