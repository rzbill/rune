#!/bin/bash
set -e

# Ensure Docker is running
if ! docker info &> /dev/null; then
    echo "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Clean up any previous test container
docker rm -f rune-test-container &> /dev/null || true

# Create a test container that keeps running
echo "Creating test container..."
CONTAINER_ID=$(docker run -d --name rune-test-container alpine:latest sleep 300)
echo "Container ID: $CONTAINER_ID"

# Start the Rune server in the background if not already running
if ! pgrep -f "./bin/runed" > /dev/null; then
    echo "Starting Rune server..."
    if [ ! -d "bin" ]; then
        echo "Building Rune server..."
        mkdir -p bin
        make -s build
    fi
    
    # Start the server in the background
    ./bin/runed &
    SERVER_PID=$!
    echo "Server started with PID: $SERVER_PID"
    
    # Give it time to start up
    sleep 2
else
    echo "Rune server already running"
fi

echo
echo "===== Rune Exec Client Demo ====="
echo

# Run the register container example to demonstrate the API
echo "STEP 1: Attempting to register the container"
echo "----------------------------------------"
echo "Running: go run examples/exec-client/register-container/main.go $CONTAINER_ID"
go run examples/exec-client/register-container/main.go $CONTAINER_ID
echo "----------------------------------------"

echo
echo "STEP 2: Testing the exec-client with container ID"
echo "----------------------------------------"
echo "Running: go run examples/exec-client/main.go $CONTAINER_ID ls -la"
go run examples/exec-client/main.go $CONTAINER_ID ls -la
echo "----------------------------------------"

echo
echo "NOTES:"
echo "1. The real workflow would involve properly registering the container with Rune first."
echo "2. This would be done using a combination of store operations and API calls."
echo "3. The current API expects the instance to exist in the store before StartInstance is called."
echo "4. In a production system, the Rune CLI would handle these steps transparently."
echo

# Clean up
echo "Cleaning up..."
docker rm -f rune-test-container

echo "Done!" 