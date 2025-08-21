package types

import "time"

// User represents an identity
type User struct {
	Namespace string    `json:"namespace" yaml:"namespace"`
	Name      string    `json:"name" yaml:"name"`
	ID        string    `json:"id" yaml:"id"`
	Email     string    `json:"email,omitempty" yaml:"email,omitempty"`
	Policies  []string  `json:"policies,omitempty" yaml:"policies,omitempty"`
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
}

func (u *User) NamespacedName() NamespacedName {
	return NamespacedName{Namespace: u.Namespace, Name: u.Name}
}
func (u *User) GetID() string                 { return u.ID }
func (u *User) GetResourceType() ResourceType { return ResourceTypeUser }
