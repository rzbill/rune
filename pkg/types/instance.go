package types

import (
	"time"
)

// Instance represents a running copy of a service.
type Instance struct {
	// Unique identifier for the instance
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the instance
	Name string `json:"name" yaml:"name"`

	// ID of the service this instance belongs to
	ServiceID string `json:"serviceId" yaml:"serviceId"`

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

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
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
)

// Validate validates the instance configuration.
func (i *Instance) Validate() error {
	if i.ID == "" {
		return NewValidationError("instance ID is required")
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
