package types

import (
	"time"

	"github.com/google/uuid"
)

// Secret represents a securely stored piece of sensitive data.
type Secret struct {
	// Unique identifier for the secret
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the secret
	Name string `json:"name" yaml:"name"`

	// Namespace the secret belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Type of secret (static or dynamic)
	Type string `json:"type" yaml:"type"`

	// Static data (encrypted at rest, only for static secrets)
	Data map[string]string `json:"-" yaml:"-"` // Not serialized, stored encrypted separately

	// Engine configuration for dynamic secrets
	Engine *SecretEngine `json:"engine,omitempty" yaml:"engine,omitempty"`

	// Rotation configuration
	Rotation *RotationPolicy `json:"rotation,omitempty" yaml:"rotation,omitempty"`

	// Current version number
	Version int `json:"version" yaml:"version"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`

	// Last rotation timestamp
	LastRotated *time.Time `json:"lastRotated,omitempty" yaml:"lastRotated,omitempty"`
}

// SecretEngine defines a dynamic secret engine configuration.
type SecretEngine struct {
	// Name of the engine to use (references a SecretsEngine resource)
	Name string `json:"name" yaml:"name"`

	// Role or profile to use with the engine (engine-specific)
	Role string `json:"role,omitempty" yaml:"role,omitempty"`

	// Engine-specific configuration for this secret
	Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// RotationPolicy defines when and how to rotate a secret.
type RotationPolicy struct {
	// Interval between rotations (e.g., "30d", "90d")
	Interval string `json:"interval" yaml:"interval"`

	// Actions to take after rotation
	OnRotate []RotationAction `json:"onRotate,omitempty" yaml:"onRotate,omitempty"`
}

// RotationAction defines an action to take after secret rotation.
type RotationAction struct {
	// Services to reload (send SIGHUP)
	ReloadServices []string `json:"reloadServices,omitempty" yaml:"reloadServices,omitempty"`

	// Services to restart (rolling restart)
	RestartServices []string `json:"restartServices,omitempty" yaml:"restartServices,omitempty"`

	// Job to run
	RunJob string `json:"runJob,omitempty" yaml:"runJob,omitempty"`
}

// Config represents a piece of non-sensitive configuration data.
type Config struct {
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

// SecretsEngine represents a configured secrets engine.
type SecretsEngine struct {
	// Unique identifier for the engine
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the engine
	Name string `json:"name" yaml:"name"`

	// Namespace the engine belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Type of engine (builtin, function, plugin)
	Type string `json:"type" yaml:"type"`

	// For function-based engines, the reference to the function
	Function string `json:"function,omitempty" yaml:"function,omitempty"`

	// For plugin-based engines, the path to the plugin executable
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Engine-wide configuration
	Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// SecretSpec represents the YAML specification for a secret.
type SecretSpec struct {
	// Secret metadata and data
	Secret struct {
		// Human-readable name for the secret (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the secret belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Type of secret (static or dynamic)
		Type string `json:"type" yaml:"type"`

		// Secret data for static secrets (only for creation, not returned in GET)
		Data map[string]string `json:"data,omitempty" yaml:"data,omitempty"`

		// Single value convenience field (alternative to data.value for simple secrets)
		Value string `json:"value,omitempty" yaml:"value,omitempty"`

		// Base64-encoded value (for binary data)
		ValueBase64 string `json:"valueBase64,omitempty" yaml:"valueBase64,omitempty"`

		// Engine configuration for dynamic secrets
		Engine *SecretEngine `json:"engine,omitempty" yaml:"engine,omitempty"`

		// Rotation configuration
		Rotation *RotationPolicy `json:"rotation,omitempty" yaml:"rotation,omitempty"`
	} `json:"secret" yaml:"secret"`
}

// ConfigSpec represents the YAML specification for a config.
type ConfigSpec struct {
	// Config metadata and data
	Config struct {
		// Human-readable name for the config (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the config belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Config data
		Data map[string]string `json:"data" yaml:"data"`
	} `json:"config" yaml:"config"`
}

// SecretsEngineSpec represents the YAML specification for a secrets engine.
type SecretsEngineSpec struct {
	// Engine metadata and configuration
	SecretsEngine struct {
		// Human-readable name for the engine (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the engine belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Type of engine (required: builtin, function, plugin)
		Type string `json:"type" yaml:"type"`

		// For function-based engines, the reference to the function
		Function string `json:"function,omitempty" yaml:"function,omitempty"`

		// For plugin-based engines, the path to the plugin executable
		Path string `json:"path,omitempty" yaml:"path,omitempty"`

		// Engine-wide configuration
		Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	} `json:"secretsEngine" yaml:"secretsEngine"`
}

// Validate checks if a secret specification is valid.
func (s *SecretSpec) Validate() error {
	if s.Secret.Name == "" {
		return NewValidationError("secret name is required")
	}

	if s.Secret.Type != "static" && s.Secret.Type != "dynamic" {
		return NewValidationError("secret type must be 'static' or 'dynamic'")
	}

	if s.Secret.Type == "static" {
		// For static secrets, either data, value, or valueBase64 must be provided
		if len(s.Secret.Data) == 0 && s.Secret.Value == "" && s.Secret.ValueBase64 == "" {
			return NewValidationError("static secret must have data, value, or valueBase64")
		}
	} else if s.Secret.Type == "dynamic" {
		// For dynamic secrets, engine must be specified
		if s.Secret.Engine == nil {
			return NewValidationError("dynamic secret must have an engine configuration")
		}

		if s.Secret.Engine.Name == "" {
			return NewValidationError("dynamic secret engine name is required")
		}
	}

	return nil
}

// Validate checks if a config specification is valid.
func (c *ConfigSpec) Validate() error {
	if c.Config.Name == "" {
		return NewValidationError("config name is required")
	}

	if len(c.Config.Data) == 0 {
		return NewValidationError("config must have data")
	}

	return nil
}

// Validate checks if a secrets engine specification is valid.
func (s *SecretsEngineSpec) Validate() error {
	if s.SecretsEngine.Name == "" {
		return NewValidationError("secrets engine name is required")
	}

	if s.SecretsEngine.Type == "" {
		return NewValidationError("secrets engine type is required")
	}

	// Type-specific validation
	switch s.SecretsEngine.Type {
	case "function":
		if s.SecretsEngine.Function == "" {
			return NewValidationError("function-based secrets engine must specify a function")
		}
	case "plugin":
		if s.SecretsEngine.Path == "" {
			return NewValidationError("plugin-based secrets engine must specify a path")
		}
	case "builtin":
		// No additional validation for builtin engines
	default:
		return NewValidationError("unknown secrets engine type: " + s.SecretsEngine.Type)
	}

	return nil
}

// ToSecret converts a SecretSpec to a Secret.
func (s *SecretSpec) ToSecret() (*Secret, error) {
	// Validate
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := s.Secret.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	// Create the secret
	secret := &Secret{
		ID:        uuid.New().String(),
		Name:      s.Secret.Name,
		Namespace: namespace,
		Type:      s.Secret.Type,
		Engine:    s.Secret.Engine,
		Rotation:  s.Secret.Rotation,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// For static secrets, set the data map
	if s.Secret.Type == "static" {
		data := make(map[string]string)

		// Start with any values in the data map
		for k, v := range s.Secret.Data {
			data[k] = v
		}

		// If value field is set, add it
		if s.Secret.Value != "" {
			data["value"] = s.Secret.Value
		}

		// If valueBase64 field is set, add it
		// In a real implementation, we'd decode this and store the raw value
		if s.Secret.ValueBase64 != "" {
			data["valueBase64"] = s.Secret.ValueBase64
		}

		secret.Data = data
	}

	return secret, nil
}

// ToConfig converts a ConfigSpec to a Config.
func (c *ConfigSpec) ToConfig() (*Config, error) {
	// Validate
	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := c.Config.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &Config{
		ID:        uuid.New().String(),
		Name:      c.Config.Name,
		Namespace: namespace,
		Data:      c.Config.Data,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ToSecretsEngine converts a SecretsEngineSpec to a SecretsEngine.
func (s *SecretsEngineSpec) ToSecretsEngine() (*SecretsEngine, error) {
	// Validate
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := s.SecretsEngine.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &SecretsEngine{
		ID:        uuid.New().String(),
		Name:      s.SecretsEngine.Name,
		Namespace: namespace,
		Type:      s.SecretsEngine.Type,
		Function:  s.SecretsEngine.Function,
		Path:      s.SecretsEngine.Path,
		Config:    s.SecretsEngine.Config,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
