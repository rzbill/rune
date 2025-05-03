package types

import (
	"io"
	"time"
)

// LogOptions defines options for retrieving logs
type LogOptions struct {
	Follow     bool
	Since      time.Time
	Until      time.Time
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

// ServiceStatusInfo contains a summary of a service's status
type ServiceStatusInfo struct {
	Status ServiceStatus

	// ObservedGeneration tracks which generation was last processed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`

	// DesiredInstances is the number of instances that should be running
	DesiredInstances int

	// RunningInstances is the number of instances that are currently running
	RunningInstances int
}

// ExecOptions defines options for executing a command in a running instance
type ExecOptions struct {
	// Command is the command to execute
	Command []string

	// Env is a map of environment variables to set for the command
	Env map[string]string

	// WorkingDir is the working directory for the command
	WorkingDir string

	// TTY indicates whether to allocate a pseudo-TTY
	TTY bool

	// TerminalWidth is the initial width of the terminal
	TerminalWidth uint32

	// TerminalHeight is the initial height of the terminal
	TerminalHeight uint32
}

// ExecStream provides bidirectional communication with an exec session
type ExecStream interface {
	// Write writes data to the standard input of the process
	Write(p []byte) (n int, err error)

	// Read reads data from the standard output of the process
	Read(p []byte) (n int, err error)

	// Stderr provides access to the standard error stream of the process
	Stderr() io.Reader

	// ResizeTerminal resizes the terminal (if TTY was enabled)
	ResizeTerminal(width, height uint32) error

	// Signal sends a signal to the process
	Signal(sigName string) error

	// ExitCode returns the exit code after the process has completed
	ExitCode() (int, error)

	// Close terminates the exec session and releases resources
	Close() error
}
