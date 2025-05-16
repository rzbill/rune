package types

import "time"

// ScalingMode defines how the scaling operation should be performed
type ScalingMode string

const (
	// ScalingModeImmediate indicates scaling should happen immediately
	ScalingModeImmediate ScalingMode = "immediate"

	// ScalingModeGradual indicates scaling should happen gradually
	ScalingModeGradual ScalingMode = "gradual"
)

// ScalingOperationStatus represents the status of a scaling operation
type ScalingOperationStatus string

const (
	// ScalingOperationStatusInProgress indicates the operation is still running
	ScalingOperationStatusInProgress ScalingOperationStatus = "in_progress"

	// ScalingOperationStatusCompleted indicates the operation completed successfully
	ScalingOperationStatusCompleted ScalingOperationStatus = "completed"

	// ScalingOperationStatusFailed indicates the operation failed
	ScalingOperationStatusFailed ScalingOperationStatus = "failed"

	// ScalingOperationStatusCancelled indicates the operation was cancelled
	ScalingOperationStatusCancelled ScalingOperationStatus = "cancelled"
)

// ScalingOperationMetadata represents metadata for a scaling operation
type ScalingOperationMetadata struct {
	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// ScalingOperation represents a scaling operation for a service
type ScalingOperation struct {
	// Unique identifier for the operation
	ID string `json:"id"`

	// Namespace the service belongs to
	Namespace string `json:"namespace"`

	// Name of the service being scaled
	ServiceName string `json:"service_name"`

	// Current scale when operation started
	CurrentScale int `json:"current_scale"`

	// Target scale to reach
	TargetScale int `json:"target_scale"`

	// Step size for gradual scaling
	StepSize int `json:"step_size"`

	// Interval between steps in seconds
	Interval int `json:"interval"`

	// Time the operation started
	StartTime time.Time `json:"start_time"`

	// Time the operation ended (if completed/failed/cancelled)
	EndTime time.Time `json:"end_time,omitempty"`

	// Current status of the operation
	Status ScalingOperationStatus `json:"status"`

	// Mode of scaling (immediate or gradual)
	Mode ScalingMode `json:"mode"`

	// Reason for failure/cancellation if applicable
	FailureReason string `json:"failure_reason,omitempty"`
}

// GetMetadata returns the resource metadata.
func (s *ScalingOperation) GetMetadata() *ScalingOperationMetadata {
	return &ScalingOperationMetadata{
		CreatedAt: s.StartTime,
		UpdatedAt: s.EndTime,
	}
}

// ScalingOperationParams encapsulates parameters for creating a scaling operation
type ScalingOperationParams struct {
	// Current scale of the service
	CurrentScale int

	// Target scale to reach
	TargetScale int

	// Step size for gradual scaling
	StepSize int

	// Interval between steps in seconds
	IntervalSeconds int

	// Whether to use gradual scaling
	IsGradual bool
}
