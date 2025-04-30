package orchestrator

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner/manager"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

// Orchestrator manages service lifecycle and coordinates with runners
type Orchestrator interface {
	// Start the orchestrator with the given context
	Start(ctx context.Context) error

	// Stop the orchestrator and clean up resources
	Stop() error

	// GetServiceStatus returns the current status of a service
	GetServiceStatus(ctx context.Context, namespace, name string) (*ServiceStatusInfo, error)

	// GetInstanceStatus returns the current status of an instance
	GetInstanceStatus(ctx context.Context, namespace, serviceName, instanceID string) (*InstanceStatusInfo, error)

	// GetServiceLogs returns a stream of logs for a service
	GetServiceLogs(ctx context.Context, namespace, name string, opts LogOptions) (io.ReadCloser, error)

	// GetInstanceLogs returns a stream of logs for an instance
	GetInstanceLogs(ctx context.Context, namespace, serviceName, instanceID string, opts LogOptions) (io.ReadCloser, error)

	// ExecInService executes a command in a running instance of the service
	// If multiple instances exist, one will be chosen
	ExecInService(ctx context.Context, namespace, serviceName string, options ExecOptions) (ExecStream, error)

	// ExecInInstance executes a command in a specific instance
	ExecInInstance(ctx context.Context, namespace, serviceName, instanceID string, options ExecOptions) (ExecStream, error)

	// RestartService restarts all instances of a service
	RestartService(ctx context.Context, namespace, serviceName string) error

	// RestartInstance restarts a specific instance
	RestartInstance(ctx context.Context, namespace, serviceName, instanceID string) error
}

// orchestrator implements the Orchestrator interface
type orchestrator struct {
	store              store.Store
	instanceController InstanceController
	healthController   HealthController
	reconciler         *reconciler
	logger             log.Logger

	// Context for background operations
	ctx    context.Context
	cancel context.CancelFunc

	// WaitGroup for goroutines
	wg sync.WaitGroup

	// Watch channel for services
	watchCh <-chan store.WatchEvent
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(store store.Store, instanceController InstanceController, healthController HealthController, runnerManager manager.IRunnerManager, logger log.Logger) Orchestrator {
	// Create the reconciler
	reconciler := newReconciler(
		store,
		instanceController,
		healthController,
		runnerManager,
		logger,
	)

	return &orchestrator{
		store:              store,
		instanceController: instanceController,
		healthController:   healthController,
		reconciler:         reconciler,
		logger:             logger.WithComponent("orchestrator"),
	}
}

// Start the orchestrator
func (o *orchestrator) Start(ctx context.Context) error {
	o.logger.Info("Starting orchestrator")

	// Create a context with cancel for all background operations
	o.ctx, o.cancel = context.WithCancel(ctx)

	// Start the reconciler
	if err := o.reconciler.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start reconciler: %w", err)
	}

	// Start health controller
	if err := o.healthController.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start health controller: %w", err)
	}

	// Start watching services
	watchCh, err := o.store.Watch(o.ctx, "services", "")
	if err != nil {
		return fmt.Errorf("failed to watch services: %w", err)
	}
	o.watchCh = watchCh

	// Start a goroutine to process service events
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		o.watchServices()
	}()

	return nil
}

// Stop the orchestrator
func (o *orchestrator) Stop() error {
	o.logger.Info("Stopping orchestrator")

	// Cancel context to stop all operations
	if o.cancel != nil {
		o.cancel()
	}

	// Stop reconciler
	o.reconciler.Stop()

	// Stop health controller
	if err := o.healthController.Stop(); err != nil {
		o.logger.Error("Failed to stop health controller", log.Err(err))
	}

	// Wait for all goroutines to finish
	o.wg.Wait()

	o.logger.Info("Orchestrator stopped")
	return nil
}

// watchServices watches service events and processes them
func (o *orchestrator) watchServices() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case event, ok := <-o.watchCh:
			if !ok {
				o.logger.Error("Service watch channel closed, restarting watch")
				// Try to restart the watch
				watchCh, err := o.store.Watch(o.ctx, "services", "")
				if err != nil {
					o.logger.Error("Failed to restart service watch", log.Err(err))
					time.Sleep(5 * time.Second) // Backoff before retry
					continue
				}
				o.watchCh = watchCh
				continue
			}

			// Process the event
			switch event.Type {
			case store.WatchEventCreated:
				o.logger.Info("Service created",
					log.Str("name", event.Name),
					log.Str("namespace", event.Namespace))

				// Type assert to *types.Service
				service, ok := event.Resource.(*types.Service)
				if !ok {
					o.logger.Error("Failed to convert resource to Service",
						log.Str("name", event.Name),
						log.Str("namespace", event.Namespace))
					continue
				}
				o.handleServiceCreated(o.ctx, service)

			case store.WatchEventUpdated:
				o.logger.Info("Service updated",
					log.Str("name", event.Name),
					log.Str("namespace", event.Namespace))

				// Type assert to *types.Service
				service, ok := event.Resource.(*types.Service)
				if !ok {
					o.logger.Error("Failed to convert resource to Service",
						log.Str("name", event.Name),
						log.Str("namespace", event.Namespace))
					continue
				}
				o.handleServiceUpdated(o.ctx, service)

			case store.WatchEventDeleted:
				o.logger.Info("Service deleted",
					log.Str("name", event.Name),
					log.Str("namespace", event.Namespace))

				// Type assert to *types.Service
				service, ok := event.Resource.(*types.Service)
				if !ok {
					o.logger.Error("Failed to convert resource to Service",
						log.Str("name", event.Name),
						log.Str("namespace", event.Namespace))
					continue
				}
				o.handleServiceDeleted(o.ctx, service)
			}
		}
	}
}

// handleServiceCreated handles service creation events
func (o *orchestrator) handleServiceCreated(ctx context.Context, service *types.Service) {
	// Set initial service state
	service.Status = types.ServiceStatusPending

	// Update service status in store
	if err := o.store.Update(ctx, types.ResourceTypeService, service.Namespace, service.Name, service); err != nil {
		o.logger.Error("Failed to update service status",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return
	}

	// Schedule reconciliation for this service using the reconciler
	go func() {
		// Collect running instances for reconciliation
		runningInstances, err := o.reconciler.collectRunningInstances(ctx)
		if err != nil {
			o.logger.Error("Failed to collect running instances",
				log.Str("name", service.Name),
				log.Str("namespace", service.Namespace),
				log.Err(err))
			return
		}

		// Use the reconciler to handle this service
		if err := o.reconciler.reconcileService(ctx, service, runningInstances); err != nil {
			o.logger.Error("Failed to reconcile service",
				log.Str("name", service.Name),
				log.Str("namespace", service.Namespace),
				log.Err(err))
		}
	}()
}

// handleServiceUpdated handles service update events
func (o *orchestrator) handleServiceUpdated(ctx context.Context, service *types.Service) {
	// Schedule reconciliation for this service using the reconciler
	go func() {
		// Collect running instances for reconciliation
		runningInstances, err := o.reconciler.collectRunningInstances(ctx)
		if err != nil {
			o.logger.Error("Failed to collect running instances",
				log.Str("name", service.Name),
				log.Str("namespace", service.Namespace),
				log.Err(err))
			return
		}

		// Use the reconciler to handle this service
		if err := o.reconciler.reconcileService(ctx, service, runningInstances); err != nil {
			o.logger.Error("Failed to reconcile updated service",
				log.Str("name", service.Name),
				log.Str("namespace", service.Namespace),
				log.Err(err))
		}
	}()
}

// handleServiceDeleted handles service deletion events
func (o *orchestrator) handleServiceDeleted(ctx context.Context, service *types.Service) {
	// List all instances for this service
	instances, err := o.listInstancesForService(ctx, service.Namespace, service.Name)
	if err != nil {
		o.logger.Error("Failed to list instances for deleted service",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return
	}

	// Mark the service as deleted
	service.Status = types.ServiceStatusDeleted

	// Delete all instances
	for _, instance := range instances {
		if err := o.instanceController.DeleteInstance(ctx, instance); err != nil {
			o.logger.Error("Failed to delete instance for removed service",
				log.Str("name", service.Name),
				log.Str("namespace", service.Namespace),
				log.Str("instance", instance.ID),
				log.Err(err))
		}

		// Remove from health monitoring
		o.healthController.RemoveInstance(instance.ID)

		// Remove from store
		if err := o.store.Delete(ctx, "instances", service.Namespace, instance.ID); err != nil {
			o.logger.Error("Failed to remove instance from store",
				log.Str("instance", instance.ID),
				log.Err(err))
		}
	}
}

// GetServiceStatus returns the current status of a service
func (o *orchestrator) GetServiceStatus(ctx context.Context, namespace, name string) (*ServiceStatusInfo, error) {
	// Get service from store
	var service types.Service
	if err := o.store.Get(ctx, types.ResourceTypeService, namespace, name, &service); err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// List instances for this service
	instances, err := o.listInstancesForService(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	status := &ServiceStatusInfo{
		State:         service.Status,
		InstanceCount: len(instances),
	}

	// Count ready instances
	for _, instance := range instances {
		if instance.Status == types.InstanceStatusRunning {
			status.ReadyInstanceCount++
		}
	}

	return status, nil
}

// GetInstanceStatus returns the current status of an instance
func (o *orchestrator) GetInstanceStatus(ctx context.Context, namespace, serviceName, instanceName string) (*InstanceStatusInfo, error) {
	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceName, &instance); err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceID != serviceName {
		return nil, fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	return &InstanceStatusInfo{
		State:      instance.Status,
		InstanceID: instance.ID,
		NodeID:     instance.NodeID,
		CreatedAt:  instance.CreatedAt,
	}, nil
}

// GetServiceLogs returns a stream of logs for a service
func (o *orchestrator) GetServiceLogs(ctx context.Context, namespace, name string, opts LogOptions) (io.ReadCloser, error) {
	// List instances for this service
	instances, err := o.listInstancesForService(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances found for service %s in namespace %s", name, namespace)
	}

	// Collect log streams from all instances
	logInfos := make([]InstanceLogInfo, 0, len(instances))
	for _, instance := range instances {
		logReader, err := o.GetInstanceLogs(ctx, namespace, name, instance.Name, opts)
		if err != nil {
			o.logger.Warn("Failed to get logs for instance",
				log.Str("service", name),
				log.Str("namespace", namespace),
				log.Str("instance", instance.Name),
				log.Err(err))
			// Continue with other instances even if one fails
			continue
		}
		logInfos = append(logInfos, InstanceLogInfo{
			InstanceID: instance.ID,
			Reader:     logReader,
		})
	}

	if len(logInfos) == 0 {
		return nil, fmt.Errorf("failed to get logs from any instance of service %s in namespace %s", name, namespace)
	}

	// If only one reader is available, return it directly without metadata
	if len(logInfos) == 1 {
		return logInfos[0].Reader, nil
	}

	// Create a multi-instance log streamer to combine logs
	return NewMultiLogStreamer(logInfos, true), nil
}

// GetInstanceLogs returns a stream of logs for an instance
func (o *orchestrator) GetInstanceLogs(ctx context.Context, namespace, serviceName, instanceName string, opts LogOptions) (io.ReadCloser, error) {
	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceName, &instance); err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceID != serviceName {
		return nil, fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	// Get logs from instance controller
	return o.instanceController.GetInstanceLogs(ctx, &instance, opts)
}

// ExecInService executes a command in a running instance of the service
func (o *orchestrator) ExecInService(ctx context.Context, namespace, serviceName string, options ExecOptions) (ExecStream, error) {
	// List instances for this service
	instances, err := o.listInstancesForService(ctx, namespace, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances found for service %s in namespace %s", serviceName, namespace)
	}

	// Find a running instance
	var runningInstance *types.Instance
	for _, instance := range instances {
		if instance.Status == types.InstanceStatusRunning {
			runningInstance = instance
			break
		}
	}

	if runningInstance == nil {
		return nil, fmt.Errorf("no running instances found for service %s in namespace %s", serviceName, namespace)
	}

	// Execute command in the selected instance
	return o.instanceController.Exec(ctx, runningInstance, options)
}

// ExecInInstance executes a command in a specific instance
func (o *orchestrator) ExecInInstance(ctx context.Context, namespace, serviceName, instanceID string, options ExecOptions) (ExecStream, error) {
	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceID != serviceName {
		return nil, fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	// Verify instance is running
	if instance.Status != types.InstanceStatusRunning {
		return nil, fmt.Errorf("instance %s is not running, current status: %s", instanceID, instance.Status)
	}

	// Execute command in the instance
	return o.instanceController.Exec(ctx, &instance, options)
}

// listInstancesForService lists all instances for a service
func (o *orchestrator) listInstancesForService(ctx context.Context, namespace, serviceName string) ([]*types.Instance, error) {
	// Get all instances
	var instances []types.Instance
	err := o.store.List(ctx, "instances", namespace, &instances)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	// Filter instances for this service
	filteredInstances := make([]*types.Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.ServiceID == serviceName {
			filteredInstances = append(filteredInstances, &instance)
		}
	}

	return filteredInstances, nil
}

// GetInstanceController returns the instance controller for testing purposes
func (o *orchestrator) GetInstanceController() InstanceController {
	return o.instanceController
}

// NewDefaultOrchestrator creates a new orchestrator with default components
func NewDefaultOrchestrator(store store.Store, logger log.Logger, runnerManager manager.IRunnerManager) (Orchestrator, error) {
	// Create the instance controller
	instanceController := NewInstanceController(store, runnerManager, logger)

	// Create the health controller
	healthController := NewHealthController(logger, store, runnerManager)

	// Setup the instance controller reference for health controller
	if hc, ok := healthController.(interface{ SetInstanceController(InstanceController) }); ok {
		hc.SetInstanceController(instanceController)
	} else {
		logger.Warn("Health controller does not support SetInstanceController method")
	}

	// Create and return the orchestrator
	return NewOrchestrator(store, instanceController, healthController, runnerManager, logger), nil
}

// RestartService restarts all instances of a service
func (o *orchestrator) RestartService(ctx context.Context, namespace, serviceName string) error {
	o.logger.Info("Restarting service",
		log.Str("namespace", namespace),
		log.Str("service", serviceName))

	// List instances for this service
	instances, err := o.listInstancesForService(ctx, namespace, serviceName)
	if err != nil {
		return fmt.Errorf("failed to list instances for service: %w", err)
	}

	if len(instances) == 0 {
		return fmt.Errorf("no instances found for service %s in namespace %s", serviceName, namespace)
	}

	// Restart each instance
	var lastError error
	for _, instance := range instances {
		if err := o.RestartInstance(ctx, namespace, serviceName, instance.ID); err != nil {
			o.logger.Error("Failed to restart instance",
				log.Str("namespace", namespace),
				log.Str("service", serviceName),
				log.Str("instance", instance.ID),
				log.Err(err))
			lastError = err
			// Continue with other instances even if one fails
		}
	}

	// Return the last error encountered, if any
	if lastError != nil {
		return fmt.Errorf("one or more instances failed to restart: %w", lastError)
	}

	o.logger.Info("Successfully restarted service",
		log.Str("namespace", namespace),
		log.Str("service", serviceName))
	return nil
}

// RestartInstance restarts a specific instance
func (o *orchestrator) RestartInstance(ctx context.Context, namespace, serviceName, instanceID string) error {
	o.logger.Info("Restarting instance",
		log.Str("namespace", namespace),
		log.Str("service", serviceName),
		log.Str("instance", instanceID))

	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceID != serviceName {
		return fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	// Restart the instance through instance controller
	if err := o.instanceController.RestartInstance(ctx, &instance, InstanceRestartReasonManual); err != nil {
		return fmt.Errorf("failed to restart instance: %w", err)
	}

	o.logger.Info("Successfully restarted instance",
		log.Str("namespace", namespace),
		log.Str("service", serviceName),
		log.Str("instance", instanceID))
	return nil
}
