#!/bin/bash
set -e

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

COVERAGE_FILE="coverage_unit.out"

# Print header
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}         Rune Unit Tests Runner         ${NC}"
echo -e "${GREEN}=========================================${NC}"

# Display testing environment
echo -e "${YELLOW}Testing environment:${NC}"
echo -e "  - Go version: $(go version)"

# Get list of packages excluding tests/integrations
UNIT_PACKAGES=$(go list ./... | grep -v 'tests/integrations')

# Run unit tests and generate coverage report
echo -e "${YELLOW}Running unit tests...${NC}"
# Skip docker package integration-like tests unless explicitly enabled
export SKIP_DOCKER_TESTS=${SKIP_DOCKER_TESTS:-1}
go test -tags=unit $UNIT_PACKAGES -v -coverprofile=$COVERAGE_FILE

TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}All unit tests passed!${NC}"
    echo -e "${YELLOW}Coverage report written to ${COVERAGE_FILE}${NC}"
else
    echo -e "${RED}Some unit tests failed.${NC}"
fi

exit $TEST_EXIT_CODE
