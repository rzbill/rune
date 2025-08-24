package types

// PolicyRule defines a single permission rule within a policy
type PolicyRule struct {
	Resource  string   `json:"resource" yaml:"resource"`
	Verbs     []string `json:"verbs" yaml:"verbs"`
	Namespace string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// Policy represents a named set of permission rules
type Policy struct {
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"` // DNS-1123 unique name within a namespace
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Rules       []PolicyRule `json:"rules" yaml:"rules"`
	Builtin     bool         `json:"builtin" yaml:"builtin"`
}

func (p *Policy) GetID() string                 { return p.ID }
func (p *Policy) GetResourceType() ResourceType { return ResourceTypePolicy }
