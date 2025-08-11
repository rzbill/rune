package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var _ Spec = (*ServiceSpec)(nil)

// ServiceSpec is the YAML/JSON specification for a service.
type ServiceSpec struct {
	// Human-readable name for the service (required)
	Name string `json:"name" yaml:"name"`

	// Namespace the service belongs to (optional, defaults to "default")
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Labels for the service
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Container image for the service (required)
	Image string `json:"image" yaml:"image"`

	// Command to run in the container (overrides image CMD)
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// Arguments to the command
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// Environment variables for the service
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// Number of instances to run (default: 1)
	Scale int `json:"scale" yaml:"scale"`

	// Ports exposed by the service
	Ports []ServicePort `json:"ports,omitempty" yaml:"ports,omitempty"`

	// Resource requirements for each instance
	Resources *Resources `json:"resources,omitempty" yaml:"resources,omitempty"`

	// Health checks for the service
	Health *HealthCheck `json:"health,omitempty" yaml:"health,omitempty"`

	// Network policy for controlling traffic
	NetworkPolicy *ServiceNetworkPolicy `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`

	// External exposure configuration
	Expose *ServiceExpose `json:"expose,omitempty" yaml:"expose,omitempty"`

	// Placement preferences and requirements
	Affinity *ServiceAffinity `json:"affinity,omitempty" yaml:"affinity,omitempty"`

	// Autoscaling configuration
	Autoscale *ServiceAutoscale `json:"autoscale,omitempty" yaml:"autoscale,omitempty"`

	// Secret mounts
	SecretMounts []SecretMount `json:"secretMounts,omitempty" yaml:"secretMounts,omitempty"`

	// Configmap mounts
	ConfigmapMounts []ConfigmapMount `json:"configmapMounts,omitempty" yaml:"configmapMounts,omitempty"`

	// Service discovery configuration
	Discovery *ServiceDiscovery `json:"discovery,omitempty" yaml:"discovery,omitempty"`

	// Skip indicates this spec should be ignored by castfile parsing
	Skip bool `json:"skip,omitempty" yaml:"skip,omitempty"`

	// rawNode holds the original YAML mapping node for structural validation
	rawNode *yaml.Node `json:"-" yaml:"-"`
}

// Implement Spec interface for ServiceSpec
func (s *ServiceSpec) GetName() string      { return s.Name }
func (s *ServiceSpec) GetNamespace() string { return s.Namespace }
func (s *ServiceSpec) Kind() string         { return "Service" }

// ValidateStructure validates the YAML structure against the service specification
// Deprecated: ValidateStructure is no longer used; structural checks happen in Validate via validateStructureFromNode.
func (s *ServiceSpec) ValidateStructure(data []byte) error { return nil }

// Validate validates the service specification.
func (s *ServiceSpec) Validate() error {
	// Structural validation against original YAML node when available
	if err := s.validateStructureFromNode(); err != nil {
		return err
	}
	if s.Name == "" {
		return NewValidationError("service name is required")
	}

	if s.Image == "" {
		return NewValidationError("service image is required")
	}

	if s.Scale < 0 {
		return NewValidationError("service scale cannot be negative")
	}

	// Validate ports if present
	for i, port := range s.Ports {
		if port.Name == "" {
			return NewValidationError("port name is required for port at index " + strconv.Itoa(i))
		}
		if port.Port <= 0 || port.Port > 65535 {
			return NewValidationError("port must be between 1 and 65535 for port " + port.Name)
		}
	}

	// Validate health checks if present
	if s.Health != nil {
		if err := s.Health.Validate(); err != nil {
			return WrapValidationError(err, "invalid health check")
		}
	}

	// Validate network policy if present
	if s.NetworkPolicy != nil {
		if err := s.NetworkPolicy.Validate(); err != nil {
			return WrapValidationError(err, "invalid network policy")
		}
	}

	// Validate autoscale if present
	if s.Autoscale != nil && s.Autoscale.Enabled {
		if s.Autoscale.Min < 0 {
			return NewValidationError("autoscale min cannot be negative")
		}
		if s.Autoscale.Max < s.Autoscale.Min {
			return NewValidationError("autoscale max cannot be less than min")
		}
		if s.Autoscale.Metric == "" {
			return NewValidationError("autoscale metric is required")
		}
		if s.Autoscale.Target == "" {
			return NewValidationError("autoscale target is required")
		}
	}

	// Validate expose if present
	if s.Expose != nil {
		if s.Expose.Port == "" {
			return NewValidationError("expose port is required")
		}
	}

	return nil
}

// validateStructureFromNode validates unknown fields using the captured raw YAML node.
// If no raw node is available (e.g., constructed programmatically), it is a no-op.
func (s *ServiceSpec) validateStructureFromNode() error {
	if s.rawNode == nil {
		return nil
	}

	// Define valid service fields based on ServiceSpec
	validServiceFields := map[string]bool{
		"name":          true,
		"namespace":     true,
		"labels":        true,
		"image":         true,
		"command":       true,
		"args":          true,
		"env":           true,
		"scale":         true,
		"ports":         true,
		"resources":     true,
		"health":        true,
		"networkPolicy": true,
		"expose":        true,
		"affinity":      true,
		"autoscale":     true,
		"secretMounts":  true,
		"configMounts":  true,
		"discovery":     true,
		"skip":          true,
	}

	validHealthFields := map[string]bool{
		"liveness":  true,
		"readiness": true,
	}
	validProbeFields := map[string]bool{
		"type":                true,
		"path":                true,
		"port":                true,
		"command":             true,
		"initialDelaySeconds": true,
		"intervalSeconds":     true,
		"timeoutSeconds":      true,
		"failureThreshold":    true,
		"successThreshold":    true,
	}
	validPortFields := map[string]bool{
		"name":       true,
		"port":       true,
		"targetPort": true,
		"protocol":   true,
	}

	var errors []string
	// Validate fields directly on the service mapping node
	if s.rawNode.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(s.rawNode.Content); i += 2 {
			fieldKey := s.rawNode.Content[i]
			fieldVal := s.rawNode.Content[i+1]
			if !validServiceFields[fieldKey.Value] {
				errors = append(errors, fmt.Sprintf("unknown field '%s' in service specification at line %d", fieldKey.Value, fieldKey.Line))
				continue
			}
			if fieldKey.Value == "health" && fieldVal.Kind == yaml.MappingNode {
				collectServiceHealthErrors(fieldVal, validHealthFields, validProbeFields, &errors)
			}
			if fieldKey.Value == "ports" && fieldVal.Kind == yaml.SequenceNode {
				collectServicePortsErrors(fieldVal, validPortFields, &errors)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n%s", strings.Join(errors, "\n"))
	}
	return nil
}

// ToService converts a ServiceSpec to a Service.
func (s *ServiceSpec) ToService() (*Service, error) {
	// Validate
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := s.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	var resources Resources
	if s.Resources != nil {
		resources = *s.Resources
	}

	return &Service{
		ID:              uuid.New().String(),
		Name:            s.Name,
		Namespace:       namespace,
		Labels:          s.Labels,
		Image:           s.Image,
		Command:         s.Command,
		Args:            s.Args,
		Env:             s.Env,
		Scale:           s.Scale,
		Ports:           s.Ports,
		Resources:       resources,
		Health:          s.Health,
		NetworkPolicy:   s.NetworkPolicy,
		Expose:          s.Expose,
		Affinity:        s.Affinity,
		Autoscale:       s.Autoscale,
		SecretMounts:    s.SecretMounts,
		ConfigmapMounts: s.ConfigmapMounts,
		Discovery:       s.Discovery,
		Status:          ServiceStatusPending,
		Metadata:        &ServiceMetadata{CreatedAt: now, UpdatedAt: now},
	}, nil
}

// RestoreTemplateReferences restores template references in environment variables
// This should be called after parsing to restore the original template syntax
func (s *ServiceSpec) RestoreTemplateReferences(templateMap map[string]string) {
	if s.Env == nil {
		return
	}

	for key, value := range s.Env {
		// Create a working copy of the value
		restoredValue := value

		// Replace all placeholders in this value
		for placeholder, templateRef := range templateMap {
			if strings.Contains(restoredValue, placeholder) {
				restoredValue = strings.ReplaceAll(restoredValue, placeholder, "{{"+templateRef+"}}")
			}
		}

		// Update the environment variable with the restored value
		s.Env[key] = restoredValue
	}
}

// GetEnvWithTemplates returns environment variables with template references restored
func (s *ServiceSpec) GetEnvWithTemplates(templateMap map[string]string) map[string]string {
	if s.Env == nil {
		return nil
	}

	result := make(map[string]string)
	for key, value := range s.Env {
		// Create a working copy of the value
		restoredValue := value

		// Replace all placeholders in this value
		for placeholder, templateRef := range templateMap {
			if strings.Contains(restoredValue, placeholder) {
				restoredValue = strings.ReplaceAll(restoredValue, placeholder, "{{"+templateRef+"}}")
			}
		}

		result[key] = restoredValue
	}
	return result
}

// collectValidationErrors recursively collects validation errors for YAML structure
// Deprecated: collectServiceValidationErrors is no longer used.

// collectHealthErrors collects validation errors for health check structure
func collectServiceHealthErrors(healthNode *yaml.Node, validHealthFields map[string]bool, validProbeFields map[string]bool, errors *[]string) {
	for i := 0; i < len(healthNode.Content); i += 2 {
		key := healthNode.Content[i]
		value := healthNode.Content[i+1]

		if !validHealthFields[key.Value] {
			*errors = append(*errors, fmt.Sprintf("unknown field '%s' in health check specification at line %d", key.Value, key.Line))
		}

		// Validate probe structure
		if value.Kind == yaml.MappingNode {
			for j := 0; j < len(value.Content); j += 2 {
				probeKey := value.Content[j]
				if !validProbeFields[probeKey.Value] {
					*errors = append(*errors, fmt.Sprintf("unknown field '%s' in probe specification at line %d", probeKey.Value, probeKey.Line))
				}
			}
		}
	}
}

// collectPortsErrors collects validation errors for ports structure
func collectServicePortsErrors(portsNode *yaml.Node, validPortFields map[string]bool, errors *[]string) {
	for _, portNode := range portsNode.Content {
		if portNode.Kind == yaml.MappingNode {
			for i := 0; i < len(portNode.Content); i += 2 {
				key := portNode.Content[i]
				if !validPortFields[key.Value] {
					*errors = append(*errors, fmt.Sprintf("unknown field '%s' in port specification at line %d", key.Value, key.Line))
				}
			}
		}
	}
}
