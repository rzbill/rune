package types

import (
	"time"

	"github.com/google/uuid"
)

// Namespace represents a logical boundary for isolation and scoping of resources.
type Namespace struct {
	// Unique identifier for the namespace
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the namespace
	Name string `json:"name" yaml:"name"`

	// Optional description for the namespace
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Optional resource quotas for this namespace
	Quota *NamespaceQuota `json:"quota,omitempty" yaml:"quota,omitempty"`

	// Labels attached to the namespace for organization
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// NamespaceQuota represents resource quotas for a namespace.
type NamespaceQuota struct {
	// Maximum CPU allocation for all resources in the namespace (millicores)
	CPU int64 `json:"cpu,omitempty" yaml:"cpu,omitempty"`

	// Maximum memory allocation for all resources in the namespace (bytes)
	Memory int64 `json:"memory,omitempty" yaml:"memory,omitempty"`

	// Maximum storage allocation for all resources in the namespace (bytes)
	Storage int64 `json:"storage,omitempty" yaml:"storage,omitempty"`

	// Maximum number of services allowed in the namespace
	Services int `json:"services,omitempty" yaml:"services,omitempty"`

	// Maximum number of jobs allowed in the namespace
	Jobs int `json:"jobs,omitempty" yaml:"jobs,omitempty"`

	// Maximum number of instances (all services combined) allowed in the namespace
	Instances int `json:"instances,omitempty" yaml:"instances,omitempty"`
}

// NamespaceSpec represents the YAML specification for a namespace.
type NamespaceSpec struct {
	// Human-readable name for the namespace (required)
	Name string `json:"name" yaml:"name"`

	// Optional description for the namespace
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Optional resource quotas for this namespace
	Quota *NamespaceQuota `json:"quota,omitempty" yaml:"quota,omitempty"`

	// Labels attached to the namespace for organization
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// Validate checks if a namespace specification is valid.
func (n *NamespaceSpec) Validate() error {
	if n.Name == "" {
		return NewValidationError("namespace name is required")
	}

	// Validate that the name follows DNS label conventions (for DNS compatibility)
	// This is a simplistic validation - should be enhanced with proper regex
	for _, c := range n.Name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return NewValidationError("namespace name must consist of lowercase alphanumeric characters or '-'")
		}
	}

	// Check that it doesn't start or end with a hyphen
	if len(n.Name) > 0 && (n.Name[0] == '-' || n.Name[len(n.Name)-1] == '-') {
		return NewValidationError("namespace name must not start or end with '-'")
	}

	return nil
}

// ToNamespace converts a NamespaceSpec to a Namespace.
func (n *NamespaceSpec) ToNamespace() (*Namespace, error) {
	// Validate
	if err := n.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()

	return &Namespace{
		ID:          uuid.New().String(),
		Name:        n.Name,
		Description: n.Description,
		Quota:       n.Quota,
		Labels:      n.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
