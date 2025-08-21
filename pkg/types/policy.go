package types

// PolicyRule defines a single permission rule within a policy
type PolicyRule struct {
	Resource  string   `json:"resource" yaml:"resource"`
	Verbs     []string `json:"verbs" yaml:"verbs"`
	Namespace string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// Policy represents a named set of permission rules
type Policy struct {
	Namespace   string       `json:"namespace" yaml:"namespace"`
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Rules       []PolicyRule `json:"rules" yaml:"rules"`
	Builtin     bool         `json:"builtin" yaml:"builtin"`
}

func (p *Policy) NamespacedName() NamespacedName {
	return NamespacedName{Namespace: p.Namespace, Name: p.Name}
}
func (p *Policy) GetID() string                 { return p.ID }
func (p *Policy) GetResourceType() ResourceType { return ResourceTypePolicy }
