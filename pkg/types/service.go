package types

import (
	"strconv"
	"time"
)

// Service represents a deployable application or workload.
type Service struct {
	NamespacedResource `json:"-" yaml:"-"`

	// Unique identifier for the service
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the service
	Name string `json:"name" yaml:"name"`

	// Namespace the service belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Labels are key/value pairs that can be used to organize and categorize services
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Container image for the service
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// Command to run in the container (overrides image CMD)
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// Arguments to the command
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// Environment variables for the service
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// Number of instances to run
	Scale int `json:"scale" yaml:"scale"`

	// Ports exposed by the service
	Ports []ServicePort `json:"ports,omitempty" yaml:"ports,omitempty"`

	// Resource requirements for each instance
	Resources Resources `json:"resources,omitempty" yaml:"resources,omitempty"`

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

	// Status of the service
	Status ServiceStatus `json:"status" yaml:"status"`

	// Instances of this service currently running
	Instances []Instance `json:"instances,omitempty" yaml:"instances,omitempty"`

	// Runtime for the service ("container" or "process")
	Runtime RuntimeType `json:"runtime,omitempty" yaml:"runtime,omitempty"`

	// Process-specific configuration (when Runtime="process")
	Process *ProcessSpec `json:"process,omitempty" yaml:"process,omitempty"`

	// Restart policy for the service
	RestartPolicy RestartPolicy `json:"restart_policy,omitempty" yaml:"restart_policy,omitempty"`

	// Metadata for the service
	Metadata *ServiceMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (s *Service) GetResourceType() ResourceType {
	return ResourceTypeService
}

// ServiceMetadata represents metadata for a service.
type ServiceMetadata struct {
	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`

	// Generation of the service
	Generation int64 `json:"generation,omitempty" yaml:"generation,omitempty"`
}

// ServicePort represents a port exposed by a service.
type ServicePort struct {
	// Name for this port (used in references)
	Name string `json:"name" yaml:"name"`

	// Port number
	Port int `json:"port" yaml:"port"`

	// Target port (if different from port)
	TargetPort int `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`

	// Protocol (default: TCP)
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}

// ServiceExpose defines how a service is exposed externally.
type ServiceExpose struct {
	// Port or port name to expose
	Port string `json:"port" yaml:"port"`

	// Host for the exposed service
	Host string `json:"host,omitempty" yaml:"host,omitempty"`

	// Path prefix for the exposed service
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// TLS configuration for the exposed service
	TLS *ExposeServiceTLS `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// ExposeServiceTLS defines TLS configuration for exposed services.
type ExposeServiceTLS struct {
	// Secret name containing TLS certificate and key
	SecretName string `json:"secretName,omitempty" yaml:"secretName,omitempty"`

	// Whether to automatically generate a TLS certificate
	Auto bool `json:"auto,omitempty" yaml:"auto,omitempty"`
}

// ServiceDiscovery defines how a service is discovered by other services.
type ServiceDiscovery struct {
	// Discovery mode (load-balanced or headless)
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

// ServiceAffinity defines placement rules for a service.
type ServiceAffinity struct {
	// Hard constraints (service can only run on nodes matching these)
	Required []string `json:"required,omitempty" yaml:"required,omitempty"`

	// Soft preferences (scheduler will try to place on nodes matching these)
	Preferred []string `json:"preferred,omitempty" yaml:"preferred,omitempty"`

	// Run instances near services matching these labels
	With []string `json:"with,omitempty" yaml:"with,omitempty"`

	// Avoid running instances on nodes with services matching these labels
	Avoid []string `json:"avoid,omitempty" yaml:"avoid,omitempty"`

	// Try to distribute instances across this topology key (e.g., "zone")
	Spread string `json:"spread,omitempty" yaml:"spread,omitempty"`
}

// ServiceAutoscale defines autoscaling behavior for a service.
type ServiceAutoscale struct {
	// Whether autoscaling is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Minimum number of instances
	Min int `json:"min" yaml:"min"`

	// Maximum number of instances
	Max int `json:"max" yaml:"max"`

	// Metric to scale on (e.g., cpu, memory)
	Metric string `json:"metric" yaml:"metric"`

	// Target value for the metric (e.g., 70%)
	Target string `json:"target" yaml:"target"`

	// Cooldown period between scaling events
	Cooldown string `json:"cooldown,omitempty" yaml:"cooldown,omitempty"`

	// Maximum number of instances to add/remove in a single scaling event
	Step int `json:"step,omitempty" yaml:"step,omitempty"`
}

// ServiceStatus represents the current status of a service.
type ServiceStatus string

const (
	// ServiceStatusPending indicates the service is being created.
	ServiceStatusPending ServiceStatus = "Pending"

	// ServiceStatusRunning indicates the service is running.
	ServiceStatusRunning ServiceStatus = "Running"

	// ServiceStatusDeploying indicates the service is being updated.
	ServiceStatusDeploying ServiceStatus = "Deploying"

	// ServiceStatusFailed indicates the service failed to deploy or run.
	ServiceStatusFailed ServiceStatus = "Failed"

	// ServiceStatusDeleted indicates the service has been deleted.
	ServiceStatusDeleted ServiceStatus = "Deleted"
)

// Resources represents resource requirements for a service instance.
type Resources struct {
	// CPU request in millicores (1000m = 1 CPU)
	CPU ResourceLimit `json:"cpu,omitempty" yaml:"cpu,omitempty"`

	// Memory request in bytes
	Memory ResourceLimit `json:"memory,omitempty" yaml:"memory,omitempty"`
}

// ResourceLimit defines request and limit for a resource.
type ResourceLimit struct {
	// Requested resources (guaranteed)
	Request string `json:"request,omitempty" yaml:"request,omitempty"`

	// Maximum resources (limit)
	Limit string `json:"limit,omitempty" yaml:"limit,omitempty"`
}

// HealthCheck represents health check configuration for a service.
type HealthCheck struct {
	// Liveness probe checks if the instance is running
	Liveness *Probe `json:"liveness,omitempty" yaml:"liveness,omitempty"`

	// Readiness probe checks if the instance is ready to receive traffic
	Readiness *Probe `json:"readiness,omitempty" yaml:"readiness,omitempty"`
}

// Probe represents a health check probe configuration.
type Probe struct {
	// Type of probe (http, tcp, exec)
	Type string `json:"type" yaml:"type"`

	// HTTP path for http probe
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Port to connect to
	Port int `json:"port" yaml:"port"`

	// Command to execute for exec probe
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`

	// Initial delay seconds before starting checks
	InitialDelaySeconds int `json:"initialDelaySeconds,omitempty" yaml:"initialDelaySeconds,omitempty"`

	// Interval between checks in seconds
	IntervalSeconds int `json:"intervalSeconds,omitempty" yaml:"intervalSeconds,omitempty"`

	// Timeout for the probe in seconds
	TimeoutSeconds int `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`

	// Failure threshold for the probe
	FailureThreshold int `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`

	// Success threshold for the probe
	SuccessThreshold int `json:"successThreshold,omitempty" yaml:"successThreshold,omitempty"`
}

// RestartPolicy defines how instances should be restarted
type RestartPolicy string

const (
	// RestartPolicyAlways means always restart when not explicitly stopped
	RestartPolicyAlways RestartPolicy = "Always"

	// RestartPolicyOnFailure means only restart on failure
	RestartPolicyOnFailure RestartPolicy = "OnFailure"

	// RestartPolicyNever means never restart automatically, only manual restarts are allowed
	RestartPolicyNever RestartPolicy = "Never"
)

// Validate validates the service configuration.
func (s *Service) Validate() error {
	if s.ID == "" {
		return NewValidationError("service ID is required")
	}

	if s.Name == "" {
		return NewValidationError("service name is required")
	}

	// Check runtime specific requirements
	if s.Runtime == "container" || s.Runtime == "" {
		// Default is container runtime
		if s.Image == "" {
			return NewValidationError("service image is required for container runtime")
		}
	} else if s.Runtime == "process" {
		// For process runtime, we need a process spec
		if s.Process == nil {
			return NewValidationError("process configuration is required for process runtime")
		}
		if err := s.Process.Validate(); err != nil {
			return WrapValidationError(err, "invalid process configuration")
		}
	} else {
		return NewValidationError("unknown runtime: " + string(s.Runtime))
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

// Validate validates the health check configuration.
func (h *HealthCheck) Validate() error {
	if h.Liveness != nil {
		if err := h.Liveness.Validate(); err != nil {
			return WrapValidationError(err, "invalid liveness probe")
		}
	}

	if h.Readiness != nil {
		if err := h.Readiness.Validate(); err != nil {
			return WrapValidationError(err, "invalid readiness probe")
		}
	}

	return nil
}

// Validate validates the probe configuration.
func (p *Probe) Validate() error {
	switch p.Type {
	case "http":
		if p.Path == "" {
			return NewValidationError("http probe must have a path")
		}
		if p.Port <= 0 {
			return NewValidationError("http probe must have a valid port")
		}
	case "tcp":
		if p.Port <= 0 {
			return NewValidationError("tcp probe must have a valid port")
		}
	case "exec":
		if len(p.Command) == 0 {
			return NewValidationError("exec probe must have a command")
		}
	default:
		return NewValidationError("unknown probe type: " + p.Type)
	}

	return nil
}
