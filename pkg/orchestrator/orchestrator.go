package orchestrator

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/orchestrator/controllers"
	"github.com/rzbill/rune/pkg/orchestrator/queue"
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
	GetServiceStatus(ctx context.Context, namespace, name string) (*types.ServiceStatusInfo, error)

	// GetInstanceStatus returns the current status of an instance
	GetInstanceStatus(ctx context.Context, namespace, serviceName, instanceID string) (*types.InstanceStatusInfo, error)

	// GetServiceLogs returns a stream of logs for a service
	GetServiceLogs(ctx context.Context, namespace, name string, opts types.LogOptions) (io.ReadCloser, error)

	// GetInstanceLogs returns a stream of logs for an instance
	GetInstanceLogs(ctx context.Context, namespace, instanceID string, opts types.LogOptions) (io.ReadCloser, error)

	// ExecInService executes a command in a running instance of the service
	// If multiple instances exist, one will be chosen
	ExecInService(ctx context.Context, namespace, serviceName string, options types.ExecOptions) (types.ExecStream, error)

	// ExecInInstance executes a command in a specific instance
	ExecInInstance(ctx context.Context, namespace, serviceName, instanceID string, options types.ExecOptions) (types.ExecStream, error)

	// RestartService restarts all instances of a service
	RestartService(ctx context.Context, namespace, serviceName string) error

	// RestartInstance restarts a specific instance
	RestartInstance(ctx context.Context, namespace, serviceName, instanceID string) error

	// StopService stops all instances of a service but keeps them in the store
	StopService(ctx context.Context, namespace, serviceName string) error

	// StopInstance stops a specific instance but keeps it in the store
	StopInstance(ctx context.Context, namespace, serviceName, instanceID string) error
}

// orchestrator implements the Orchestrator interface
type orchestrator struct {
	store              store.Store
	instanceController controllers.InstanceController
	healthController   controllers.HealthController
	reconciler         *reconciler
	logger             log.Logger
	statusUpdater      StatusUpdater

	// Context for background operations
	ctx    context.Context
	cancel context.CancelFunc

	// WaitGroup for goroutines
	wg sync.WaitGroup

	// Watch channel for services
	watchCh <-chan store.WatchEvent

	// Worker queue for processing service events
	workerQueue *queue.WorkerQueue

	// For tracking observed generations to prevent reconciliation loops
	serviceObservedGenerations map[string]int64
	serviceObservedLock        sync.RWMutex

	// For tracking internal updates to prevent reconciliation loops
	internalUpdates sync.Map
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(store store.Store, instanceController controllers.InstanceController, healthController controllers.HealthController, runnerManager manager.IRunnerManager, logger log.Logger) Orchestrator {
	// Create the reconciler
	reconciler := newReconciler(
		store,
		instanceController,
		healthController,
		runnerManager,
		logger,
	)

	// Create the status updater
	statusUpdater := NewStatusUpdater(store, logger)

	return &orchestrator{
		store:                      store,
		instanceController:         instanceController,
		healthController:           healthController,
		reconciler:                 reconciler,
		logger:                     logger.WithComponent("orchestrator"),
		statusUpdater:              statusUpdater,
		serviceObservedGenerations: make(map[string]int64),
	}
}

// NewDefaultOrchestrator creates a new orchestrator with default components
func NewDefaultOrchestrator(store store.Store, logger log.Logger, runnerManager manager.IRunnerManager) (Orchestrator, error) {
	// Create the instance controller
	instanceController := controllers.NewInstanceController(store, runnerManager, logger)

	// Create the health controller
	healthController := controllers.NewHealthController(logger, store, runnerManager)
	healthController.SetInstanceController(instanceController)

	// Create and return the orchestrator
	return NewOrchestrator(store, instanceController, healthController, runnerManager, logger), nil
}

// Start the orchestrator
func (o *orchestrator) Start(ctx context.Context) error {
	o.logger.Info("Starting orchestrator")

	// Create a context with cancel for all background operations
	o.ctx, o.cancel = context.WithCancel(ctx)

	// Initialize the worker queue
	o.workerQueue = queue.NewWorkerQueue(queue.WorkerQueueOptions{
		Name:            "orchestrator",
		Workers:         10, // Configurable worker count
		ProcessFunc:     o.processWorkItem,
		Logger:          o.logger,
		RateLimiterType: queue.RateLimiterTypeDefault,
		EnableMetrics:   true,
	})

	// Start the worker queue
	if err := o.workerQueue.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start worker queue: %w", err)
	}

	// Start the reconciler
	if err := o.reconciler.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start reconciler: %w", err)
	}

	// Start health controller
	if err := o.healthController.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start health controller: %w", err)
	}

	// Start watching services
	watchCh, err := o.store.Watch(o.ctx, types.ResourceTypeService, "")
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

	// Stop worker queue
	if o.workerQueue != nil {
		o.workerQueue.Stop()
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

// watchServices watches service events and adds them to the worker queue
func (o *orchestrator) watchServices() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case event, ok := <-o.watchCh:
			if !ok {
				o.logger.Error("Service watch channel closed, restarting watch")
				// Try to restart the watch
				watchCh, err := o.store.Watch(o.ctx, types.ResourceTypeService, "")
				if err != nil {
					o.logger.Error("Failed to restart service watch", log.Err(err))
					time.Sleep(5 * time.Second) // Backoff before retry
					continue
				}
				o.watchCh = watchCh
				continue
			}

			// Check if this is an internal update to avoid reconciliation loops
			if o.isInternalUpdate(event) {
				o.logger.Debug("Ignoring internally triggered update",
					log.Any("resource_type", event.ResourceType),
					log.Str("namespace", event.Namespace),
					log.Str("name", event.Name),
					log.Str("source", string(event.Source)))
				continue
			}

			// Create a key for the work item
			key := buildWorkItemKey(event)

			// Add to worker queue
			o.logger.Debug("Enqueuing service event",
				log.Str("key", key),
				log.Str("type", string(event.Type)),
				log.Str("name", event.Name),
				log.Str("namespace", event.Namespace))

			// Add event to queue context
			o.workerQueue.Enqueue(key)
		}
	}
}

// processWorkItem processes a single work item from the queue
func (o *orchestrator) processWorkItem(key string) error {
	// Parse the key to extract resource type, namespace, name, and event type
	workItemKey, err := parseWorkItemKey(key)
	if err != nil {
		return fmt.Errorf("invalid work item key format: %s, error: %w", key, err)
	}

	o.logger.Debug("Processing work item",
		log.Any("resourceType", workItemKey.ResourceType),
		log.Str("namespace", workItemKey.Namespace),
		log.Str("name", workItemKey.Name),
		log.Str("eventType", workItemKey.EventType))

	// Get the resource from the store
	var service types.Service
	if err := o.store.Get(o.ctx, workItemKey.ResourceType, workItemKey.Namespace, workItemKey.Name, &service); err != nil {
		// If the resource doesn't exist and this is a delete event, that's ok
		if workItemKey.EventType == string(store.WatchEventDeleted) {
			return nil
		}
		return fmt.Errorf("failed to get resource %s/%s/%s: %w", workItemKey.ResourceType, workItemKey.Namespace, workItemKey.Name, err)
	}

	var processErr error

	// Process based on event type
	switch store.WatchEventType(workItemKey.EventType) {
	case store.WatchEventCreated:
		processErr = o.handleServiceCreated(o.ctx, &service)
	case store.WatchEventUpdated:
		processErr = o.handleServiceUpdated(o.ctx, &service)
	case store.WatchEventDeleted:
		o.handleServiceDeleted(o.ctx, &service)
		processErr = nil // Explicitly set to nil since handleServiceDeleted doesn't return an error
	default:
		return fmt.Errorf("unknown event type: %s", workItemKey.EventType)
	}

	// If we successfully processed the work item and it was an update,
	// record the observed generation to avoid reprocessing
	if processErr == nil && store.WatchEventType(workItemKey.EventType) == store.WatchEventUpdated {
		o.recordObservedGeneration(service.Namespace, service.Name, service.Metadata.Generation)
		o.logger.Debug("Successfully processed event and recorded generation",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Int64("generation", service.Metadata.Generation),
			log.Json("event", workItemKey))
	}

	return processErr
}

// isInternalUpdate checks if an event was triggered by the orchestrator itself
func (o *orchestrator) isInternalUpdate(event store.WatchEvent) bool {
	// Check if event has an orchestrator source
	if event.Source == "orchestrator" {
		return true
	}

	// Check if we have a record of this being an internal update
	key := buildWorkItemKey(event)
	_, exists := o.internalUpdates.Load(key)
	return exists
}

// inProgressUpdate marks an update as being in progress to avoid reconciliation loops
func (o *orchestrator) inProgressUpdate(resourceType types.ResourceType, namespace, name string) {
	key := fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
	o.internalUpdates.Store(key, time.Now())
}

// clearInProgressUpdate clears an update marked as in progress
func (o *orchestrator) clearInProgressUpdate(resourceType types.ResourceType, namespace, name string) {
	key := fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
	o.internalUpdates.Delete(key)
}

// recordObservedGeneration records the observed generation for a service
func (o *orchestrator) recordObservedGeneration(namespace, name string, generation int64) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	o.serviceObservedLock.Lock()
	defer o.serviceObservedLock.Unlock()
	o.serviceObservedGenerations[key] = generation
}

// getObservedGeneration gets the observed generation for a service
func (o *orchestrator) getObservedGeneration(namespace, name string) (int64, bool) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	o.serviceObservedLock.RLock()
	defer o.serviceObservedLock.RUnlock()
	gen, exists := o.serviceObservedGenerations[key]
	return gen, exists
}

// isStatusOnlyChange checks if only the status was updated (not the spec)
func (o *orchestrator) isStatusOnlyChange(service *types.Service) bool {
	// Check if we've observed this generation before
	lastObserved, exists := o.getObservedGeneration(service.Namespace, service.Name)
	if !exists {
		return false // We haven't seen this service before
	}

	o.logger.Debug("Checking if status-only change",
		log.Str("namespace", service.Namespace),
		log.Str("name", service.Name),
		log.Int64("last_observed", lastObserved),
		log.Int64("current_generation", service.Metadata.Generation),
		log.Json("service", service))

	// If the generation hasn't changed, it's a status-only update
	return lastObserved == service.Metadata.Generation
}

// updateServiceStatus updates a service's status while preventing reconciliation loops
func (o *orchestrator) updateServiceStatus(ctx context.Context, service *types.Service, status types.ServiceStatus) error {
	// Mark this as an internal update
	o.inProgressUpdate(types.ResourceTypeService, service.Namespace, service.Name)
	defer o.clearInProgressUpdate(types.ResourceTypeService, service.Namespace, service.Name)

	// Use the status updater to update the service status
	err := o.statusUpdater.UpdateServiceStatus(ctx, service.Namespace, service.Name, service, status)
	if err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	return nil
}

// handleServiceCreated handles service creation events
func (o *orchestrator) handleServiceCreated(ctx context.Context, service *types.Service) error {
	// Set initial service state
	service.Status = types.ServiceStatusPending

	o.logger.Info("Service created",
		log.Str("name", service.Name),
		log.Str("namespace", service.Namespace))

	// Update service status in store
	if err := o.updateServiceStatus(ctx, service, types.ServiceStatusPending); err != nil {
		o.logger.Error("Failed to update service status",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return err
	}

	// Collect running instances for reconciliation
	runningInstances, err := o.reconciler.collectRunningInstances(ctx)
	if err != nil {
		o.logger.Error("Failed to collect running instances",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return err
	}

	// Use the reconciler to handle this service
	if err := o.reconciler.reconcileService(ctx, service, runningInstances); err != nil {
		o.logger.Error("Failed to reconcile service",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return err
	}

	return nil
}

// handleServiceUpdated handles service update events
func (o *orchestrator) handleServiceUpdated(ctx context.Context, service *types.Service) error {
	o.logger.Info("Service updated",
		log.Str("name", service.Name),
		log.Str("namespace", service.Namespace))

	// Check if this is a status-only change
	if o.isStatusOnlyChange(service) {
		o.logger.Debug("Skipping reconciliation for status-only change",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Int64("generation", service.Metadata.Generation))
		return nil
	}

	// Mark as in progress
	o.inProgressUpdate(types.ResourceTypeService, service.Namespace, service.Name)
	defer o.clearInProgressUpdate(types.ResourceTypeService, service.Namespace, service.Name)

	// Collect running instances for reconciliation
	runningInstances, err := o.reconciler.collectRunningInstances(ctx)
	if err != nil {
		o.logger.Error("Failed to collect running instances",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return err
	}

	// Use the reconciler to handle this service
	if err := o.reconciler.reconcileService(ctx, service, runningInstances); err != nil {
		o.logger.Error("Failed to reconcile updated service",
			log.Str("name", service.Name),
			log.Str("namespace", service.Namespace),
			log.Err(err))
		return err
	}

	// The recordObservedGeneration call has been moved to processWorkItem

	return nil
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
		if err := o.store.Delete(ctx, types.ResourceTypeInstance, service.Namespace, instance.ID); err != nil {
			o.logger.Error("Failed to remove instance from store",
				log.Str("instance", instance.ID),
				log.Err(err))
		}
	}
}

// GetServiceStatus returns the current status of a service
func (o *orchestrator) GetServiceStatus(ctx context.Context, namespace, name string) (*types.ServiceStatusInfo, error) {
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

	observedGeneration, exists := o.getObservedGeneration(namespace, name)
	if !exists {
		observedGeneration = 0
	}

	status := &types.ServiceStatusInfo{
		Status:             service.Status,
		DesiredInstances:   service.Scale,
		ObservedGeneration: observedGeneration,
	}

	// Count ready instances
	for _, instance := range instances {
		if instance.Status == types.InstanceStatusRunning {
			status.RunningInstances++
		}
	}

	return status, nil
}

// GetInstanceStatus returns the current status of an instance
func (o *orchestrator) GetInstanceStatus(ctx context.Context, namespace, serviceName, instanceID string) (*types.InstanceStatusInfo, error) {
	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceName != serviceName {
		return nil, fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	return &types.InstanceStatusInfo{
		Status:     instance.Status,
		InstanceID: instance.ID,
		NodeID:     instance.NodeID,
		CreatedAt:  instance.CreatedAt,
	}, nil
}

// GetServiceLogs returns a stream of logs for a service
func (o *orchestrator) GetServiceLogs(ctx context.Context, namespace, name string, opts types.LogOptions) (io.ReadCloser, error) {
	// First, get the service to confirm it exists
	var service types.Service
	if err := o.store.Get(ctx, types.ResourceTypeService, namespace, name, &service); err != nil {
		return nil, fmt.Errorf("failed to get service %s: %w", name, err)
	}

	// Debug logging for service details
	o.logger.Debug("Getting logs for service",
		log.Str("namespace", namespace),
		log.Str("name", name),
		log.Str("id", service.ID))

	// List instances for this service
	instances, err := o.listInstancesForService(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	// Debug logging for instances found
	o.logger.Debug("Found instances for service",
		log.Str("namespace", namespace),
		log.Str("name", name),
		log.Int("count", len(instances)))

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances found for service %s in namespace %s", name, namespace)
	}

	// Collect log streams from all instances
	logInfos := make([]InstanceLogInfo, 0, len(instances))
	for _, instance := range instances {
		// Skip instances that are marked as deleted
		if instance.Status == types.InstanceStatusDeleted {
			o.logger.Debug("Skipping deleted instance for logs",
				log.Str("namespace", namespace),
				log.Str("service", name),
				log.Str("instance_id", instance.ID),
				log.Str("instance_name", instance.Name))
			continue
		}

		// Debug logging for instance details
		o.logger.Debug("Processing instance",
			log.Str("namespace", namespace),
			log.Str("service", name),
			log.Str("instance_id", instance.ID),
			log.Str("instance_name", instance.Name),
			log.Str("status", string(instance.Status)))

		// Apply filtering based on instance status and requested log types
		// - Running instances provide container logs (if ShowLogs is true)
		// - All instances can provide events and status updates
		if instance.Status == types.InstanceStatusRunning {
			// For running instances, we need at least one type of log enabled
			if !opts.ShowLogs && !opts.ShowEvents && !opts.ShowStatus {
				o.logger.Debug("Skipping running instance - no log types enabled",
					log.Str("instance_id", instance.ID))
				continue
			}
		} else {
			// For non-running instances, we only care about events and status
			// Skip if we're not interested in those
			if !opts.ShowEvents && !opts.ShowStatus {
				o.logger.Debug("Skipping non-running instance - events/status not enabled",
					log.Str("instance_id", instance.ID),
					log.Str("status", string(instance.Status)))
				continue
			}
		}

		logReader, err := o.GetInstanceLogs(ctx, namespace, instance.ID, opts)
		if err != nil {
			o.logger.Warn("Failed to get logs for instance",
				log.Str("service", name),
				log.Str("namespace", namespace),
				log.Str("instance", instance.ID),
				log.Err(err))
			// Continue with other instances even if one fails
			continue
		}
		logInfos = append(logInfos, InstanceLogInfo{
			InstanceID:   instance.ID,
			InstanceName: instance.Name,
			Reader:       logReader,
		})
	}

	o.logger.Debug("Collected log streams",
		log.Str("service", name),
		log.Int("streams", len(logInfos)))

	if len(logInfos) == 0 {
		// Check if we have any non-deleted instances
		hasNonDeletedInstances := false
		for _, instance := range instances {
			if instance.Status != types.InstanceStatusDeleted {
				hasNonDeletedInstances = true
				break
			}
		}

		// If we have non-deleted instances but couldn't get logs, report error
		if hasNonDeletedInstances {
			return nil, fmt.Errorf("failed to get logs from any instance of service %s in namespace %s", name, namespace)
		} else {
			// If all instances are deleted, return a specific message
			return nil, fmt.Errorf("all instances of service %s in namespace %s are marked as deleted", name, namespace)
		}
	}

	// Always use MultiLogStreamer to ensure consistent metadata handling
	// regardless of whether we have one or multiple log readers
	return NewMultiLogStreamer(logInfos, true), nil
}

// GetInstanceLogs returns a stream of logs for an instance
func (o *orchestrator) GetInstanceLogs(ctx context.Context, namespace, id string, opts types.LogOptions) (io.ReadCloser, error) {
	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, id, &instance); err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// Get logs from instance controller
	logReader, err := o.instanceController.GetInstanceLogs(ctx, &instance, opts)
	if err != nil {
		return nil, err
	}

	// Wrap the reader in a MultiLogStreamer to ensure consistent metadata formatting
	logInfos := []InstanceLogInfo{
		{
			InstanceID:   instance.ID,
			InstanceName: instance.Name,
			Reader:       logReader,
		},
	}

	return NewMultiLogStreamer(logInfos, true), nil
}

// ExecInService executes a command in a running instance of the service
func (o *orchestrator) ExecInService(ctx context.Context, namespace, serviceName string, options types.ExecOptions) (types.ExecStream, error) {
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
func (o *orchestrator) ExecInInstance(ctx context.Context, namespace, serviceName, instanceID string, options types.ExecOptions) (types.ExecStream, error) {
	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceName != serviceName {
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
	err := o.store.List(ctx, types.ResourceTypeInstance, namespace, &instances)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	// Filter instances for this service
	filteredInstances := make([]*types.Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.ServiceName == serviceName {
			filteredInstances = append(filteredInstances, &instance)
		}
	}

	return filteredInstances, nil
}

// GetInstanceController returns the instance controller for testing purposes
func (o *orchestrator) GetInstanceController() controllers.InstanceController {
	return o.instanceController
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
	if instance.ServiceName != serviceName {
		return fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	// Restart the instance through instance controller
	if err := o.instanceController.RestartInstance(ctx, &instance, controllers.InstanceRestartReasonManual); err != nil {
		return fmt.Errorf("failed to restart instance: %w", err)
	}

	o.logger.Info("Successfully restarted instance",
		log.Str("namespace", namespace),
		log.Str("service", serviceName),
		log.Str("instance", instanceID))
	return nil
}

// StopService stops all instances of a service but keeps them in the store
func (o *orchestrator) StopService(ctx context.Context, namespace, serviceName string) error {
	o.logger.Info("Stopping service",
		log.Str("namespace", namespace),
		log.Str("service", serviceName))

	// List instances for this service
	instances, err := o.listInstancesForService(ctx, namespace, serviceName)
	if err != nil {
		return fmt.Errorf("failed to list instances for service: %w", err)
	}

	if len(instances) == 0 {
		// No instances to stop
		o.logger.Info("No instances found to stop for service",
			log.Str("namespace", namespace),
			log.Str("service", serviceName))
		return nil
	}

	// Stop each instance
	var lastError error
	successCount := 0
	for _, instance := range instances {
		if err := o.instanceController.StopInstance(ctx, instance); err != nil {
			o.logger.Error("Failed to stop instance",
				log.Str("namespace", namespace),
				log.Str("service", serviceName),
				log.Str("instance", instance.ID),
				log.Err(err))
			lastError = err
			// Continue with other instances even if one fails
		} else {
			successCount++
		}
	}

	// Return the last error encountered, if any
	if lastError != nil {
		return fmt.Errorf("failed to stop all instances of service %s: %d of %d stopped: %w",
			serviceName, successCount, len(instances), lastError)
	}

	o.logger.Info("Successfully stopped all instances of service",
		log.Str("namespace", namespace),
		log.Str("service", serviceName),
		log.Int("instances", len(instances)))
	return nil
}

// StopInstance stops a specific instance but keeps it in the store
func (o *orchestrator) StopInstance(ctx context.Context, namespace, serviceName, instanceID string) error {
	o.logger.Info("Stopping instance",
		log.Str("namespace", namespace),
		log.Str("service", serviceName),
		log.Str("instance", instanceID))

	// Get instance from store
	var instance types.Instance
	if err := o.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	// Verify instance belongs to the specified service
	if instance.ServiceName != serviceName {
		return fmt.Errorf("instance does not belong to service %s", serviceName)
	}

	// Stop the instance through instance controller
	if err := o.instanceController.StopInstance(ctx, &instance); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	o.logger.Info("Successfully stopped instance",
		log.Str("namespace", namespace),
		log.Str("service", serviceName),
		log.Str("instance", instanceID))
	return nil
}
