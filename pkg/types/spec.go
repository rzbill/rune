package types

// Spec is a common interface implemented by all resource specs
// that can be validated and identify themselves by name/namespace/kind.
type Spec interface {
	// Validate ensures the spec is structurally and semantically valid.
	Validate() error
	// GetName returns the resource name declared in the spec.
	GetName() string
	// GetNamespace returns the resource namespace declared in the spec
	// (defaulting behavior may be applied by callers as needed).
	GetNamespace() string
	// Kind returns the logical resource kind (e.g., "Service", "Secret", "Config").
	Kind() string
}
