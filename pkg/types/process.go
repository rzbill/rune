package types

// ProcessSpec defines how to run a service as a process
type ProcessSpec struct {
	// Command to run
	Command string `json:"command" yaml:"command"`

	// Command arguments
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// Working directory for the process
	WorkingDir string `json:"workingDir,omitempty" yaml:"workingDir,omitempty"`

	// Security settings
	SecurityContext *ProcessSecurityContext `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
}

// ProcessSecurityContext defines security settings for a process
type ProcessSecurityContext struct {
	// User to run as
	User string `json:"user,omitempty" yaml:"user,omitempty"`

	// Group to run as
	Group string `json:"group,omitempty" yaml:"group,omitempty"`

	// Run with read-only filesystem
	ReadOnlyFS bool `json:"readOnlyFS,omitempty" yaml:"readOnlyFS,omitempty"`

	// Linux capabilities to add
	Capabilities []string `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`

	// Allowed syscalls (seccomp)
	AllowedSyscalls []string `json:"allowedSyscalls,omitempty" yaml:"allowedSyscalls,omitempty"`

	// Denied syscalls (seccomp)
	DeniedSyscalls []string `json:"deniedSyscalls,omitempty" yaml:"deniedSyscalls,omitempty"`
}

// Validate validates the process specification
func (p *ProcessSpec) Validate() error {
	if p.Command == "" {
		return NewValidationError("process command is required")
	}

	return nil
}
