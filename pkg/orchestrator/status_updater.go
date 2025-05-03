package orchestrator

import (
	"context"
	"fmt"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

// StatusUpdater handles updating status fields for resources
type StatusUpdater interface {
	// UpdateServiceStatus updates just the status field for a service
	UpdateServiceStatus(ctx context.Context, namespace, name string, service *types.Service, status types.ServiceStatus) error

	// UpdateInstanceStatus updates just the status field for an instance
	UpdateInstanceStatus(ctx context.Context, namespace, name string, instance *types.Instance, status types.InstanceStatus) error
}

// DefaultStatusUpdater implements StatusUpdater using the Store
type DefaultStatusUpdater struct {
	store  store.Store
	logger log.Logger
}

// NewStatusUpdater creates a new DefaultStatusUpdater
func NewStatusUpdater(store store.Store, logger log.Logger) StatusUpdater {
	return &DefaultStatusUpdater{
		store:  store,
		logger: logger.WithComponent("status-updater"),
	}
}

// UpdateServiceStatus updates just the status field for a service
func (u *DefaultStatusUpdater) UpdateServiceStatus(ctx context.Context, namespace, name string, service *types.Service, status types.ServiceStatus) error {
	u.logger.Debug("Updating service status",
		log.Str("namespace", namespace),
		log.Str("name", name),
		log.Str("status", string(status)))

	// Get the current service to update just the status field
	var currentService types.Service
	if err := u.store.Get(ctx, types.ResourceTypeService, namespace, name, &currentService); err != nil {
		return fmt.Errorf("failed to get service for status update: %w", err)
	}

	// Update only the status field
	currentService.Status = status

	// Update the service with source tag
	err := u.store.Update(ctx, types.ResourceTypeService, namespace, name, &currentService,
		store.WithSource(store.EventSourceOrchestrator))
	if err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	return nil
}

// UpdateInstanceStatus updates just the status field for an instance
func (u *DefaultStatusUpdater) UpdateInstanceStatus(ctx context.Context, namespace, name string, instance *types.Instance, status types.InstanceStatus) error {
	u.logger.Debug("Updating instance status",
		log.Str("namespace", namespace),
		log.Str("name", name),
		log.Str("status", string(status)))

	// Get the current instance to update just the status field
	var currentInstance types.Instance
	if err := u.store.Get(ctx, types.ResourceTypeInstance, namespace, name, &currentInstance); err != nil {
		return fmt.Errorf("failed to get instance for status update: %w", err)
	}

	// Update only the status field
	currentInstance.Status = status

	// Update the instance with source tag
	err := u.store.Update(ctx, types.ResourceTypeInstance, namespace, name, &currentInstance,
		store.WithSource(store.EventSourceOrchestrator))
	if err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}

	return nil
}
