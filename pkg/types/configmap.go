package types

import "time"

// ConfigMap represents a piece of non-sensitive configuration data.
type ConfigMap struct {
	// Unique identifier for the config
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the config
	Name string `json:"name" yaml:"name"`

	// Namespace the config belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Configuration data (not encrypted)
	Data map[string]string `json:"data" yaml:"data"`

	// Current version number
	Version int `json:"version" yaml:"version"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}
