package types

import (
	"context"
)

type InstanceFinalizerInterface interface {
	StopInstance(ctx context.Context, instance *Instance) error
	DeleteInstance(ctx context.Context, instance *Instance) error
}

type ServiceFinalizerInterface interface {
	DeleteService(ctx context.Context, service *Service) error
}

type HealthFinalizerInterface interface {
	RemoveInstance(instanceID string) error
}

// FinalizerInterface for cleanup operations
type FinalizerInterface interface {
	GetID() string
	GetType() FinalizerType
	Execute(ctx context.Context, service *Service) error
	GetDependencies() []FinalizerDependency
	Validate(service *Service) error
}
