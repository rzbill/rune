package types

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Policy represents an RBAC policy that defines permissions.
type Policy struct {
	// Unique identifier for the policy
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the policy
	Name string `json:"name" yaml:"name"`

	// Namespace the policy belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Subject selectors (who this policy applies to)
	Subjects []Subject `json:"subjects" yaml:"subjects"`

	// Permissions granted by this policy
	Permissions []Permission `json:"permissions" yaml:"permissions"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// Subject represents a subject that a policy applies to.
type Subject struct {
	// User identifier (e.g., "alice@example.com")
	User string `json:"user,omitempty" yaml:"user,omitempty"`

	// Group identifier (e.g., "developers")
	Group string `json:"group,omitempty" yaml:"group,omitempty"`

	// Service identifier (e.g., "api")
	Service string `json:"service,omitempty" yaml:"service,omitempty"`

	// Namespace scope for service identity
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// Permission represents a set of actions allowed on resources.
type Permission struct {
	// Resource types this permission applies to (e.g., "service", "secret")
	Resources []string `json:"resources" yaml:"resources"`

	// Specific resource names this permission applies to (optional)
	ResourceNames []string `json:"resourceNames,omitempty" yaml:"resourceNames,omitempty"`

	// Actions allowed on these resources (e.g., "create", "read", "update")
	Actions []string `json:"actions" yaml:"actions"`

	// Namespaces this permission applies to (optional)
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
}

// NetworkPolicy represents a set of rules controlling network traffic.
type NetworkPolicy struct {
	// Unique identifier for the network policy
	ID string `json:"id" yaml:"id"`

	// Human-readable name for the network policy
	Name string `json:"name" yaml:"name"`

	// Namespace the network policy belongs to
	Namespace string `json:"namespace" yaml:"namespace"`

	// Selector for the workloads this policy applies to (standalone policy only)
	Selector map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`

	// Ingress rules (traffic coming into the selected workloads)
	Ingress []IngressRule `json:"ingress,omitempty" yaml:"ingress,omitempty"`

	// Egress rules (traffic going out from the selected workloads)
	Egress []EgressRule `json:"egress,omitempty" yaml:"egress,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// IngressRule defines allowed incoming traffic.
type IngressRule struct {
	// Sources allowed to connect to the selected workloads
	From []NetworkPolicyPeer `json:"from" yaml:"from"`

	// Ports that the traffic is allowed on
	Ports []string `json:"ports,omitempty" yaml:"ports,omitempty"`
}

// EgressRule defines allowed outgoing traffic.
type EgressRule struct {
	// Destinations that the selected workloads are allowed to connect to
	To []NetworkPolicyPeer `json:"to" yaml:"to"`

	// Ports that the traffic is allowed on
	Ports []string `json:"ports,omitempty" yaml:"ports,omitempty"`
}

// NetworkPolicyPeer defines a peer in network policy rules.
type NetworkPolicyPeer struct {
	// Service name to allow traffic from/to
	Service string `json:"service,omitempty" yaml:"service,omitempty"`

	// Namespace to allow all traffic from/to
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Service selector to match services by label
	ServiceSelector map[string]string `json:"serviceSelector,omitempty" yaml:"serviceSelector,omitempty"`

	// CIDR to allow traffic from/to
	CIDR string `json:"cidr,omitempty" yaml:"cidr,omitempty"`
}

// PolicySpec represents the YAML specification for an RBAC policy.
type PolicySpec struct {
	// Policy metadata and rules
	Policy struct {
		// Human-readable name for the policy (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the policy belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Subject selectors (who this policy applies to)
		Subjects []Subject `json:"subjects" yaml:"subjects"`

		// Permissions granted by this policy
		Permissions []Permission `json:"permissions" yaml:"permissions"`
	} `json:"policy" yaml:"policy"`
}

// NetworkPolicySpec represents the YAML specification for a network policy.
type NetworkPolicySpec struct {
	// NetworkPolicy metadata and rules
	NetworkPolicy struct {
		// Human-readable name for the network policy (required)
		Name string `json:"name" yaml:"name"`

		// Namespace the network policy belongs to (optional, defaults to "default")
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

		// Selector for the workloads this policy applies to (standalone policy only)
		Selector map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`

		// Ingress rules (traffic coming into the selected workloads)
		Ingress []IngressRule `json:"ingress,omitempty" yaml:"ingress,omitempty"`

		// Egress rules (traffic going out from the selected workloads)
		Egress []EgressRule `json:"egress,omitempty" yaml:"egress,omitempty"`
	} `json:"networkPolicy" yaml:"networkPolicy"`
}

// ServiceNetworkPolicy represents a network policy embedded in a service spec.
type ServiceNetworkPolicy struct {
	// Ingress rules (traffic coming into the service)
	Ingress []IngressRule `json:"ingress,omitempty" yaml:"ingress,omitempty"`

	// Egress rules (traffic going out from the service)
	Egress []EgressRule `json:"egress,omitempty" yaml:"egress,omitempty"`
}

// Validate checks if a policy specification is valid.
func (p *PolicySpec) Validate() error {
	if p.Policy.Name == "" {
		return NewValidationError("policy name is required")
	}

	if len(p.Policy.Subjects) == 0 {
		return NewValidationError("policy must have at least one subject")
	}

	if len(p.Policy.Permissions) == 0 {
		return NewValidationError("policy must have at least one permission")
	}

	// Validate each subject has at least one identifier
	for i, subject := range p.Policy.Subjects {
		if subject.User == "" && subject.Group == "" && subject.Service == "" {
			return NewValidationError("subject at index " + strconv.Itoa(i) + " must specify user, group, or service")
		}

		if subject.Service != "" && subject.Namespace == "" {
			return NewValidationError("service subject must specify a namespace")
		}
	}

	// Validate each permission has resources and actions
	for i, permission := range p.Policy.Permissions {
		if len(permission.Resources) == 0 {
			return NewValidationError("permission at index " + strconv.Itoa(i) + " must specify resources")
		}

		if len(permission.Actions) == 0 {
			return NewValidationError("permission at index " + strconv.Itoa(i) + " must specify actions")
		}
	}

	return nil
}

// Validate checks if a network policy specification is valid.
func (n *NetworkPolicySpec) Validate() error {
	if n.NetworkPolicy.Name == "" {
		return NewValidationError("network policy name is required")
	}

	// Validate each ingress rule
	for i, rule := range n.NetworkPolicy.Ingress {
		if len(rule.From) == 0 {
			return NewValidationError("ingress rule at index " + strconv.Itoa(i) + " must specify 'from' peers")
		}

		// Validate each peer has at least one identifier
		for j, peer := range rule.From {
			if peer.Service == "" && peer.Namespace == "" && len(peer.ServiceSelector) == 0 && peer.CIDR == "" {
				return NewValidationError("ingress peer at index " + strconv.Itoa(j) + " in rule " + strconv.Itoa(i) + " must specify service, namespace, serviceSelector, or cidr")
			}
		}
	}

	// Validate each egress rule
	for i, rule := range n.NetworkPolicy.Egress {
		if len(rule.To) == 0 {
			return NewValidationError("egress rule at index " + strconv.Itoa(i) + " must specify 'to' peers")
		}

		// Validate each peer has at least one identifier
		for j, peer := range rule.To {
			if peer.Service == "" && peer.Namespace == "" && len(peer.ServiceSelector) == 0 && peer.CIDR == "" {
				return NewValidationError("egress peer at index " + strconv.Itoa(j) + " in rule " + strconv.Itoa(i) + " must specify service, namespace, serviceSelector, or cidr")
			}
		}
	}

	return nil
}

// Validate checks if a service network policy is valid.
func (s *ServiceNetworkPolicy) Validate() error {
	// Validate each ingress rule
	for i, rule := range s.Ingress {
		if len(rule.From) == 0 {
			return NewValidationError("ingress rule at index " + strconv.Itoa(i) + " must specify 'from' peers")
		}

		// Validate each peer has at least one identifier
		for j, peer := range rule.From {
			if peer.Service == "" && peer.Namespace == "" && len(peer.ServiceSelector) == 0 && peer.CIDR == "" {
				return NewValidationError("ingress peer at index " + strconv.Itoa(j) + " in rule " + strconv.Itoa(i) + " must specify service, namespace, serviceSelector, or cidr")
			}
		}
	}

	// Validate each egress rule
	for i, rule := range s.Egress {
		if len(rule.To) == 0 {
			return NewValidationError("egress rule at index " + strconv.Itoa(i) + " must specify 'to' peers")
		}

		// Validate each peer has at least one identifier
		for j, peer := range rule.To {
			if peer.Service == "" && peer.Namespace == "" && len(peer.ServiceSelector) == 0 && peer.CIDR == "" {
				return NewValidationError("egress peer at index " + strconv.Itoa(j) + " in rule " + strconv.Itoa(i) + " must specify service, namespace, serviceSelector, or cidr")
			}
		}
	}

	return nil
}

// ToPolicy converts a PolicySpec to a Policy.
func (p *PolicySpec) ToPolicy() (*Policy, error) {
	// Validate
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := p.Policy.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &Policy{
		ID:          uuid.New().String(),
		Name:        p.Policy.Name,
		Namespace:   namespace,
		Subjects:    p.Policy.Subjects,
		Permissions: p.Policy.Permissions,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// ToNetworkPolicy converts a NetworkPolicySpec to a NetworkPolicy.
func (n *NetworkPolicySpec) ToNetworkPolicy() (*NetworkPolicy, error) {
	// Validate
	if err := n.Validate(); err != nil {
		return nil, err
	}

	// Set default namespace if not specified
	namespace := n.NetworkPolicy.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()

	return &NetworkPolicy{
		ID:        uuid.New().String(),
		Name:      n.NetworkPolicy.Name,
		Namespace: namespace,
		Selector:  n.NetworkPolicy.Selector,
		Ingress:   n.NetworkPolicy.Ingress,
		Egress:    n.NetworkPolicy.Egress,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
