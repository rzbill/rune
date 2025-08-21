package types

import "strconv"

// ServiceNetworkPolicy represents a network policy embedded in a service spec.
type ServiceNetworkPolicy struct {
	// Ingress rules (traffic coming into the service)
	Ingress []IngressRule `json:"ingress,omitempty" yaml:"ingress,omitempty"`

	// Egress rules (traffic going out from the service)
	Egress []EgressRule `json:"egress,omitempty" yaml:"egress,omitempty"`
}

// Validate checks if a service network policy is valid.
func (s *ServiceNetworkPolicy) Validate() error {
	// Validate each ingress rule
	for i := range s.Ingress {
		if len(s.Ingress[i].From) == 0 {
			return NewValidationError("ingress rule at index " + strconv.Itoa(i) + " must specify 'from' peers")
		}
		for j := range s.Ingress[i].From {
			p := s.Ingress[i].From[j]
			if p.Service == "" && p.Namespace == "" && len(p.ServiceSelector) == 0 && p.CIDR == "" {
				return NewValidationError("ingress peer at index " + strconv.Itoa(j) + " in rule " + strconv.Itoa(i) + " must specify service, namespace, serviceSelector, or cidr")
			}
		}
	}
	// Validate each egress rule
	for i := range s.Egress {
		if len(s.Egress[i].To) == 0 {
			return NewValidationError("egress rule at index " + strconv.Itoa(i) + " must specify 'to' peers")
		}
		for j := range s.Egress[i].To {
			p := s.Egress[i].To[j]
			if p.Service == "" && p.Namespace == "" && len(p.ServiceSelector) == 0 && p.CIDR == "" {
				return NewValidationError("egress peer at index " + strconv.Itoa(j) + " in rule " + strconv.Itoa(i) + " must specify service, namespace, serviceSelector, or cidr")
			}
		}
	}
	return nil
}

type IngressRule struct {
	From  []NetworkPolicyPeer `json:"from" yaml:"from"`
	Ports []string            `json:"ports,omitempty" yaml:"ports,omitempty"`
}

type EgressRule struct {
	To    []NetworkPolicyPeer `json:"to" yaml:"to"`
	Ports []string            `json:"ports,omitempty" yaml:"ports,omitempty"`
}

type NetworkPolicyPeer struct {
	Service         string            `json:"service,omitempty" yaml:"service,omitempty"`
	Namespace       string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ServiceSelector map[string]string `json:"serviceSelector,omitempty" yaml:"serviceSelector,omitempty"`
	CIDR            string            `json:"cidr,omitempty" yaml:"cidr,omitempty"`
}
