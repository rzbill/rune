#!/bin/bash
set -e

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Print header
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}         Rune Unit Tests Runner         ${NC}"
echo -e "${GREEN}=========================================${NC}"

# Display testing environment
echo -e "${YELLOW}Testing environment:${NC}"
echo -e "  - Go version: $(go version)"

# Run unit tests with the unit build tag
echo -e "${YELLOW}Running unit tests...${NC}"
go test -tags=unit ./... -v

TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}All unit tests passed!${NC}"
else
    echo -e "${RED}Some unit tests failed.${NC}"
fi

exit $TEST_EXIT_CODE 