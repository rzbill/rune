// Package docker provides a Docker-based implementation of the runner interface.
package docker

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/types"
	runetypes "github.com/rzbill/rune/pkg/types"
)

// Validate that DockerRunner implements the runner.Runner interface
var _ runner.Runner = &DockerRunner{}

// DockerRunner implements the runner.Runner interface for Docker.
type DockerRunner struct {
	client *client.Client
	logger log.Logger
}

func (r *DockerRunner) Type() types.RunnerType {
	return types.RunnerTypeDocker
}

// NewDockerRunner creates a new DockerRunner instance.
func NewDockerRunner(logger log.Logger) (*DockerRunner, error) {
	// Default to use global logger if none provided
	if logger == nil {
		logger = log.GetDefaultLogger().WithComponent("docker-runner")
	}

	// Create Docker client with default configuration
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Set API version to negotiate the highest supported version
	client.NegotiateAPIVersion(context.Background())

	return &DockerRunner{
		client: client,
		logger: logger,
	}, nil
}

// Create creates a new container but does not start it.
func (r *DockerRunner) Create(ctx context.Context, instance *runetypes.Instance) error {
	if instance == nil {
		return fmt.Errorf("invalid instance: nil pointer")
	}

	// Create container config and host config
	containerConfig, hostConfig, err := r.instanceToContainerConfig(instance)
	if err != nil {
		return fmt.Errorf("failed to create container configuration: %w", err)
	}

	// Pull the image first
	if err := r.pullImage(ctx, containerConfig.Image); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", containerConfig.Image, err)
	}

	// Create the container
	resp, err := r.client.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,           // No network config for now
		nil,           // No platform config
		instance.Name, // Use instance name as container name
	)

	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Update instance with container ID
	instance.ContainerID = resp.ID

	r.logger.Info("Created container for instance",
		log.Str("container_id", resp.ID),
		log.Str("instance_id", instance.ID))

	return nil
}

// Start starts an existing container.
func (r *DockerRunner) Start(ctx context.Context, instance *runetypes.Instance) error {
	containerID, err := r.getContainerID(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get container ID: %w", err)
	}

	// Start the container - using empty StartOptions as we don't need any special configuration
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	r.logger.Info("Started container for instance",
		log.Str("container_id", containerID),
		log.Str("instance_id", instance.ID),
		log.Str("instance_name", instance.Name),
		log.Str("instance_status", string(instance.Status)))

	return nil
}

// Stop stops a running container.
func (r *DockerRunner) Stop(ctx context.Context, instance *runetypes.Instance, timeout time.Duration) error {
	containerID, err := r.getContainerID(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get container ID: %w", err)
	}

	// Convert timeout to seconds
	timeoutSeconds := int(timeout.Seconds())

	// Stop the container
	if err := r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSeconds}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	r.logger.Info("Stopped container for instance",
		log.Str("container_id", containerID),
		log.Str("instance_id", instance.ID))

	return nil
}

// Remove removes a container.
func (r *DockerRunner) Remove(ctx context.Context, instance *runetypes.Instance, force bool) error {
	containerID, err := r.getContainerID(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get container ID: %w", err)
	}

	// Remove the container
	options := container.RemoveOptions{
		Force: force,
	}

	if err := r.client.ContainerRemove(ctx, containerID, options); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	r.logger.Info("Removed container for instance",
		log.Str("container_id", containerID),
		log.Str("instance_id", instance.ID))

	return nil
}

// GetLogs retrieves logs from a container.
func (r *DockerRunner) GetLogs(ctx context.Context, instance *runetypes.Instance, options runner.LogOptions) (io.ReadCloser, error) {
	containerID, err := r.getContainerID(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get container ID: %w", err)
	}

	// Convert our options to Docker options
	since := options.Since.Format(time.RFC3339Nano)
	until := options.Until.Format(time.RFC3339Nano)

	// Convert tail option to string
	var tail string
	if options.Tail <= 0 {
		tail = "all"
	} else {
		tail = fmt.Sprintf("%d", options.Tail)
	}

	// Prepare Docker API log options
	logsOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     options.Follow,
		Timestamps: options.Timestamps,
		Since:      since,
		Until:      until,
		Tail:       tail,
	}

	// Get logs from Docker
	logs, err := r.client.ContainerLogs(ctx, containerID, logsOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	// Docker multiplexes stdout and stderr, so we need to demultiplex
	return newLogReader(logs), nil
}

// Status retrieves the current status of a container.
func (r *DockerRunner) Status(ctx context.Context, instance *runetypes.Instance) (runetypes.InstanceStatus, error) {
	containerID, err := r.getContainerID(ctx, instance)
	if err != nil {
		return "", fmt.Errorf("failed to get container ID: %w", err)
	}

	// Get container information
	container, err := r.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	// Map Docker state to Rune instance status
	if container.State.Running {
		return runetypes.InstanceStatusRunning, nil
	} else if container.State.ExitCode != 0 {
		return runetypes.InstanceStatusFailed, nil
	} else {
		return runetypes.InstanceStatusStopped, nil
	}
}

// List lists all service instances managed by this runner.
func (r *DockerRunner) List(ctx context.Context, namespace string) ([]*runetypes.Instance, error) {
	// Filter containers managed by this runner
	args := filters.NewArgs(
		filters.Arg("label", "rune.managed=true"),
	)

	if namespace != "" {
		args.Add("label", "rune.namespace="+namespace)
	}

	containers, err := r.client.ContainerList(ctx, container.ListOptions{
		All:     true, // Include stopped containers
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Convert Docker API types to Rune types
	instances := make([]*runetypes.Instance, 0, len(containers))

	for _, c := range containers {
		// Extract instance information from container labels
		instanceID := c.Labels["rune.instance.id"]
		if instanceID == "" {
			r.logger.Warn("Found container without instance ID",
				log.Str("container_id", c.ID),
				log.Str("container_name", c.Names[0]))
			continue
		}

		// Create instance object
		instance := &runetypes.Instance{
			ID:          instanceID,
			ContainerID: c.ID,
			Name:        c.Names[0][1:], // Remove leading slash from container name
			ServiceID:   c.Labels["rune.service.id"],
			NodeID:      "local", // Assume local node for now
		}

		// Set status based on container state
		switch c.State {
		case "running":
			instance.Status = runetypes.InstanceStatusRunning
		case "exited":
			// For exited containers, we need to inspect to get the exit code
			inspect, err := r.client.ContainerInspect(ctx, c.ID)
			if err == nil && inspect.State.ExitCode != 0 {
				instance.Status = runetypes.InstanceStatusFailed
			} else {
				instance.Status = runetypes.InstanceStatusStopped
			}
		case "created":
			instance.Status = runetypes.InstanceStatusPending
		default:
			instance.Status = ""
		}

		instances = append(instances, instance)
	}

	return instances, nil
}

// Exec creates an interactive exec session with a running container.
func (r *DockerRunner) Exec(ctx context.Context, instance *runetypes.Instance, options runner.ExecOptions) (runner.ExecStream, error) {
	containerID, err := r.getContainerID(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get container ID: %w", err)
	}

	// Check if the container is running
	container, err := r.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	if !container.State.Running {
		return nil, fmt.Errorf("container is not running")
	}

	// Create the exec stream
	execStream, err := NewDockerExecStream(ctx, r.client, containerID, instance.ID, options, r.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec stream: %w", err)
	}

	return execStream, nil
}

// getContainerID gets the container ID for an instance.
func (r *DockerRunner) getContainerID(ctx context.Context, instance *runetypes.Instance) (string, error) {
	// Try to get the container directly from the instance ID using labels
	args := filters.NewArgs(
		filters.Arg("label", "rune.managed=true"),
		filters.Arg("label", "rune.instance.id="+instance.ID),
	)

	if instance.Namespace != "" {
		args.Add("label", "rune.namespace="+instance.Namespace)
	}

	containers, err := r.client.ContainerList(ctx, container.ListOptions{
		All:     true, // Include stopped containers
		Filters: args,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("no container found for instance ID: %s", instance.ID)
	}

	if len(containers) > 1 {
		r.logger.Warn("Multiple containers found for instance ID",
			log.Str("instance_id", instance.ID),
			log.Int("container_count", len(containers)))
	}

	// Return the first matching container
	return containers[0].ID, nil
}

// pullImage pulls an image from the registry if it doesn't exist locally
func (r *DockerRunner) pullImage(ctx context.Context, image string) error {
	// Check if we already have the image
	_, _, err := r.client.ImageInspectWithRaw(ctx, image)
	if err == nil {
		// Image exists locally
		return nil
	}

	r.logger.Info("Pulling Docker image", log.Str("image", image))

	// Pull the image
	reader, err := r.client.ImagePull(ctx, image, imageTypes.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Read the output to complete the pull
	_, err = io.Copy(io.Discard, reader)
	return err
}

// instanceToContainerConfig converts a Rune instance to Docker container config.
func (r *DockerRunner) instanceToContainerConfig(instance *runetypes.Instance) (*container.Config, *container.HostConfig, error) {
	// Extract service ID from the instance
	serviceID := instance.ServiceID
	if serviceID == "" {
		return nil, nil, fmt.Errorf("service ID is required")
	}

	// Get the image from environment variables
	var image string
	if instance.Metadata != nil {
		image = instance.Metadata.Image
	}

	// Validate that we have an image
	if image == "" {
		return nil, nil, fmt.Errorf("no image specified for instance %s", instance.ID)
	}

	r.logger.Debug("Using image for instance",
		log.Str("instance", instance.ID),
		log.Str("image", image))

	// Configure the container
	containerConfig := &container.Config{
		Image: image,
		Labels: map[string]string{
			"rune.managed":      "true",
			"rune.namespace":    instance.Namespace,
			"rune.instance.id":  instance.ID,
			"rune.service.id":   serviceID,
			"rune.service.name": instance.Name,
		},
		Env: formatEnvVars(instance.Environment),
	}

	// Simple host config with no special settings
	hostConfig := &container.HostConfig{}

	return containerConfig, hostConfig, nil
}

// formatEnvVars formats a map of environment variables into a slice of "key=value" strings.
func formatEnvVars(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}
