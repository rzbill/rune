package helpers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerContainer "github.com/docker/docker/api/types/container"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerHelper provides utilities for working with Docker containers in tests
type DockerHelper struct {
	client *client.Client
}

// NewDockerHelper creates a new Docker helper instance
func NewDockerHelper() (*DockerHelper, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.43"), // Force compatibility with older Docker daemons
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerHelper{
		client: cli,
	}, nil
}

// Close closes the Docker client
func (h *DockerHelper) Close() error {
	return h.client.Close()
}

// EnsureImage ensures the specified image is available, pulling it if necessary
func (h *DockerHelper) EnsureImage(ctx context.Context, image string) error {
	_, _, err := h.client.ImageInspectWithRaw(ctx, image)
	if err == nil {
		// Image already exists
		return nil
	}

	if !client.IsErrNotFound(err) {
		return fmt.Errorf("error checking for image %s: %w", image, err)
	}

	// Pull the image
	out, err := h.client.ImagePull(ctx, image, imageTypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}
	defer out.Close()

	// Wait for the pull to complete
	_, err = io.Copy(io.Discard, out)
	if err != nil {
		return fmt.Errorf("error reading image pull output: %w", err)
	}

	return nil
}

// CreateContainer creates a new container
func (h *DockerHelper) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (string, error) {
	resp, err := h.client.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a container
func (h *DockerHelper) StartContainer(ctx context.Context, containerID string) error {
	return h.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a container
func (h *DockerHelper) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	timeoutSec := int(timeout.Seconds())
	return h.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSec})
}

// RemoveContainer removes a container
func (h *DockerHelper) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return h.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: force,
	})
}

// CleanupContainer stops and removes a container
func (h *DockerHelper) CleanupContainer(ctx context.Context, containerID string) {
	// Stop container with 5 second timeout
	timeout := 5 * time.Second
	timeoutSec := int(timeout.Seconds())

	err := h.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSec})
	if err != nil {
		fmt.Printf("Warning: Failed to stop container %s: %v\n", containerID, err)
	}

	// Remove container
	err = h.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		fmt.Printf("Warning: Failed to remove container %s: %v\n", containerID, err)
	}
}

// GetContainerLogs retrieves logs from a container
func (h *DockerHelper) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	reader, err := h.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	// Combine stdout and stderr
	return fmt.Sprintf("STDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String()), nil
}

// ExecCommand executes a command in a running container
func (h *DockerHelper) ExecCommand(ctx context.Context, containerID string, cmd []string) (string, error) {
	execConfig := dockerContainer.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create exec instance
	execResp, err := h.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec instance
	resp, err := h.client.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Get exec result
	inspectResp, err := h.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect exec: %w", err)
	}

	output := fmt.Sprintf("STDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

	// Check exit code
	if inspectResp.ExitCode != 0 {
		return output, fmt.Errorf("command failed with exit code %d", inspectResp.ExitCode)
	}

	return output, nil
}
