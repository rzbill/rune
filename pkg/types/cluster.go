// Package types defines the core data structures for the Rune orchestration platform.
package types

import (
	"time"
)

// Cluster represents a collection of nodes running services.
type Cluster struct {
	// Unique identifier for the cluster
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the cluster
	Name string `json:"name" yaml:"name"`

	// Version of the cluster schema
	Version string `json:"version" yaml:"version"`

	// Nodes that are part of this cluster
	Nodes []Node `json:"nodes,omitempty" yaml:"nodes,omitempty"`

	// Services deployed in this cluster
	Services []Service `json:"services,omitempty" yaml:"services,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// Validate validates the cluster configuration.
func (c *Cluster) Validate() error {
	if c.ID == "" {
		return NewValidationError("cluster ID is required")
	}

	if c.Name == "" {
		return NewValidationError("cluster name is required")
	}

	return nil
}
