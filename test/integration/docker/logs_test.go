//go:build integration
// +build integration

// go:build integration

package docker_integration

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rzbill/rune/pkg/log"
	runeRunner "github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/docker"
	runetypes "github.com/rzbill/rune/pkg/types"
	"github.com/rzbill/rune/test/integration/helpers"
)

// skipIfDockerUnavailable skips the test if Docker is not available.
func skipIfDockerUnavailable(t *testing.T) *client.Client {
	// Try to create a Docker client to see if Docker is available
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.43"), // Force compatibility with older Docker daemons
	)
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

// TestGetLogs tests the Docker runner GetLogs functionality.
func TestGetLogs(t *testing.T) {
	// Skip if Docker is not available
	cli := skipIfDockerUnavailable(t)

	// Create helper for Docker operations
	dockerHelper, err := helpers.NewDockerHelper()
	if err != nil {
		t.Fatalf("Failed to create Docker helper: %v", err)
	}
	defer dockerHelper.Close()

	// Setup context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Ensure Alpine image is available by pulling it
	image := helpers.AlpineImage
	err = dockerHelper.EnsureImage(ctx, image)
	if err != nil {
		t.Skipf("Failed to ensure image %s is available: %v - skipping test", image, err)
		return
	}

	// Create a unique test namespace based on timestamp
	namespace := "log-test-" + time.Now().Format("20060102150405")
	logger := log.NewLogger(
		log.WithLevel(log.InfoLevel),
		log.WithFormatter(&log.TextFormatter{
			DisableColors: true, // Disable colors for tests
		}),
	)

	// Create Docker runner
	runner, err := docker.NewDockerRunner(namespace, logger)
	if err != nil {
		t.Fatalf("Failed to create Docker runner: %v", err)
	}

	// Create instance to run in Docker
	instanceID := "log-test-instance"
	instance := &runetypes.Instance{
		ID:        instanceID,
		Name:      "log-test-instance",
		ServiceID: "log-test-service",
		NodeID:    "test-node",
	}

	// Create a container with a command that outputs logs
	containerConfig := &container.Config{
		Image: image,
		Cmd:   []string{"sh", "-c", "echo 'test log message 1'; sleep 1; echo 'test log message 2'; sleep 5"},
		Labels: map[string]string{
			"rune.managed":      "true",
			"rune.namespace":    namespace,
			"rune.instance.id":  instanceID,
			"rune.service.id":   instance.ServiceID,
			"rune.service.name": instance.Name,
		},
	}
	hostConfig := &container.HostConfig{}

	// Create container using the Docker client
	resp, err := cli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,           // No network config
		nil,           // No platform config
		instance.Name, // Use instance name as container name
	)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	containerID := resp.ID

	// Store container ID in instance for the runner to find it
	instance.ContainerID = containerID

	// Ensure container cleanup using DockerHelper instead of the runner
	defer dockerHelper.CleanupContainer(ctx, containerID)

	// Start the container
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Wait a bit for the container to produce logs
	time.Sleep(2 * time.Second)

	// Test 1: Get all logs
	t.Run("GetAllLogs", func(t *testing.T) {
		logs, err := runner.GetLogs(ctx, instanceID, runeRunner.LogOptions{
			Follow:     false,
			Tail:       0, // Get all logs
			Timestamps: false,
		})
		if err != nil {
			t.Fatalf("Failed to get logs: %v", err)
		}
		defer logs.Close()

		// Read logs
		buf, err := io.ReadAll(logs)
		if err != nil {
			t.Fatalf("Failed to read logs: %v", err)
		}

		logContent := string(buf)
		if !strings.Contains(logContent, "test log message 1") {
			t.Errorf("Expected log message 1 not found, got: %q", logContent)
		}
		if !strings.Contains(logContent, "test log message 2") {
			t.Errorf("Expected log message 2 not found, got: %q", logContent)
		}
	})

	// Test 2: Get limited logs
	t.Run("GetLimitedLogs", func(t *testing.T) {
		logs, err := runner.GetLogs(ctx, instanceID, runeRunner.LogOptions{
			Follow:     false,
			Tail:       1, // Only get the last line
			Timestamps: false,
		})
		if err != nil {
			t.Fatalf("Failed to get logs: %v", err)
		}
		defer logs.Close()

		// Read logs
		buf, err := io.ReadAll(logs)
		if err != nil {
			t.Fatalf("Failed to read logs: %v", err)
		}

		logContent := string(buf)
		if strings.Contains(logContent, "test log message 1") {
			t.Errorf("Unexpected log message 1 found with tail=1, got: %q", logContent)
		}
		if !strings.Contains(logContent, "test log message 2") {
			t.Errorf("Expected log message 2 not found, got: %q", logContent)
		}
	})
}
