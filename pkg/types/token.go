package types

import "time"

// Token represents an authentication token (opaque secret stored hashed)
type Token struct {
	Namespace   string     `json:"namespace" yaml:"namespace"`
	Name        string     `json:"name" yaml:"name"`
	ID          string     `json:"id" yaml:"id"`
	SubjectID   string     `json:"subjectId" yaml:"subjectId"`
	SubjectType string     `json:"subjectType" yaml:"subjectType"` // "user" | "service"
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	IssuedAt    time.Time  `json:"issuedAt" yaml:"issuedAt"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	Revoked     bool       `json:"revoked" yaml:"revoked"`
	SecretHash  string     `json:"secretHash" yaml:"secretHash"`
}

func (t *Token) NamespacedName() NamespacedName {
	return NamespacedName{Namespace: t.Namespace, Name: t.Name}
}
func (t *Token) GetID() string                 { return t.ID }
func (t *Token) GetResourceType() ResourceType { return ResourceTypeToken }
