package types

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

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

	// Config mounts
	ConfigMounts []ConfigMount `json:"configMounts,omitempty" yaml:"configMounts,omitempty"`

	// Service discovery configuration
	Discovery *ServiceDiscovery `json:"discovery,omitempty" yaml:"discovery,omitempty"`
}

// ServiceFile represents a file containing service definitions.
type ServiceFile struct {
	// Service definition (only one of Service or Services should be set)
	Service *ServiceSpec `json:"service,omitempty" yaml:"service,omitempty"`

	// Multiple service definitions
	Services []ServiceSpec `json:"services,omitempty" yaml:"services,omitempty"`

	// Internal tracking for line numbers (not serialized)
	lineInfo map[string]int `json:"-" yaml:"-"`
}

// ParseServiceFile parses a YAML file containing service definitions.
func ParseServiceFile(filename string) (*ServiceFile, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseServiceData(data)
}

// ParseServiceData parses YAML data containing service definitions.
func ParseServiceData(data []byte) (*ServiceFile, error) {
	var serviceFile ServiceFile
	serviceFile.lineInfo = make(map[string]int)

	// First unmarshal normally to do basic parsing
	if err := yaml.Unmarshal(data, &serviceFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Now decode again to track line numbers
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML structure: %w", err)
	}

	// Extract line numbers
	if err := extractLineInfo(&node, &serviceFile); err != nil {
		// Don't fail if we can't extract line info, just continue
		fmt.Printf("Warning: couldn't extract line information: %v\n", err)
	}

	// Validate that at least one service is defined
	if serviceFile.Service == nil && len(serviceFile.Services) == 0 {
		return nil, fmt.Errorf("no services defined in the file")
	}

	// Validate structure for each service
	for _, service := range serviceFile.GetServices() {
		if err := service.ValidateStructure(data); err != nil {
			return nil, fmt.Errorf("structure validation failed: %w", err)
		}
	}

	return &serviceFile, nil
}

// extractLineInfo traverses the YAML node structure to find line numbers for services
func extractLineInfo(node *yaml.Node, serviceFile *ServiceFile) error {
	// Handle different document structures - single service or list of services
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return extractLineInfo(node.Content[0], serviceFile)
	}

	if node.Kind == yaml.MappingNode && len(node.Content) >= 2 {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "service" && value.Kind == yaml.MappingNode {
				// Handle single service
				for j := 0; j < len(value.Content); j += 2 {
					if value.Content[j].Value == "name" {
						name := value.Content[j+1].Value
						serviceFile.lineInfo[name] = value.Line
						break
					}
				}
			} else if key.Value == "services" && value.Kind == yaml.SequenceNode {
				// Handle multiple services
				for _, serviceNode := range value.Content {
					if serviceNode.Kind == yaml.MappingNode {
						for j := 0; j < len(serviceNode.Content); j += 2 {
							if serviceNode.Content[j].Value == "name" {
								name := serviceNode.Content[j+1].Value
								serviceFile.lineInfo[name] = serviceNode.Line
								break
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// Validate validates the service specification.
func (s *ServiceSpec) Validate() error {
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

// ValidateStructure validates the YAML structure against the service specification
func (s *ServiceSpec) ValidateStructure(data []byte) error {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
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
	}

	// Define valid health check fields
	validHealthFields := map[string]bool{
		"liveness":  true,
		"readiness": true,
	}

	// Define valid probe fields
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

	// Define valid port fields
	validPortFields := map[string]bool{
		"name":       true,
		"port":       true,
		"targetPort": true,
		"protocol":   true,
	}

	var errors []string
	collectValidationErrors(&node, "service", validServiceFields, validHealthFields, validProbeFields, validPortFields, &errors)

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// collectValidationErrors recursively collects validation errors for YAML structure
func collectValidationErrors(node *yaml.Node, context string, validFields map[string]bool, validHealthFields map[string]bool, validProbeFields map[string]bool, validPortFields map[string]bool, errors *[]string) {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		collectValidationErrors(node.Content[0], context, validFields, validHealthFields, validProbeFields, validPortFields, errors)
		return
	}

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "service" && value.Kind == yaml.MappingNode {
				// Validate service fields
				for j := 0; j < len(value.Content); j += 2 {
					fieldKey := value.Content[j]
					fieldValue := value.Content[j+1]

					if !validFields[fieldKey.Value] {
						*errors = append(*errors, fmt.Sprintf("unknown field '%s' in service specification at line %d", fieldKey.Value, fieldKey.Line))
					}

					// Recursively validate nested structures
					if fieldKey.Value == "health" && fieldValue.Kind == yaml.MappingNode {
						collectHealthErrors(fieldValue, validHealthFields, validProbeFields, errors)
					}

					if fieldKey.Value == "ports" && fieldValue.Kind == yaml.SequenceNode {
						collectPortsErrors(fieldValue, validPortFields, errors)
					}
				}
			}
		}
	}
}

// collectHealthErrors collects validation errors for health check structure
func collectHealthErrors(healthNode *yaml.Node, validHealthFields map[string]bool, validProbeFields map[string]bool, errors *[]string) {
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
func collectPortsErrors(portsNode *yaml.Node, validPortFields map[string]bool, errors *[]string) {
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
		ID:            uuid.New().String(),
		Name:          s.Name,
		Namespace:     namespace,
		Labels:        s.Labels,
		Image:         s.Image,
		Command:       s.Command,
		Args:          s.Args,
		Env:           s.Env,
		Scale:         s.Scale,
		Ports:         s.Ports,
		Resources:     resources,
		Health:        s.Health,
		NetworkPolicy: s.NetworkPolicy,
		Expose:        s.Expose,
		Affinity:      s.Affinity,
		Autoscale:     s.Autoscale,
		SecretMounts:  s.SecretMounts,
		ConfigMounts:  s.ConfigMounts,
		Discovery:     s.Discovery,
		Status:        ServiceStatusPending,
		Metadata:      &ServiceMetadata{CreatedAt: now, UpdatedAt: now},
	}, nil
}

// GetServices returns all services defined in the file.
func (f *ServiceFile) GetServices() []*ServiceSpec {
	var services []*ServiceSpec

	if f.Service != nil {
		services = append(services, f.Service)
	}

	if len(f.Services) > 0 {
		for i := range f.Services {
			services = append(services, &f.Services[i])
		}
	}

	return services
}

// GetLineInfo returns the approximate line number for a service by name
func (f *ServiceFile) GetLineInfo(name string) (int, bool) {
	if f.lineInfo == nil {
		return 0, false
	}
	line, ok := f.lineInfo[name]
	return line, ok
}
