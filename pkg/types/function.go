package types

import (
	"time"

	"github.com/google/uuid"
)

// Function represents a managed piece of user code and its execution environment.
type Function struct {
	// Unique identifier for the function
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the function
	Name string `json:"name" yaml:"name"`

	// Namespace the function belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Specification of the function
	Spec FunctionSpec `json:"spec" yaml:"spec"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// FunctionSpec defines the execution environment and code for a Function.
type FunctionSpec struct {
	// Runtime environment (e.g., python3.9, nodejs16, go1.18)
	Runtime string `json:"runtime" yaml:"runtime"`

	// Entrypoint handler (format depends on runtime, e.g., "main.handler")
	Handler string `json:"handler" yaml:"handler"`

	// Maximum execution time (e.g., "30s", "2m")
	Timeout string `json:"timeout" yaml:"timeout"`

	// Memory limit (e.g., "128Mi", "1Gi")
	MemoryLimit string `json:"memoryLimit" yaml:"memoryLimit"`

	// CPU limit (e.g., "100m", "0.5")
	CPULimit string `json:"cpuLimit,omitempty" yaml:"cpuLimit,omitempty"`

	// Function code
	Code FunctionCode `json:"code" yaml:"code"`

	// Static environment variables
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// Network policy for the function
	NetworkPolicy *FunctionNetworkPolicy `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
}

// FunctionCode contains the function's source code, which can be specified inline or as a reference.
type FunctionCode struct {
	// Inline source code
	Inline string `json:"inline,omitempty" yaml:"inline,omitempty"`

	// Reference to code stored elsewhere (future use)
	// Reference string `json:"reference,omitempty" yaml:"reference,omitempty"`
}

// FunctionNetworkPolicy defines what network connections a function is allowed to make.
type FunctionNetworkPolicy struct {
	// Allowed outbound connections
	Allow []FunctionNetworkRule `json:"allow" yaml:"allow"`
}

// FunctionNetworkRule defines a single allow rule for network connectivity.
type FunctionNetworkRule struct {
	// Target host or IP
	Host string `json:"host" yaml:"host"`

	// Allowed ports and protocols (e.g., ["80/tcp", "443/tcp"])
	Ports []string `json:"ports" yaml:"ports"`
}

// FunctionRunStatus represents the execution status of a function run.
type FunctionRunStatus string

const (
	// FunctionRunStatusPending indicates the function execution is queued
	FunctionRunStatusPending FunctionRunStatus = "Pending"

	// FunctionRunStatusRunning indicates the function is currently executing
	FunctionRunStatusRunning FunctionRunStatus = "Running"

	// FunctionRunStatusSucceeded indicates the function completed successfully
	FunctionRunStatusSucceeded FunctionRunStatus = "Succeeded"

	// FunctionRunStatusFailed indicates the function execution failed
	FunctionRunStatusFailed FunctionRunStatus = "Failed"

	// FunctionRunStatusTimedOut indicates the function execution timed out
	FunctionRunStatusTimedOut FunctionRunStatus = "TimedOut"
)

// FunctionRun represents a single execution of a Function.
type FunctionRun struct {
	// Unique identifier for the function run
	ID string `json:"id" yaml:"id"`

	// ID of the function that was executed
	FunctionID string `json:"functionId" yaml:"functionId"`

	// Namespace the function belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Input payload (JSON-serialized)
	Input string `json:"input,omitempty" yaml:"input,omitempty"`

	// Output result (JSON-serialized)
	Output string `json:"output,omitempty" yaml:"output,omitempty"`

	// Execution logs (stdout/stderr)
	Logs string `json:"logs,omitempty" yaml:"logs,omitempty"`

	// Execution status
	Status FunctionRunStatus `json:"status" yaml:"status"`

	// Error message (if status is Failed or TimedOut)
	Error string `json:"error,omitempty" yaml:"error,omitempty"`

	// Start time
	StartTime *time.Time `json:"startTime,omitempty" yaml:"startTime,omitempty"`

	// End time
	EndTime *time.Time `json:"endTime,omitempty" yaml:"endTime,omitempty"`

	// Duration in milliseconds
	DurationMs int64 `json:"durationMs,omitempty" yaml:"durationMs,omitempty"`

	// Invoker information (component/user that triggered the execution)
	InvokedBy string `json:"invokedBy,omitempty" yaml:"invokedBy,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
}

// FunctionInvocationRequest represents a request to invoke a function.
type FunctionInvocationRequest struct {
	// Payload for the function (JSON-serializable object)
	Payload map[string]interface{} `json:"payload" yaml:"payload"`

	// Timeout override (optional)
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// Asynchronous execution flag
	Async bool `json:"async,omitempty" yaml:"async,omitempty"`
}

// FunctionSpec represents the YAML specification for a function.
type FunctionDefinition struct {
	// Function metadata and spec
	Function struct {
		// Human-readable name for the function (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the function belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Function specification
		Spec FunctionSpec `json:"spec" yaml:"spec"`
	} `json:"function" yaml:"function"`
}

// Validate checks if a function definition is valid.
func (f *FunctionDefinition) Validate() error {
	if f.Function.Name == "" {
		return NewValidationError("function name is required")
	}

	if f.Function.Spec.Runtime == "" {
		return NewValidationError("function runtime is required")
	}

	if f.Function.Spec.Handler == "" {
		return NewValidationError("function handler is required")
	}

	if f.Function.Spec.Code.Inline == "" {
		return NewValidationError("function code is required")
	}

	// Validate that network policy allows only specific outbound connections
	if f.Function.Spec.NetworkPolicy != nil {
		for _, rule := range f.Function.Spec.NetworkPolicy.Allow {
			if rule.Host == "" {
				return NewValidationError("network policy host is required")
			}
			if len(rule.Ports) == 0 {
				return NewValidationError("network policy ports are required")
			}
		}
	}

	return nil
}

// ToFunction converts a FunctionDefinition to a Function.
func (f *FunctionDefinition) ToFunction() (*Function, error) {
	// Validate
	if err := f.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := f.Function.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &Function{
		ID:        uuid.New().String(),
		Name:      f.Function.Name,
		Namespace: namespace,
		Spec:      f.Function.Spec,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
