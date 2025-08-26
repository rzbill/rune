package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	yaml "gopkg.in/yaml.v3"
)

// ConfigMapSpec represents the YAML specification for a config (flat form).
type ConfigMapSpec struct {
	// Human-readable name for the config (required)
	Name string `json:"name" yaml:"name"`

	// Namespace the config belongs to (optional, defaults to "default")
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Config data
	Data map[string]string `json:"data" yaml:"data"`

	// Skip indicates this spec should be ignored by castfile parsing
	Skip bool `json:"skip,omitempty" yaml:"skip,omitempty"`

	// rawNode holds the original YAML mapping node for structural validation
	rawNode *yaml.Node `json:"-" yaml:"-"`
}

// UnmarshalYAML implements custom unmarshalling so `data` can be provided
// either as a mapping (key: value) or as a sequence of {key, value} objects.
func (c *ConfigMapSpec) UnmarshalYAML(value *yaml.Node) error {
	// Preserve original node for structural validation
	c.rawNode = value

	// Decode known fields, keeping data raw for normalization
	var aux struct {
		Name      string      `yaml:"name"`
		Namespace string      `yaml:"namespace"`
		Data      interface{} `yaml:"data"`
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}
	c.Name = aux.Name
	c.Namespace = aux.Namespace

	if aux.Data != nil {
		normalized, err := decodeStringMapOrKVAny(aux.Data)
		if err != nil {
			return err
		}
		c.Data = normalized
	}
	return nil
}

// ToConfigMap converts a ConfigMapSpec to a ConfigMap.
func (c *ConfigMapSpec) ToConfigMap() (*Configmap, error) {
	// Validate
	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := c.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &Configmap{
		ID:        uuid.New().String(),
		Name:      c.Name,
		Namespace: namespace,
		Data:      c.Data,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks if a config specification is valid.
func (c *ConfigMapSpec) Validate() error {
	// Structural validation against original YAML node when available
	if err := c.validateStructureFromNode(); err != nil {
		return err
	}
	if c.Name == "" {
		return NewValidationError("config name is required")
	}

	if len(c.Data) == 0 {
		return NewValidationError("config must have data")
	}

	return nil
}

// validateStructureFromNode validates unknown fields using the captured raw YAML node.
// If no raw node is available (e.g., constructed programmatically), it is a no-op.
func (c *ConfigMapSpec) validateStructureFromNode() error {
	if c.rawNode == nil {
		return nil
	}
	validFields := map[string]bool{
		"name":      true,
		"namespace": true,
		"data":      true,
		"skip":      true,
	}
	var errors []string
	if c.rawNode.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(c.rawNode.Content); i += 2 {
			k := c.rawNode.Content[i]
			if !validFields[k.Value] {
				errors = append(errors, fmt.Sprintf("unknown field '%s' in config specification at line %d", k.Value, k.Line))
			}
		}
	}
	if len(errors) > 0 {
		return NewValidationError(strings.Join(errors, "\n"))
	}
	return nil
}

// Implement Spec interface for ConfigMapSpec
func (c *ConfigMapSpec) GetName() string      { return c.Name }
func (c *ConfigMapSpec) GetNamespace() string { return c.Namespace }
func (c *ConfigMapSpec) Kind() string         { return "Config" }
