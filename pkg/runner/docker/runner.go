// Package docker provides a Docker-based implementation of the runner interface.
package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/types"
	runetypes "github.com/rzbill/rune/pkg/types"
)

// DockerConfig holds Docker runner configuration options
type DockerConfig struct {
	// APIVersion is the Docker API version to use
	// If empty, auto-negotiation will be used
	APIVersion string

	// FallbackAPIVersion is used when auto-negotiation fails
	// Default is "1.43" which is widely compatible
	FallbackAPIVersion string

	// Timeout for API version negotiation in seconds
	NegotiationTimeoutSeconds int

	// Permissions for mounts created on the host before binding into containers
	// If zero, sensible defaults will be used.
	SecretDirMode  os.FileMode
	SecretFileMode os.FileMode
	ConfigDirMode  os.FileMode
	ConfigFileMode os.FileMode
}

// DefaultDockerConfig returns the default Docker configuration
func DefaultDockerConfig() *DockerConfig {
	return &DockerConfig{
		APIVersion:                "",     // Empty means use auto-negotiation
		FallbackAPIVersion:        "1.43", // Fallback to a widely compatible version
		NegotiationTimeoutSeconds: 3,
		// Defaults optimized for Docker Desktop on macOS/Windows where
		// FUSE permissions can otherwise block container access.
		SecretDirMode:  0o755,
		SecretFileMode: 0o444,
		ConfigDirMode:  0o755,
		ConfigFileMode: 0o644,
	}
}

// Validate that DockerRunner implements the runner.Runner interface
var _ runner.Runner = &DockerRunner{}

// DockerRunner implements the runner.Runner interface for Docker.
type DockerRunner struct {
	client *client.Client
	logger log.Logger
	config *DockerConfig
}

func (r *DockerRunner) Type() types.RunnerType {
	return types.RunnerTypeDocker
}

// NewDockerRunner creates a new DockerRunner instance.
func NewDockerRunner(logger log.Logger) (*DockerRunner, error) {
	// Use default configuration
	config := DefaultDockerConfig()
	return NewDockerRunnerWithConfig(logger, config)
}

// NewDockerRunnerWithConfig creates a new DockerRunner with specific configuration.
func NewDockerRunnerWithConfig(logger log.Logger, config *DockerConfig) (*DockerRunner, error) {
	// Default to use global logger if none provided
	if logger == nil {
		logger = log.GetDefaultLogger().WithComponent("docker-runner")
	}

	// Use default config if none provided
	if config == nil {
		config = DefaultDockerConfig()
	}

	// Create a client with the appropriate API version
	client, err := createClientWithVersionHandling(logger, config)
	if err != nil {
		return nil, err
	}

	return &DockerRunner{
		client: client,
		logger: logger,
		config: config,
	}, nil
}

// createClientWithVersionHandling creates a Docker client with appropriate API version handling
func createClientWithVersionHandling(logger log.Logger, config *DockerConfig) (*client.Client, error) {
	// Create Docker client with default configuration
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// If a specific API version is configured, use it
	if config.APIVersion != "" {
		logger.Info("Using specified Docker API version",
			log.Str("api_version", config.APIVersion))

		dockerClient, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithVersion(config.APIVersion),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Docker client with version %s: %w", config.APIVersion, err)
		}

		return dockerClient, nil
	}

	// Otherwise try to negotiate API version safely with timeout
	negotiationTimeout := time.Duration(config.NegotiationTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), negotiationTimeout)
	defer cancel()

	dockerClient.NegotiateAPIVersion(ctx)
	clientVersion := dockerClient.ClientVersion()
	logger.Info("Using negotiated Docker API version", log.Str("api_version", clientVersion))

	// Verify the version works by doing a ping test
	if err := verifyClientCompatibility(dockerClient, clientVersion, config.FallbackAPIVersion, logger); err != nil {
		return nil, err
	}

	return dockerClient, nil
}

// verifyClientCompatibility checks if the Docker client is compatible with the server
// and falls back to a compatible version if needed
func verifyClientCompatibility(dockerClient *client.Client, clientVersion, fallbackVersion string, logger log.Logger) error {
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()

	_, err := dockerClient.Ping(pingCtx)

	// Check for version mismatch errors
	if err != nil && strings.Contains(err.Error(), "client version") &&
		strings.Contains(err.Error(), "too new") {
		// If we get version mismatch error, use the fallback version
		logger.Warn("Docker API version mismatch, falling back to compatibility version",
			log.Str("current_version", clientVersion),
			log.Str("fallback_version", fallbackVersion),
			log.Err(err))

		// Create new client with fallback version
		newClient, err := client.NewClientWithOpts(
			client.FromEnv,
			client.WithVersion(fallbackVersion),
		)
		if err != nil {
			return fmt.Errorf("failed to create Docker client with fallback version %s: %w",
				fallbackVersion, err)
		}

		// Replace the original client with the fallback client
		*dockerClient = *newClient
	} else if err != nil {
		// If there's a non-version error, log it but continue
		logger.Warn("Docker ping error (continuing anyway)", log.Err(err))
	}

	return nil
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

	// Configure host config with mounts
	hostConfig := &container.HostConfig{}

	// Handle secret and config mounts
	if instance.Metadata != nil {
		// Process secret mounts
		if len(instance.Metadata.SecretMounts) > 0 {
			secretMounts, err := r.prepareSecretMounts(instance.Metadata.SecretMounts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to prepare secret mounts: %w", err)
			}
			hostConfig.Mounts = append(hostConfig.Mounts, secretMounts...)
		}

		// Process config mounts
		if len(instance.Metadata.ConfigmapMounts) > 0 {
			configMounts, err := r.prepareConfigmapsMounts(instance.Metadata.ConfigmapMounts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to prepare config mounts: %w", err)
			}
			hostConfig.Mounts = append(hostConfig.Mounts, configMounts...)
		}
	}

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

// prepareSecretMounts creates temporary files and Docker mounts for secret mounts
func (r *DockerRunner) prepareSecretMounts(secretMounts []types.ResolvedSecretMount) ([]mount.Mount, error) {
	var mounts []mount.Mount

	for _, secretMount := range secretMounts {
		// Create a temporary directory for this mount
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("rune-secret-%s-", secretMount.Name))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory for secret mount %s: %w", secretMount.Name, err)
		}
		// Adjust directory permissions to allow Docker Desktop to stat/bind mount
		// Keep files themselves locked down (0600) while directory is world-executable for traversal
		dirMode := r.config.SecretDirMode
		if dirMode == 0 {
			dirMode = 0o755
		}
		_ = os.Chmod(tempDir, dirMode)

		// Create files for each secret key
		for key, value := range secretMount.Data {
			// Determine the file path
			var filePath string
			if len(secretMount.Items) > 0 {
				// Check if there's a specific path mapping for this key
				for _, item := range secretMount.Items {
					if item.Key == key {
						filePath = filepath.Join(tempDir, item.Path)
						break
					}
				}
				// If no specific mapping, use the key name
				if filePath == "" {
					filePath = filepath.Join(tempDir, key)
				}
			} else {
				// No specific mapping, use the key name
				filePath = filepath.Join(tempDir, key)
			}

			// Ensure subdirectories exist if path contains directories
			// Use 0755 so the container user (often not the host owner due to Docker Desktop FUSE) can traverse
			parentMode := r.config.SecretDirMode
			if parentMode == 0 {
				parentMode = 0o755
			}
			if err := os.MkdirAll(filepath.Dir(filePath), parentMode); err != nil {
				os.RemoveAll(tempDir)
				return nil, fmt.Errorf("failed to create directory for secret file %s: %w", filePath, err)
			}
			// Create the file with the secret value
			fileMode := r.config.SecretFileMode
			if fileMode == 0 {
				fileMode = 0o444
			}
			if err := os.WriteFile(filePath, []byte(value), fileMode); err != nil {
				os.RemoveAll(tempDir) // Clean up on error
				return nil, fmt.Errorf("failed to write secret file %s: %w", filePath, err)
			}
		}

		// Create Docker mount
		dockerMount := mount.Mount{
			Type:        mount.TypeBind,
			Source:      tempDir,
			Target:      secretMount.MountPath,
			ReadOnly:    true,
			BindOptions: &mount.BindOptions{},
		}

		mounts = append(mounts, dockerMount)
	}

	return mounts, nil
}

// prepareConfigmapsMounts creates temporary files and Docker mounts for config mounts
func (r *DockerRunner) prepareConfigmapsMounts(configMounts []types.ResolvedConfigmapMount) ([]mount.Mount, error) {
	var mounts []mount.Mount

	for _, configMount := range configMounts {
		// Create a temporary directory for this mount
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("rune-config-%s-", configMount.Name))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory for config mount %s: %w", configMount.Name, err)
		}
		// Adjust directory permissions to allow Docker Desktop to stat/bind mount
		dirMode := r.config.ConfigDirMode
		if dirMode == 0 {
			dirMode = 0o755
		}
		_ = os.Chmod(tempDir, dirMode)

		// Create files for each config key
		for key, value := range configMount.Data {
			// Determine the file path
			var filePath string
			if len(configMount.Items) > 0 {
				// Check if there's a specific path mapping for this key
				for _, item := range configMount.Items {
					if item.Key == key {
						filePath = filepath.Join(tempDir, item.Path)
						break
					}
				}
				// If no specific mapping, use the key name
				if filePath == "" {
					filePath = filepath.Join(tempDir, key)
				}
			} else {
				// No specific mapping, use the key name
				filePath = filepath.Join(tempDir, key)
			}

			// Ensure subdirectories exist if path contains directories
			// Use 0755 so the container user can traverse
			parentMode := r.config.ConfigDirMode
			if parentMode == 0 {
				parentMode = 0o755
			}
			if err := os.MkdirAll(filepath.Dir(filePath), parentMode); err != nil {
				os.RemoveAll(tempDir)
				return nil, fmt.Errorf("failed to create directory for config file %s: %w", filePath, err)
			}
			// Create the file with the config value
			fileMode := r.config.ConfigFileMode
			if fileMode == 0 {
				fileMode = 0o644
			}
			if err := os.WriteFile(filePath, []byte(value), fileMode); err != nil {
				os.RemoveAll(tempDir) // Clean up on error
				return nil, fmt.Errorf("failed to write config file %s: %w", filePath, err)
			}
		}

		// Create Docker mount
		dockerMount := mount.Mount{
			Type:        mount.TypeBind,
			Source:      tempDir,
			Target:      configMount.MountPath,
			ReadOnly:    true,
			BindOptions: &mount.BindOptions{},
		}

		mounts = append(mounts, dockerMount)
	}

	return mounts, nil
}
