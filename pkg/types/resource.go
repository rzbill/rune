package types

import "fmt"

// Resource is a generic interface that all resources must implement
type Resource interface {
	// String returns a unique identifier for the resource
	String() string

	// Equals checks if two resources are functionally equivalent
	Equals(other Resource) bool
}

type NamespacedResource interface {
	NamespacedName() NamespacedName
	GetID() string // For resources that also have an ID
	GetResourceType() ResourceType
}

// NamespacedName is a struct that contains a namespace and a name.
type NamespacedName struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Name      string `json:"name" yaml:"name"`
}

func (n *NamespacedName) NamespacedName() NamespacedName {
	return NamespacedName{
		Namespace: n.Namespace,
		Name:      n.Name,
	}
}

func (n *NamespacedName) GetName() string {
	return n.Name
}

func (n *NamespacedName) String() string {
	return fmt.Sprintf("%s/%s", n.Namespace, n.Name)
}
