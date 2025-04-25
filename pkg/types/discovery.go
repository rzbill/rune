package types

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Gateway represents an ingress/egress gateway for the service mesh.
type Gateway struct {
	// Unique identifier for the gateway
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the gateway
	Name string `json:"name" yaml:"name"`

	// Namespace the gateway belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Hostname for the gateway (for virtual hosting)
	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"`

	// Whether this is an internal gateway (not exposed outside the mesh)
	Internal bool `json:"internal" yaml:"internal"`

	// Port configurations for the gateway
	Ports []GatewayPort `json:"ports" yaml:"ports"`

	// TLS configuration options
	TLS *GatewayTLS `json:"tls,omitempty" yaml:"tls,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// GatewayPort represents a port exposed by a gateway.
type GatewayPort struct {
	// Port number
	Port int `json:"port" yaml:"port"`

	// Protocol for this port (HTTP, HTTPS, TCP, etc.)
	Protocol string `json:"protocol" yaml:"protocol"`

	// Name for this port (optional)
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// GatewayTLS represents TLS configuration for a gateway.
type GatewayTLS struct {
	// Mode of TLS operation (PASSTHROUGH, SIMPLE, MUTUAL)
	Mode string `json:"mode" yaml:"mode"`

	// Secret name containing TLS credentials
	SecretName string `json:"secretName,omitempty" yaml:"secretName,omitempty"`

	// Minimum TLS version
	MinVersion string `json:"minVersion,omitempty" yaml:"minVersion,omitempty"`

	// Maximum TLS version
	MaxVersion string `json:"maxVersion,omitempty" yaml:"maxVersion,omitempty"`

	// Allow client certificate validation
	ClientCertificate bool `json:"clientCertificate,omitempty" yaml:"clientCertificate,omitempty"`
}

// Route represents a rule for routing traffic from a gateway to services.
type Route struct {
	// Unique identifier for the route
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the route
	Name string `json:"name" yaml:"name"`

	// Namespace the route belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Gateway this route is attached to
	Gateway string `json:"gateway" yaml:"gateway"`

	// Host or hosts this route matches (e.g., "api.example.com")
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`

	// HTTP path-based routing rules
	HTTP []HTTPRoute `json:"http,omitempty" yaml:"http,omitempty"`

	// TCP routing rules (port-based)
	TCP []TCPRoute `json:"tcp,omitempty" yaml:"tcp,omitempty"`

	// Priority of this route (higher numbers take precedence)
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// HTTPRoute represents an HTTP routing rule.
type HTTPRoute struct {
	// Path match for this route (e.g., "/api/v1")
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Path prefix match (e.g., "/api")
	PathPrefix string `json:"pathPrefix,omitempty" yaml:"pathPrefix,omitempty"`

	// HTTP methods this route applies to (GET, POST, etc.)
	Methods []string `json:"methods,omitempty" yaml:"methods,omitempty"`

	// Header matches required
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`

	// Query parameter matches required
	QueryParams map[string]string `json:"queryParams,omitempty" yaml:"queryParams,omitempty"`

	// Destination service and options
	Destination RouteDestination `json:"destination" yaml:"destination"`

	// Rewrite options
	Rewrite *HTTPRewrite `json:"rewrite,omitempty" yaml:"rewrite,omitempty"`

	// Response headers to be added
	AddResponseHeaders map[string]string `json:"addResponseHeaders,omitempty" yaml:"addResponseHeaders,omitempty"`

	// Request headers to be added
	AddRequestHeaders map[string]string `json:"addRequestHeaders,omitempty" yaml:"addRequestHeaders,omitempty"`

	// Timeout for this route in milliseconds
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// Retry policy
	Retries *HTTPRetryPolicy `json:"retries,omitempty" yaml:"retries,omitempty"`

	// CORS policy
	CORS *CORSPolicy `json:"cors,omitempty" yaml:"cors,omitempty"`
}

// TCPRoute represents a TCP routing rule.
type TCPRoute struct {
	// Port number for this TCP route
	Port int `json:"port" yaml:"port"`

	// Destination service and options
	Destination RouteDestination `json:"destination" yaml:"destination"`
}

// RouteDestination represents the target of a route.
type RouteDestination struct {
	// Service name to route to
	Service string `json:"service" yaml:"service"`

	// Namespace of the service (optional, defaults to route's namespace)
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Port on the service to route to
	Port int `json:"port" yaml:"port"`

	// Weight for traffic splitting (0-100)
	Weight int `json:"weight,omitempty" yaml:"weight,omitempty"`

	// Subset of the service to route to (requires service to have defined subsets)
	Subset string `json:"subset,omitempty" yaml:"subset,omitempty"`
}

// HTTPRewrite defines how the path and/or host should be rewritten.
type HTTPRewrite struct {
	// URI to replace the matched path with
	URI string `json:"uri,omitempty" yaml:"uri,omitempty"`

	// Host to replace the request host with
	Host string `json:"host,omitempty" yaml:"host,omitempty"`
}

// HTTPRetryPolicy defines how request retries should be performed.
type HTTPRetryPolicy struct {
	// Number of retry attempts
	Attempts int `json:"attempts" yaml:"attempts"`

	// Timeout per retry attempt in milliseconds
	PerTryTimeout int `json:"perTryTimeout,omitempty" yaml:"perTryTimeout,omitempty"`

	// HTTP status codes that trigger a retry
	RetryOn []int `json:"retryOn,omitempty" yaml:"retryOn,omitempty"`
}

// CORSPolicy defines Cross-Origin Resource Sharing settings.
type CORSPolicy struct {
	// Allowed origins (e.g., "https://example.com")
	AllowOrigins []string `json:"allowOrigins" yaml:"allowOrigins"`

	// Allowed methods (e.g., "GET", "POST")
	AllowMethods []string `json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`

	// Allowed headers
	AllowHeaders []string `json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`

	// Exposed headers
	ExposeHeaders []string `json:"exposeHeaders,omitempty" yaml:"exposeHeaders,omitempty"`

	// Max age for CORS preflight requests in seconds
	MaxAge int `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`

	// Allow credentials
	AllowCredentials bool `json:"allowCredentials,omitempty" yaml:"allowCredentials,omitempty"`
}

// ServiceRegistry represents the discovery registry for services.
type ServiceRegistry struct {
	// Map of service namespaced names to endpoint information
	Services map[string]*ServiceEndpoints `json:"services" yaml:"services"`

	// Last update timestamp
	LastUpdated time.Time `json:"lastUpdated" yaml:"lastUpdated"`
}

// ServiceEndpoints represents the discovered endpoints for a service.
type ServiceEndpoints struct {
	// Service full name (namespace.name)
	ServiceID string `json:"serviceId" yaml:"serviceId"`

	// Individual endpoints (instances) for this service
	Endpoints []Endpoint `json:"endpoints" yaml:"endpoints"`

	// Service-level metadata
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Endpoint represents a specific instance endpoint.
type Endpoint struct {
	// Instance ID this endpoint belongs to
	InstanceID string `json:"instanceId" yaml:"instanceId"`

	// IP address for this endpoint
	IP string `json:"ip" yaml:"ip"`

	// Port for this endpoint
	Port int `json:"port" yaml:"port"`

	// Protocol for this endpoint
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`

	// Endpoint-specific metadata
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Health status for this endpoint
	Healthy bool `json:"healthy" yaml:"healthy"`

	// Last health check timestamp
	LastCheck time.Time `json:"lastCheck,omitempty" yaml:"lastCheck,omitempty"`
}

// GatewaySpec represents the YAML specification for a gateway.
type GatewaySpec struct {
	// Gateway metadata and configuration
	Gateway struct {
		// Human-readable name for the gateway (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the gateway belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Hostname for the gateway (for virtual hosting)
		Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"`

		// Whether this is an internal gateway (not exposed outside the mesh)
		Internal bool `json:"internal" yaml:"internal"`

		// Port configurations for the gateway
		Ports []GatewayPort `json:"ports" yaml:"ports"`

		// TLS configuration options
		TLS *GatewayTLS `json:"tls,omitempty" yaml:"tls,omitempty"`
	} `json:"gateway" yaml:"gateway"`
}

// RouteSpec represents the YAML specification for a route.
type RouteSpec struct {
	// Route metadata and rules
	Route struct {
		// Human-readable name for the route (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the route belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Gateway this route is attached to
		Gateway string `json:"gateway" yaml:"gateway"`

		// Host or hosts this route matches
		Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`

		// HTTP path-based routing rules
		HTTP []HTTPRoute `json:"http,omitempty" yaml:"http,omitempty"`

		// TCP routing rules (port-based)
		TCP []TCPRoute `json:"tcp,omitempty" yaml:"tcp,omitempty"`

		// Priority of this route (higher numbers take precedence)
		Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`
	} `json:"route" yaml:"route"`
}

// Validate checks if a gateway specification is valid.
func (g *GatewaySpec) Validate() error {
	if g.Gateway.Name == "" {
		return NewValidationError("gateway name is required")
	}

	if len(g.Gateway.Ports) == 0 {
		return NewValidationError("gateway must specify at least one port")
	}

	// Validate TLS configuration if provided
	if g.Gateway.TLS != nil {
		if g.Gateway.TLS.Mode != "PASSTHROUGH" && g.Gateway.TLS.Mode != "SIMPLE" && g.Gateway.TLS.Mode != "MUTUAL" {
			return NewValidationError("tls mode must be PASSTHROUGH, SIMPLE, or MUTUAL")
		}

		if (g.Gateway.TLS.Mode == "SIMPLE" || g.Gateway.TLS.Mode == "MUTUAL") && g.Gateway.TLS.SecretName == "" {
			return NewValidationError("tls secretName is required for SIMPLE and MUTUAL modes")
		}
	}

	// Validate each port
	portSet := make(map[int]bool)
	for i, port := range g.Gateway.Ports {
		if port.Port <= 0 || port.Port > 65535 {
			return NewValidationError("port at index " + strconv.Itoa(i) + " must be between 1 and 65535")
		}

		if portSet[port.Port] {
			return NewValidationError("duplicate port: " + strconv.Itoa(port.Port))
		}
		portSet[port.Port] = true

		if port.Protocol == "" {
			return NewValidationError("port at index " + strconv.Itoa(i) + " must specify protocol")
		}
	}

	return nil
}

// Validate checks if a route specification is valid.
func (r *RouteSpec) Validate() error {
	if r.Route.Name == "" {
		return NewValidationError("route name is required")
	}

	if r.Route.Gateway == "" {
		return NewValidationError("route must specify a gateway")
	}

	if len(r.Route.HTTP) == 0 && len(r.Route.TCP) == 0 {
		return NewValidationError("route must specify at least one HTTP or TCP route")
	}

	// Validate HTTP routes
	for i, route := range r.Route.HTTP {
		if err := validateHTTPRoute(route, i); err != nil {
			return err
		}
	}

	// Validate TCP routes
	portSet := make(map[int]bool)
	for i, route := range r.Route.TCP {
		if route.Port <= 0 || route.Port > 65535 {
			return NewValidationError("tcp route at index " + strconv.Itoa(i) + " must have port between 1 and 65535")
		}

		if portSet[route.Port] {
			return NewValidationError("duplicate tcp port: " + strconv.Itoa(route.Port))
		}
		portSet[route.Port] = true

		if err := validateRouteDestination(route.Destination); err != nil {
			return NewValidationError("tcp route at index " + strconv.Itoa(i) + ": " + err.Error())
		}
	}

	return nil
}

// Helper function to validate HTTP routes
func validateHTTPRoute(route HTTPRoute, index int) error {
	// Validate path configuration
	if route.Path != "" && route.PathPrefix != "" {
		return NewValidationError("http route at index " + strconv.Itoa(index) + " cannot specify both path and pathPrefix")
	}

	// Validate destination
	if err := validateRouteDestination(route.Destination); err != nil {
		return NewValidationError("http route at index " + strconv.Itoa(index) + ": " + err.Error())
	}

	// Validate retry policy if specified
	if route.Retries != nil && route.Retries.Attempts <= 0 {
		return NewValidationError("http route at index " + strconv.Itoa(index) + " retry attempts must be greater than 0")
	}

	// Validate CORS policy if specified
	if route.CORS != nil && len(route.CORS.AllowOrigins) == 0 {
		return NewValidationError("http route at index " + strconv.Itoa(index) + " cors policy must specify at least one allowed origin")
	}

	return nil
}

// Helper function to validate route destinations
func validateRouteDestination(dest RouteDestination) error {
	if dest.Service == "" {
		return NewValidationError("destination must specify a service")
	}

	if dest.Port <= 0 || dest.Port > 65535 {
		return NewValidationError("destination port must be between 1 and 65535")
	}

	if dest.Weight < 0 || dest.Weight > 100 {
		return NewValidationError("destination weight must be between 0 and 100")
	}

	return nil
}

// ToGateway converts a GatewaySpec to a Gateway.
func (g *GatewaySpec) ToGateway() (*Gateway, error) {
	// Validate
	if err := g.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := g.Gateway.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &Gateway{
		ID:        uuid.New().String(),
		Name:      g.Gateway.Name,
		Namespace: namespace,
		Hostname:  g.Gateway.Hostname,
		Internal:  g.Gateway.Internal,
		Ports:     g.Gateway.Ports,
		TLS:       g.Gateway.TLS,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ToRoute converts a RouteSpec to a Route.
func (r *RouteSpec) ToRoute() (*Route, error) {
	// Validate
	if err := r.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := r.Route.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &Route{
		ID:        uuid.New().String(),
		Name:      r.Route.Name,
		Namespace: namespace,
		Gateway:   r.Route.Gateway,
		Hosts:     r.Route.Hosts,
		HTTP:      r.Route.HTTP,
		TCP:       r.Route.TCP,
		Priority:  r.Route.Priority,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
