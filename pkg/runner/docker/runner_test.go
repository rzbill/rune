package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/rzbill/rune/pkg/log"
	runetypes "github.com/rzbill/rune/pkg/types"
)

// skipIfDockerUnavailable skips the test if Docker is not available.
func skipIfDockerUnavailable(t *testing.T) *client.Client {
	// Skip if SKIP_DOCKER_TESTS is set
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Skipping Docker tests")
	}

	// Try to create a Docker client to see if Docker is available
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Skip("Docker is not available:", err)
	}

	// Ping the Docker daemon to ensure it's responding
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		t.Skip("Docker daemon is not responding:", err)
	}

	return cli
}

// TestNewDockerRunner tests the creation of a new Docker runner.
func TestNewDockerRunner(t *testing.T) {
	skipIfDockerUnavailable(t)

	logger := log.NewLogger(
		log.WithLevel(log.InfoLevel),
		log.WithFormatter(&log.TextFormatter{
			DisableColors: true, // Disable colors for tests
		}),
	)
	r, err := NewDockerRunner("test", logger)

	if err != nil {
		t.Fatalf("Failed to create Docker runner: %v", err)
	}

	if r == nil {
		t.Fatal("Docker runner is nil")
	}

	if r.namespace != "test" {
		t.Errorf("Expected namespace 'test', got '%s'", r.namespace)
	}
}

// TestDockerRunnerLifecycle tests the full lifecycle of an instance using a real Docker daemon.
// This is an integration test and can be skipped if Docker is not available.
func TestDockerRunnerLifecycle(t *testing.T) {
	skipIfDockerUnavailable(t)

	// Create a unique test namespace based on timestamp
	namespace := "test-" + time.Now().Format("20060102150405")
	logger := log.NewLogger(
		log.WithLevel(log.InfoLevel),
		log.WithFormatter(&log.TextFormatter{
			DisableColors: true, // Disable colors for tests
		}),
	)
	r, err := NewDockerRunner(namespace, logger)
	if err != nil {
		t.Fatalf("Failed to create Docker runner: %v", err)
	}

	// Use a simple, small image that starts quickly
	instance := &runetypes.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		ServiceID: "test-service",
		NodeID:    "test-node",
	}

	// Create a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure cleanup on test completion
	defer func() {
		// Attempt to remove the container regardless of test outcome
		r.Remove(ctx, instance.ID, true)
	}()

	// Test creating a container
	err = r.Create(ctx, instance)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Verify container was created and ID was set
	if instance.ContainerID == "" {
		t.Fatal("Container ID was not set after Create")
	}

	// Test starting the container
	err = r.Start(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Wait for container to fully start
	time.Sleep(2 * time.Second)

	// Check status
	status, err := r.Status(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	if status != runetypes.InstanceStatusRunning {
		t.Errorf("Expected status %s, got %s", runetypes.InstanceStatusRunning, status)
	}

	// Test stopping the container
	err = r.Stop(ctx, instance.ID, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}

	// Verify container stopped
	status, err = r.Status(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	if status != runetypes.InstanceStatusStopped {
		t.Errorf("Expected status %s, got %s", runetypes.InstanceStatusStopped, status)
	}

	// Test removing the container
	err = r.Remove(ctx, instance.ID, false)
	if err != nil {
		t.Fatalf("Failed to remove container: %v", err)
	}

	// Verify container was removed by checking that getting its status now fails
	_, err = r.Status(ctx, instance.ID)
	if err == nil {
		t.Fatal("Expected error getting status of removed container, got nil")
	}
}

// TestDockerLogReader tests the log reader implementation.
func TestDockerLogReader(t *testing.T) {
	// Create a buffer with the test content and prepend a mock Docker log header
	testContent := "Test log content"

	// Create a buffer to hold the Docker-formatted log
	var buf bytes.Buffer

	// First, write a fake Docker log header:
	// - Byte 0: Stream type (1 for stdout)
	// - Bytes 1-3: Padding (zeros)
	// - Bytes 4-7: Frame size (length of our test content as uint32 big endian)
	header := []byte{1, 0, 0, 0, 0, 0, 0, 0}
	// Set the content length in big endian
	binary.BigEndian.PutUint32(header[4:], uint32(len(testContent)))
	buf.Write(header)

	// Then write the content
	buf.WriteString(testContent)

	// Create a reader using our mock Docker log format
	mockLog := io.NopCloser(&buf)
	logReader := newLogReader(mockLog)
	defer logReader.Close()

	// Read the content
	result := make([]byte, len(testContent))
	n, err := io.ReadFull(logReader, result)

	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		t.Fatalf("Failed to read from log reader: %v", err)
	}

	// Check the content
	actualContent := string(result[:n])
	if actualContent != testContent {
		t.Errorf("Expected '%s', got: '%s'", testContent, actualContent)
	}
}

// TestGetLogs has been moved to test/integration/docker/logs_test.go
// as it requires a Docker image and should be executed as part of integration tests.

// TestList tests listing of containers.
func TestList(t *testing.T) {
	skipIfDockerUnavailable(t)

	// Create a unique test namespace based on timestamp
	namespace := "list-test-" + time.Now().Format("20060102150405")
	logger := log.NewLogger(
		log.WithLevel(log.InfoLevel),
		log.WithFormatter(&log.TextFormatter{
			DisableColors: true, // Disable colors for tests
		}),
	)
	r, err := NewDockerRunner(namespace, logger)
	if err != nil {
		t.Fatalf("Failed to create Docker runner: %v", err)
	}

	// Create a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a couple of containers
	instance1 := &runetypes.Instance{
		ID:        "list-test-1",
		Name:      "list-test-1",
		ServiceID: "list-test-service",
		NodeID:    "test-node",
	}

	instance2 := &runetypes.Instance{
		ID:        "list-test-2",
		Name:      "list-test-2",
		ServiceID: "list-test-service",
		NodeID:    "test-node",
	}

	// Ensure cleanup on test completion
	defer func() {
		r.Remove(ctx, instance1.ID, true)
		r.Remove(ctx, instance2.ID, true)
	}()

	// Create the containers
	err = r.Create(ctx, instance1)
	if err != nil {
		t.Fatalf("Failed to create container 1: %v", err)
	}

	err = r.Create(ctx, instance2)
	if err != nil {
		t.Fatalf("Failed to create container 2: %v", err)
	}

	// List containers
	instances, err := r.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	// Check that we found at least our 2 containers
	found1, found2 := false, false
	for _, inst := range instances {
		if inst.ID == instance1.ID {
			found1 = true
		}
		if inst.ID == instance2.ID {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("Not all test containers were found in the list. Found container 1: %v, Found container 2: %v", found1, found2)
	}
}
