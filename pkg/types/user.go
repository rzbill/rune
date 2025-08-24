package types

import "time"

// User represents an identity
type User struct {
	ID        string    `json:"id" yaml:"id"`
	Name      string    `json:"name" yaml:"name"`
	Email     string    `json:"email,omitempty" yaml:"email,omitempty"`
	Policies  []string  `json:"policies,omitempty" yaml:"policies,omitempty"`
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
}

func (u *User) GetID() string                 { return u.ID }
func (u *User) GetResourceType() ResourceType { return ResourceTypeUser }
