package orchestrator

import (
	"time"

	"github.com/rzbill/rune/pkg/types"
)

// LogOptions defines options for retrieving logs
type LogOptions struct {
	Follow     bool
	Since      time.Time
	Tail       int
	Timestamps bool
}

// InstanceActionType represents an action to perform on an instance
type InstanceActionType string

const (
	// InstanceActionCreate indicates an instance should be created
	InstanceActionCreate InstanceActionType = "create"

	// InstanceActionUpdate indicates an instance should be updated
	InstanceActionUpdate InstanceActionType = "update"

	// InstanceActionDelete indicates an instance should be deleted
	InstanceActionDelete InstanceActionType = "delete"
)

// InstanceAction represents a pending action for an instance
type InstanceAction struct {
	Type       InstanceActionType
	Service    string
	Namespace  string
	InstanceID string
	Timestamp  time.Time
}

// HealthCheckResult represents the result of a health check execution
type HealthCheckResult struct {
	Success    bool
	Message    string
	Duration   time.Duration
	CheckTime  time.Time
	InstanceID string
	CheckType  string
}

// InstanceHealthStatus represents the health status of an instance
type InstanceHealthStatus struct {
	InstanceID  string
	Liveness    bool
	Readiness   bool
	LastChecked time.Time
}

// InstanceStatus represents the status of an instance.
type InstanceStatus struct {
	InstanceID    string
	Status        string
	StatusMessage string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ServiceStatusInfo contains information about a service's status
type ServiceStatusInfo struct {
	State              types.ServiceStatus
	InstanceCount      int
	ReadyInstanceCount int
}

// InstanceStatusInfo contains information about an instance's status
type InstanceStatusInfo struct {
	State      types.InstanceStatus
	InstanceID string
	NodeID     string
	CreatedAt  time.Time
}
