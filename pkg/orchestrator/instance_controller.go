package orchestrator

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/manager"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

type InstanceRestartReason string

const (
	InstanceRestartReasonManual             InstanceRestartReason = "manual"
	InstanceRestartReasonHealthCheckFailure InstanceRestartReason = "health-check-failure"
	InstanceRestartReasonUpdate             InstanceRestartReason = "update"
	InstanceRestartReasonFailure            InstanceRestartReason = "failure"
)

// InstanceController manages instance lifecycle
type InstanceController interface {
	// CreateInstance creates a new instance for a service
	CreateInstance(ctx context.Context, service *types.Service, instanceName string) (*types.Instance, error)

	// RecreateInstance recreates an instance
	RecreateInstance(ctx context.Context, service *types.Service, instance *types.Instance) (*types.Instance, error)

	// UpdateInstance updates an existing instance
	UpdateInstance(ctx context.Context, service *types.Service, instance *types.Instance) error

	// DeleteInstance removes an instance
	DeleteInstance(ctx context.Context, instance *types.Instance) error

	// GetInstanceStatus gets the current status of an instance
	GetInstanceStatus(ctx context.Context, instance *types.Instance) (*InstanceStatusInfo, error)

	// GetInstanceLogs gets logs for an instance
	GetInstanceLogs(ctx context.Context, instance *types.Instance, opts LogOptions) (io.ReadCloser, error)

	// RestartInstance restarts an instance with respect to the service's restart policy
	RestartInstance(ctx context.Context, instance *types.Instance, reason InstanceRestartReason) error

	// Exec executes a command in a running instance
	// Returns an ExecStream for bidirectional communication
	Exec(ctx context.Context, instance *types.Instance, options ExecOptions) (ExecStream, error)
}

// instanceController implements the InstanceController interface
type instanceController struct {
	store         store.Store
	runnerManager manager.IRunnerManager
	logger        log.Logger
}

// NewInstanceController creates a new instance controller
func NewInstanceController(store store.Store, runnerManager manager.IRunnerManager, logger log.Logger) InstanceController {
	return &instanceController{
		store:         store,
		runnerManager: runnerManager,
		logger:        logger.WithComponent("instance-controller"),
	}
}

// CreateInstance creates a new instance for a service
// This would be simplified to only handle the pure creation case
func (c *instanceController) CreateInstance(ctx context.Context, service *types.Service, instanceName string) (*types.Instance, error) {
	c.logger.Info("Creating new instance",
		log.Str("service", service.Name),
		log.Str("namespace", service.Namespace),
		log.Str("instance", instanceName))

	// Create instance object
	instance := &types.Instance{
		ID:        uuid.New().String(),
		Name:      instanceName,
		Namespace: service.Namespace,
		ServiceID: service.ID,
		NodeID:    "local",
		Status:    types.InstanceStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save instance to store
	if err := c.store.Create(ctx, types.ResourceTypeInstance, service.Namespace, instance.Name, instance); err != nil {
		return nil, fmt.Errorf("failed to create instance in store: %w", err)
	}

	// Create instance based on runtime
	serviceRunner, err := c.runnerManager.GetServiceRunner(service)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner for service: %w", err)
	}

	// Set the runner type for the instance
	instance.Runner = serviceRunner.Type()

	// Build environment variables using the proper function from envvar.go
	envVars := buildEnvVars(service, instance)
	c.logger.Debug("Prepared environment variables",
		log.Str("instance", instanceName),
		log.Int("env_var_count", len(envVars)))

	// Set environment variables in the instance
	instance.Environment = envVars

	// Store original image for future compatibility checks
	if service.Image != "" {
		if instance.Environment == nil {
			instance.Environment = make(map[string]string)
		}
		instance.Environment["RUNE_ORIGINAL_IMAGE"] = service.Image
	}

	// Update instance with pending status
	instance.Status = types.InstanceStatusStarting
	if err := c.store.Update(ctx, types.ResourceTypeInstance, service.Namespace, instanceName, instance); err != nil {
		c.logger.Error("Failed to update instance status",
			log.Str("instance", instance.ID),
			log.Err(err))
	}

	// Create the instance using the runner
	if err := serviceRunner.Create(ctx, instance); err != nil {
		instance.Status = types.InstanceStatusFailed
		if updateErr := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); updateErr != nil {
			c.logger.Error("Failed to update instance status after create failure",
				log.Str("instance", instance.ID),
				log.Err(updateErr))
		}
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	// Start the instance
	if err := serviceRunner.Start(ctx, instance); err != nil {
		instance.Status = types.InstanceStatusFailed
		if updateErr := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); updateErr != nil {
			c.logger.Error("Failed to update instance status after start failure",
				log.Str("instance", instance.ID),
				log.Err(updateErr))
		}
		return nil, fmt.Errorf("failed to start instance: %w", err)
	}

	// Update instance with running status
	instance.Status = types.InstanceStatusRunning
	instance.StatusMessage = "Created successfully"
	if err := c.store.Update(ctx, types.ResourceTypeInstance, service.Namespace, instanceName, instance); err != nil {
		c.logger.Error("Failed to update instance status",
			log.Str("instance", instance.ID),
			log.Err(err))
	}

	return instance, nil
}

// RecreateInstance destroys an existing instance and creates a new one with the same name
func (c *instanceController) RecreateInstance(ctx context.Context, service *types.Service, existingInstance *types.Instance) (*types.Instance, error) {
	instanceName := existingInstance.Name
	c.logger.Info("Recreating instance",
		log.Str("service", service.Name),
		log.Str("namespace", service.Namespace),
		log.Str("instance", instanceName))

	// Delete the existing instance
	if err := c.DeleteInstance(ctx, existingInstance); err != nil {
		return nil, fmt.Errorf("failed to delete instance for recreation: %w", err)
	}

	// Create a new instance
	return c.CreateInstance(ctx, service, instanceName)
}

// UpdateInstance updates an existing instance
func (c *instanceController) UpdateInstance(ctx context.Context, service *types.Service, instance *types.Instance) error {
	c.logger.Debug("Checking instance for updates",
		log.Str("instance", instance.ID),
		log.Str("service", service.Name))

	// Get current runner for this instance
	runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return fmt.Errorf("failed to get runner for instance: %w", err)
	}

	// Check if the instance is compatible with the current service definition
	isCompatible, reason := c.isInstanceCompatibleWithService(ctx, instance, service)
	if !isCompatible {
		c.logger.Info("Instance is not compatible with current service definition, recreation required",
			log.Str("instance", instance.ID),
			log.Str("service", service.Name),
			log.Str("reason", reason))

		// For now, we'll stop and return an error indicating that recreation is needed
		// The caller should handle the recreation
		return fmt.Errorf("instance %s requires recreation to update: %s", instance.ID, reason)
	}

	// For compatible instances, we can apply in-place updates
	// First check if instance is running
	status, err := runner.Status(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to get instance status: %w", err)
	}

	// Check if the instance is in a state that can be updated
	if status != types.InstanceStatusRunning {
		c.logger.Info("Instance is not in a state that can be updated in-place",
			log.Str("instance", instance.ID),
			log.Str("currentStatus", string(status)))
		return fmt.Errorf("instance %s is in state %s and cannot be updated in-place", instance.ID, status)
	}

	// Apply updates to the instance object
	instanceUpdated := false

	// Update environment variables (only adding/modifying, not removing)
	envVarsUpdated := false
	envVars := buildEnvVars(service, instance)
	for key, value := range envVars {
		// Skip internal RUNE environment variables for comparison
		if len(key) > 5 && key[:5] == "RUNE_" {
			continue
		}

		// Check if this is a new or changed env var
		currentValue, exists := instance.Environment[key]
		if !exists || currentValue != value {
			if instance.Environment == nil {
				instance.Environment = make(map[string]string)
			}
			instance.Environment[key] = value
			envVarsUpdated = true
		}
	}

	if envVarsUpdated {
		c.logger.Debug("Environment variables updated",
			log.Str("instance", instance.ID))
		instanceUpdated = true
	}

	// Update status message if needed
	if instance.StatusMessage == "" || instance.StatusMessage == "Created" {
		instance.StatusMessage = "Updated"
		instanceUpdated = true
	}

	// Update timestamp
	instance.UpdatedAt = time.Now()
	instanceUpdated = true

	// If we've made any updates to the instance object, save it back to the store
	if instanceUpdated {
		if err := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); err != nil {
			return fmt.Errorf("failed to update instance in store: %w", err)
		}
		c.logger.Info("Instance updated successfully",
			log.Str("instance", instance.ID),
			log.Str("service", service.Name))
	} else {
		c.logger.Debug("No changes needed for instance",
			log.Str("instance", instance.ID))
	}

	return nil
}

// DeleteInstance removes an instance
func (c *instanceController) DeleteInstance(ctx context.Context, instance *types.Instance) error {
	c.logger.Info("Deleting instance",
		log.Str("instance", instance.ID))

	c.logger.Info("deleting instance", log.Json("instance", instance))

	runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return fmt.Errorf("failed to get runner for instance: %w", err)
	}

	// Track failures separately for better error reporting
	failedToStop := false
	failedToRemove := false

	// Try to stop and remove with runner
	if err := runner.Stop(ctx, instance, 10*time.Second); err != nil {
		c.logger.Debug("Failed to stop instance with runner",
			log.Str("instance", instance.ID),
			log.Err(err))
		failedToStop = true
	}

	if err := runner.Remove(ctx, instance, true); err != nil {
		c.logger.Debug("Failed to remove instance with runner",
			log.Str("instance", instance.ID),
			log.Err(err))
		failedToRemove = true
	}

	// If both operations failed, return a more descriptive error
	if failedToStop && failedToRemove {
		return fmt.Errorf("failed to both stop and remove instance")
	}

	if failedToStop {
		return fmt.Errorf("failed to stop instance")
	}

	if failedToRemove {
		return fmt.Errorf("failed to remove instance")
	}

	c.logger.Info("instance deleted successfully", log.Json("instance", instance))

	// Update instance status in store
	instance.Status = types.InstanceStatusStopped
	if err := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); err != nil {
		c.logger.Error("Failed to update instance status",
			log.Str("instance", instance.ID),
			log.Err(err))
	}

	return nil
}

// GetInstanceStatus gets the current status of an instance
func (c *instanceController) GetInstanceStatus(ctx context.Context, instance *types.Instance) (*InstanceStatusInfo, error) {
	// For now, we'll try to get status from both runners and use the first one that succeeds
	runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner for instance: %w", err)
	}

	status, err := runner.Status(ctx, instance)
	if err == nil {
		// Assuming Status returns a string representing the state
		return &InstanceStatusInfo{
			State:      status,
			InstanceID: instance.ID,
			NodeID:     instance.NodeID,
			CreatedAt:  instance.CreatedAt,
		}, nil
	}

	// If runner failed, return the instance status from the store
	return &InstanceStatusInfo{
		State:      instance.Status,
		InstanceID: instance.ID,
		NodeID:     instance.NodeID,
		CreatedAt:  instance.CreatedAt,
	}, nil
}

// GetInstanceLogs gets logs for an instance
func (c *instanceController) GetInstanceLogs(ctx context.Context, instance *types.Instance, opts LogOptions) (io.ReadCloser, error) {
	// Try to get logs from both runners and use the first one that succeeds
	_runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner for instance: %w", err)
	}

	logs, err := _runner.GetLogs(ctx, instance, runner.LogOptions{
		Follow:     opts.Follow,
		Since:      opts.Since,
		Until:      opts.Until,
		Tail:       opts.Tail,
		Timestamps: opts.Timestamps,
	})
	if err == nil {
		return logs, nil
	}

	return nil, fmt.Errorf("failed to get logs for instance %s: %w", instance.ID, err)
}

// Exec executes a command in a running instance
func (c *instanceController) Exec(ctx context.Context, instance *types.Instance, options ExecOptions) (ExecStream, error) {
	c.logger.Debug("Executing command in instance",
		log.Str("instance", instance.ID),
		log.Str("command", strings.Join(options.Command, " ")))

	// Get runner for the instance
	_runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner for instance: %w", err)
	}

	// Convert orchestrator exec options to runner exec options
	runnerOptions := runner.ExecOptions{
		Command:        options.Command,
		Env:            options.Env,
		WorkingDir:     options.WorkingDir,
		TTY:            options.TTY,
		TerminalWidth:  options.TerminalWidth,
		TerminalHeight: options.TerminalHeight,
	}

	// Create exec session with the runner
	execStream, err := _runner.Exec(ctx, instance, runnerOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command in instance %s: %w", instance.ID, err)
	}

	return execStreamAdapter{execStream}, nil
}

// execStreamAdapter adapts runner.ExecStream to orchestrator.ExecStream
type execStreamAdapter struct {
	runner.ExecStream
}

// buildEnvVars prepares environment variables for an instance
func buildEnvVars(service *types.Service, instance *types.Instance) map[string]string {
	envVars := make(map[string]string)

	// Add service-defined environment variables
	for key, value := range service.Env {
		envVars[key] = value
	}

	// Add built-in environment variables
	envVars["RUNE_SERVICE_NAME"] = service.Name
	envVars["RUNE_SERVICE_NAMESPACE"] = service.Namespace
	envVars["RUNE_INSTANCE_ID"] = instance.ID

	return envVars
}

// RestartInstance restarts an instance with respect to the service's restart policy
func (c *instanceController) RestartInstance(ctx context.Context, instance *types.Instance, reason InstanceRestartReason) error {
	c.logger.Info("Restarting instance",
		log.Str("instance", instance.ID),
		log.Str("reason", string(reason)))

	// Get the service to check its restart policy
	var service types.Service
	if err := c.store.Get(ctx, types.ResourceTypeService, instance.Namespace, instance.ServiceID, &service); err != nil {
		return fmt.Errorf("failed to get service for restart policy: %w", err)
	}

	// Manual restarts always override any policy
	if reason == InstanceRestartReasonManual {
		c.logger.Info("Manual restart requested, overriding restart policy",
			log.Str("instance", instance.ID))
	} else {
		// Check restart policy for non-manual restarts
		restartPolicy := types.RestartPolicyAlways // Default to Always
		if service.RestartPolicy != "" {
			restartPolicy = service.RestartPolicy
		}

		// Implement restart policy
		switch restartPolicy {
		case types.RestartPolicyNever:
			// No automatic restarts allowed
			c.logger.Info("Skipping restart due to 'Never' policy",
				log.Str("instance", instance.ID),
				log.Str("reason", string(reason)))
			return nil

		case types.RestartPolicyOnFailure:
			// Only restart if the reason is a failure or health check issue
			isFailureRelated := reason == InstanceRestartReasonFailure || reason == InstanceRestartReasonHealthCheckFailure
			if !isFailureRelated {
				c.logger.Info("Skipping restart due to 'OnFailure' policy with non-failure reason",
					log.Str("instance", instance.ID),
					log.Str("reason", string(reason)))
				return nil
			}
		}
	}

	// Get the appropriate runner
	runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return fmt.Errorf("failed to get runner for restart: %w", err)
	}

	// Update instance status to restarting
	instance.Status = types.InstanceStatusStarting
	instance.StatusMessage = fmt.Sprintf("Restarting due to: %s", reason)
	instance.UpdatedAt = time.Now()

	if err := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); err != nil {
		c.logger.Error("Failed to update instance status before restart",
			log.Str("instance", instance.ID),
			log.Err(err))
		// Continue anyway
	}

	// Stop the instance first
	stopTimeout := 10 * time.Second
	if err := runner.Stop(ctx, instance, stopTimeout); err != nil {
		c.logger.Warn("Failed to stop instance gracefully, will force restart",
			log.Str("instance", instance.ID),
			log.Err(err))

		// Try to forcefully remove if stop fails
		if err := runner.Remove(ctx, instance, true); err != nil {
			c.logger.Error("Failed to remove instance during restart",
				log.Str("instance", instance.ID),
				log.Err(err))
			// Continue anyway
		}
	}

	// Create the instance again
	if err := runner.Create(ctx, instance); err != nil {
		instance.Status = types.InstanceStatusFailed
		instance.StatusMessage = fmt.Sprintf("Failed to recreate: %v", err)
		if updateErr := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); updateErr != nil {
			c.logger.Error("Failed to update instance status after create failure",
				log.Str("instance", instance.ID),
				log.Err(updateErr))
		}
		return fmt.Errorf("failed to recreate instance: %w", err)
	}

	// Start the instance
	if err := runner.Start(ctx, instance); err != nil {
		instance.Status = types.InstanceStatusFailed
		instance.StatusMessage = fmt.Sprintf("Failed to start: %v", err)
		if updateErr := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); updateErr != nil {
			c.logger.Error("Failed to update instance status after start failure",
				log.Str("instance", instance.ID),
				log.Err(updateErr))
		}
		return fmt.Errorf("failed to start instance: %w", err)
	}

	// Update instance with running status
	instance.Status = types.InstanceStatusRunning
	instance.StatusMessage = "Restarted successfully"
	instance.UpdatedAt = time.Now()

	if err := c.store.Update(ctx, types.ResourceTypeInstance, instance.Namespace, instance.Name, instance); err != nil {
		c.logger.Error("Failed to update instance status after restart",
			log.Str("instance", instance.ID),
			log.Err(err))
	}

	c.logger.Info("Instance restarted successfully",
		log.Str("instance", instance.ID))

	return nil
}

// isInstanceCompatibleWithService checks if an instance is compatible with a service
func (c *instanceController) isInstanceCompatibleWithService(ctx context.Context, instance *types.Instance, service *types.Service) (bool, string) {
	// Check if the instance belongs to the correct service
	if instance.ServiceID != service.ID {
		return false, "instance belongs to different service"
	}

	// Check if the instance is in a failed state
	if instance.Status == types.InstanceStatusFailed ||
		instance.Status == types.InstanceStatusExited ||
		instance.Status == types.InstanceStatusUnknown {
		return false, fmt.Sprintf("instance is in failed state: %s", string(instance.Status))
	}

	// Get the current runner for the instance
	runner, err := c.runnerManager.GetInstanceRunner(instance)
	if err != nil {
		return false, fmt.Sprintf("failed to get runner: %v", err)
	}

	// Check if the instance still exists in the runner
	status, err := runner.Status(ctx, instance)
	if err != nil {
		return false, fmt.Sprintf("instance not found in runner: %v", err)
	}

	// If instance exists but is in a terminal state, it's incompatible
	if status == types.InstanceStatusExited || status == types.InstanceStatusFailed {
		return false, "instance is in terminal state in the runner"
	}

	// For Docker-based instances, perform additional checks
	if instance.ContainerID != "" && service.Runtime == "docker" {
		// Check if image has changed
		// This would require the Instance to store the image it was created with
		if instance.Environment != nil {
			// Look for stored image information in the environment
			if originalImage, ok := instance.Environment["RUNE_ORIGINAL_IMAGE"]; ok {
				if originalImage != service.Image {
					return false, fmt.Sprintf("image changed: %s -> %s", originalImage, service.Image)
				}
			} else {
				// If we can't determine the original image, be cautious and recreate
				c.logger.Debug("Cannot determine original image for instance, assuming incompatible")
				return false, "cannot determine original image"
			}
		}

		// Check for significant resource changes
		if service.Resources.CPU.Limit != "" || service.Resources.Memory.Limit != "" {
			// If instance doesn't have resources configured but service does
			if instance.Resources == nil ||
				(instance.Resources.CPU.Limit != service.Resources.CPU.Limit) ||
				(instance.Resources.Memory.Limit != service.Resources.Memory.Limit) {
				return false, "resource requirements changed"
			}
		}

		// Check for port mapping changes
		// This is more complex and would need to compare port configurations

		// Check for significant environment changes
		// Partial environment changes might be fine, but essential vars should match
		if service.Env != nil && len(service.Env) > 0 {
			for key, value := range service.Env {
				// Skip RUNE internal environment variables
				if len(key) > 5 && key[:5] == "RUNE_" {
					continue
				}

				instanceValue, exists := instance.Environment[key]
				if !exists || instanceValue != value {
					return false, fmt.Sprintf("environment variable %s changed or missing", key)
				}
			}
		}
	}

	// For process-based instances, perform process-specific checks
	if instance.PID != 0 && service.Runtime == "process" {
		// Check command consistency
		if instance.Process != nil && service.Process != nil {
			if instance.Process.Command != service.Process.Command ||
				!areStringSlicesEqual(instance.Process.Args, service.Process.Args) {
				return false, "process command or arguments changed"
			}

			// Check for working directory changes
			if instance.Process.WorkingDir != service.Process.WorkingDir {
				return false, "process working directory changed"
			}
		}
	}

	// If we get here, the instance is compatible
	return true, ""
}

// areStringSlicesEqual checks if two string slices are equal
func areStringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}
