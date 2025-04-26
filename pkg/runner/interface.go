// Package runner provides interfaces and implementations for managing service instances.
package runner

import (
	"context"
	"io"
	"time"

	"github.com/rzbill/rune/pkg/types"
)

// Runner defines the interface for service runners, which are responsible for
// managing the lifecycle of service instances (containers, processes, etc.).
type Runner interface {
	// Create creates a new service instance but does not start it.
	Create(ctx context.Context, instance *types.Instance) error

	// Start starts an existing service instance.
	Start(ctx context.Context, instanceID string) error

	// Stop stops a running service instance.
	Stop(ctx context.Context, instanceID string, timeout time.Duration) error

	// Remove removes a service instance.
	Remove(ctx context.Context, instanceID string, force bool) error

	// GetLogs retrieves logs from a service instance.
	GetLogs(ctx context.Context, instanceID string, options LogOptions) (io.ReadCloser, error)

	// Status retrieves the current status of a service instance.
	Status(ctx context.Context, instanceID string) (types.InstanceStatus, error)

	// List lists all service instances managed by this runner.
	List(ctx context.Context) ([]*types.Instance, error)
}

// LogOptions defines options for retrieving logs.
type LogOptions struct {
	// Follow indicates whether to follow the log output (like tail -f).
	Follow bool

	// Tail indicates the number of lines to show from the end of the logs (0 for all).
	Tail int

	// Since shows logs since a specific timestamp.
	Since time.Time

	// Until shows logs until a specific timestamp.
	Until time.Time

	// Timestamps indicates whether to include timestamps.
	Timestamps bool
}

// InstanceStatus extends types.InstanceStatus with additional details needed by runners.
type InstanceStatus struct {
	// State is the current state of the instance.
	State types.InstanceStatus

	// ContainerID is the ID of the container (if applicable).
	ContainerID string

	// InstanceID is the ID of the Rune instance.
	InstanceID string

	// CreatedAt is when the instance was created.
	CreatedAt time.Time

	// StartedAt is when the instance was started.
	StartedAt time.Time

	// FinishedAt is when the instance finished or failed.
	FinishedAt time.Time

	// ExitCode is the exit code if the instance has stopped.
	ExitCode int

	// ErrorMessage contains any error information.
	ErrorMessage string
}
