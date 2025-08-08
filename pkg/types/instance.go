package types

import (
	"time"
)

// Instance represents a running copy of a service.
type Instance struct {
	NamespacedResource `json:"-" yaml:"-"`

	// Runner type for the instance
	Runner RunnerType `json:"runner" yaml:"runner"`

	// Unique identifier for the instance
	ID string `json:"id" yaml:"id"`

	// Namespace of the instance
	Namespace string `json:"namespace" yaml:"namespace"`

	// Human-readable name for the instance
	Name string `json:"name" yaml:"name"`

	// ID of the service this instance belongs to
	ServiceID string `json:"serviceId" yaml:"serviceId"`

	// Name of the service this instance belongs to
	ServiceName string `json:"serviceName" yaml:"serviceName"`

	// ID of the node running this instance
	NodeID string `json:"nodeId" yaml:"nodeId"`

	// IP address assigned to this instance
	IP string `json:"ip" yaml:"ip"`

	// Status of the instance
	Status InstanceStatus `json:"status" yaml:"status"`

	// Detailed status information
	StatusMessage string `json:"statusMessage,omitempty" yaml:"statusMessage,omitempty"`

	// Container ID or process ID
	ContainerID string `json:"containerId,omitempty" yaml:"containerId,omitempty"`

	// Process ID for process runner
	PID int `json:"pid,omitempty" yaml:"pid,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`

	// Process-specific configuration for process runner
	Process *ProcessSpec `json:"process,omitempty" yaml:"process,omitempty"`

	// Execution configuration for commands and environment
	Exec *Exec `json:"exec,omitempty" yaml:"exec,omitempty"`

	// Resources requirements for the instance
	Resources *Resources `json:"resources,omitempty" yaml:"resources,omitempty"`

	// Environment variables for the instance
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`

	// Metadata contains additional information about the instance
	// Use for storing system properties that aren't part of the core spec
	Metadata *InstanceMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (i *Instance) GetResourceType() ResourceType {
	return ResourceTypeInstance
}

// InstanceMetadata contains additional information about the instance
type InstanceMetadata struct {
	// Image is the image that the instance is running
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// ServiceGeneration is the generation of the service that the instance belongs to
	ServiceGeneration int64 `json:"serviceGeneration,omitempty" yaml:"serviceGeneration,omitempty"`

	// DeletionTimestamp is the timestamp when the instance was marked for deletion
	DeletionTimestamp *time.Time `json:"deletionTimestamp,omitempty" yaml:"deletionTimestamp,omitempty"`

	// RestartCount is the number of times this instance has been restarted
	RestartCount int `json:"restartCount,omitempty" yaml:"restartCount,omitempty"`
}

// Exec represents execution configuration for a command
type Exec struct {
	// Command to execute
	Command []string `json:"command" yaml:"command"`

	// Environment variables
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// InstanceStatus represents the current status of an instance.
type InstanceStatus string

const (
	// InstanceStatusPending indicates the instance is being created.
	InstanceStatusPending InstanceStatus = "Pending"

	// InstanceStatusRunning indicates the instance is running.
	InstanceStatusRunning InstanceStatus = "Running"

	// InstanceStatusStopped indicates the instance has stopped.
	InstanceStatusStopped InstanceStatus = "Stopped"

	// InstanceStatusFailed indicates the instance failed to start or crashed.
	InstanceStatusFailed InstanceStatus = "Failed"

	// InstanceStatusDeleted indicates the instance has been marked for deletion
	// but is retained in the store for a period before garbage collection.
	InstanceStatusDeleted InstanceStatus = "Deleted"

	// Process runner specific statuses
	InstanceStatusCreated  InstanceStatus = "Created"
	InstanceStatusStarting InstanceStatus = "Starting"
	InstanceStatusExited   InstanceStatus = "Exited"
	InstanceStatusUnknown  InstanceStatus = "Unknown"
)

// InstanceStatusInfo contains information about an instance's status
type InstanceStatusInfo struct {
	Status        InstanceStatus
	StatusMessage string
	InstanceID    string
	NodeID        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Validate validates the instance configuration.
func (i *Instance) Validate() error {
	if i.ID == "" {
		return NewValidationError("instance ID is required")
	}

	if i.Namespace == "" {
		return NewValidationError("instance namespace is required")
	}

	if i.Name == "" {
		return NewValidationError("instance name is required")
	}

	if i.ServiceID == "" {
		return NewValidationError("instance serviceId is required")
	}

	if i.NodeID == "" {
		return NewValidationError("instance nodeId is required")
	}

	return nil
}
