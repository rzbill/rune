package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/types"
)

// FakeOrchestrator implements the Orchestrator interface for testing purposes
type FakeOrchestrator struct {
	mu                     sync.Mutex
	logger                 log.Logger
	started                bool
	stopped                bool
	services               map[string]map[string]*types.Service  // namespace -> name -> service
	instances              map[string]map[string]*types.Instance // namespace -> ID -> instance
	StartCalls             []context.Context
	StopCalls              []bool
	GetServiceStatusCalls  []GetServiceStatusCall
	GetInstanceStatusCalls []GetInstanceStatusCall
	GetServiceLogsCalls    []GetServiceLogsCall
	GetInstanceLogsCalls   []GetInstanceLogsCall
	ExecInServiceCalls     []ExecInServiceCall
	ExecInInstanceCalls    []ExecInInstanceCall
	RestartServiceCalls    []RestartServiceCall
	RestartInstanceCalls   []OrchestratorRestartInstanceCall
	StopServiceCalls       []StopServiceCall
	StopInstanceCalls      []OrchestratorStopInstanceCall
	DeleteServiceCalls     []DeleteServiceCall
	GetDeletionStatusCalls []GetDeletionStatusCall

	// Custom behavior options
	StartFunc             func(ctx context.Context) error
	StopFunc              func() error
	GetServiceStatusFunc  func(ctx context.Context, namespace, name string) (*types.ServiceStatusInfo, error)
	GetInstanceStatusFunc func(ctx context.Context, namespace, serviceName, instanceID string) (*types.InstanceStatusInfo, error)
	GetServiceLogsFunc    func(ctx context.Context, namespace, name string, opts types.LogOptions) (io.ReadCloser, error)
	GetInstanceLogsFunc   func(ctx context.Context, namespace, instanceName string, opts types.LogOptions) (io.ReadCloser, error)
	ExecInServiceFunc     func(ctx context.Context, namespace, serviceName string, options types.ExecOptions) (types.ExecStream, error)
	ExecInInstanceFunc    func(ctx context.Context, namespace, serviceName, instanceID string, options types.ExecOptions) (types.ExecStream, error)
	RestartServiceFunc    func(ctx context.Context, namespace, serviceName string) error
	RestartInstanceFunc   func(ctx context.Context, namespace, serviceName, instanceID string) error
	StopServiceFunc       func(ctx context.Context, namespace, serviceName string) error
	StopInstanceFunc      func(ctx context.Context, namespace, serviceName, instanceID string) error
	DeleteServiceFunc     func(ctx context.Context, request *types.DeletionRequest) (*types.DeletionResponse, error)
	GetDeletionStatusFunc func(ctx context.Context, deletionID string) (*types.DeletionOperation, error)

	// Default error responses
	StartError             error
	StopError              error
	GetServiceStatusError  error
	GetInstanceStatusError error
	GetServiceLogsError    error
	GetInstanceLogsError   error
	ExecInServiceError     error
	ExecInInstanceError    error
	RestartServiceError    error
	RestartInstanceError   error
	StopServiceError       error
	StopInstanceError      error
	DeleteServiceError     error
	GetDeletionStatusError error

	// Mock responses
	LogsOutput   []byte
	ExecStdout   []byte
	ExecStderr   []byte
	ExecExitCode int
}

// Call tracking structs
type GetServiceStatusCall struct {
	Namespace string
	Name      string
}

type GetInstanceStatusCall struct {
	Namespace   string
	ServiceName string
	InstanceID  string
}

type GetServiceLogsCall struct {
	Namespace string
	Name      string
	Options   types.LogOptions
}

type GetInstanceLogsCall struct {
	Namespace    string
	InstanceName string
	Options      types.LogOptions
}

type ExecInServiceCall struct {
	Namespace   string
	ServiceName string
	Options     types.ExecOptions
}

type ExecInInstanceCall struct {
	Namespace   string
	ServiceName string
	InstanceID  string
	Options     types.ExecOptions
}

type RestartServiceCall struct {
	Namespace   string
	ServiceName string
}

type OrchestratorRestartInstanceCall struct {
	Namespace   string
	ServiceName string
	InstanceID  string
}

// Add these new call tracking structs after OrchestratorRestartInstanceCall

type StopServiceCall struct {
	Namespace   string
	ServiceName string
}

type OrchestratorStopInstanceCall struct {
	Namespace   string
	ServiceName string
	InstanceID  string
}

type DeleteServiceCall struct {
	Request *types.DeletionRequest
}

type GetDeletionStatusCall struct {
	DeletionID string
}

// NewFakeOrchestrator creates a new fake orchestrator for testing
func NewFakeOrchestrator() *FakeOrchestrator {
	return &FakeOrchestrator{
		logger:    log.NewLogger().WithComponent("fake-orchestrator"),
		services:  make(map[string]map[string]*types.Service),
		instances: make(map[string]map[string]*types.Instance),
	}
}

// Start implementation for testing
func (o *FakeOrchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.StartCalls = append(o.StartCalls, ctx)

	if o.StartFunc != nil {
		return o.StartFunc(ctx)
	}

	if o.StartError != nil {
		return o.StartError
	}

	o.started = true
	return nil
}

// Stop implementation for testing
func (o *FakeOrchestrator) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.StopCalls = append(o.StopCalls, true)

	if o.StopFunc != nil {
		return o.StopFunc()
	}

	if o.StopError != nil {
		return o.StopError
	}

	o.stopped = true
	return nil
}

// GetServiceStatus implementation for testing
func (o *FakeOrchestrator) GetServiceStatus(ctx context.Context, namespace, name string) (*types.ServiceStatusInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.GetServiceStatusCalls = append(o.GetServiceStatusCalls, GetServiceStatusCall{
		Namespace: namespace,
		Name:      name,
	})

	if o.GetServiceStatusFunc != nil {
		return o.GetServiceStatusFunc(ctx, namespace, name)
	}

	if o.GetServiceStatusError != nil {
		return nil, o.GetServiceStatusError
	}

	// Default behavior - look up in our fake storage
	nsServices, ok := o.services[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	service, ok := nsServices[name]
	if !ok {
		return nil, fmt.Errorf("service %s not found in namespace %s", name, namespace)
	}

	// Count instances for this service
	instanceCount := 0
	readyCount := 0
	for _, nsInstances := range o.instances {
		for _, instance := range nsInstances {
			if instance.Namespace == namespace && instance.ServiceID == name {
				instanceCount++
				if instance.Status == types.InstanceStatusRunning {
					readyCount++
				}
			}
		}
	}

	return &types.ServiceStatusInfo{
		Status:           service.Status,
		DesiredInstances: instanceCount,
		RunningInstances: readyCount,
	}, nil
}

// GetInstanceStatus implementation for testing
func (o *FakeOrchestrator) GetInstanceStatus(ctx context.Context, namespace, serviceName, instanceID string) (*types.InstanceStatusInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.GetInstanceStatusCalls = append(o.GetInstanceStatusCalls, GetInstanceStatusCall{
		Namespace:   namespace,
		ServiceName: serviceName,
		InstanceID:  instanceID,
	})

	if o.GetInstanceStatusFunc != nil {
		return o.GetInstanceStatusFunc(ctx, namespace, serviceName, instanceID)
	}

	if o.GetInstanceStatusError != nil {
		return nil, o.GetInstanceStatusError
	}

	// Default behavior - look up in our fake storage
	nsInstances, ok := o.instances[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	instance, ok := nsInstances[instanceID]
	if !ok {
		return nil, fmt.Errorf("instance %s not found in namespace %s", instanceID, namespace)
	}

	// Verify service association
	if instance.ServiceName != serviceName {
		return nil, fmt.Errorf("instance %s does not belong to service %s", instanceID, serviceName)
	}

	return &types.InstanceStatusInfo{
		Status:     instance.Status,
		InstanceID: instance.ID,
		NodeID:     instance.NodeID,
		CreatedAt:  instance.CreatedAt,
	}, nil
}

// GetServiceLogs implementation for testing
func (o *FakeOrchestrator) GetServiceLogs(ctx context.Context, namespace, name string, opts types.LogOptions) (io.ReadCloser, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.GetServiceLogsCalls = append(o.GetServiceLogsCalls, GetServiceLogsCall{
		Namespace: namespace,
		Name:      name,
		Options:   opts,
	})

	if o.GetServiceLogsFunc != nil {
		return o.GetServiceLogsFunc(ctx, namespace, name, opts)
	}

	if o.GetServiceLogsError != nil {
		return nil, o.GetServiceLogsError
	}

	// Default behavior - verify service exists first
	nsServices, ok := o.services[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	_, ok = nsServices[name]
	if !ok {
		return nil, fmt.Errorf("service %s not found in namespace %s", name, namespace)
	}

	// Return predefined logs content or empty if not set
	content := o.LogsOutput
	if content == nil {
		content = []byte("")
	}

	return io.NopCloser(bytes.NewReader(content)), nil
}

// GetInstanceLogs implementation for testing
func (o *FakeOrchestrator) GetInstanceLogs(ctx context.Context, namespace, instanceName string, opts types.LogOptions) (io.ReadCloser, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.GetInstanceLogsCalls = append(o.GetInstanceLogsCalls, GetInstanceLogsCall{
		Namespace:    namespace,
		InstanceName: instanceName,
		Options:      opts,
	})

	if o.GetInstanceLogsFunc != nil {
		return o.GetInstanceLogsFunc(ctx, namespace, instanceName, opts)
	}

	if o.GetInstanceLogsError != nil {
		return nil, o.GetInstanceLogsError
	}

	// Default behavior - verify instance exists
	nsInstances, ok := o.instances[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	_, ok = nsInstances[instanceName]
	if !ok {
		return nil, fmt.Errorf("instance %s not found in namespace %s", instanceName, namespace)
	}

	// Return predefined logs content or empty if not set
	content := o.LogsOutput
	if content == nil {
		content = []byte("")
	}

	return io.NopCloser(bytes.NewReader(content)), nil
}

// ExecInService implementation for testing
func (o *FakeOrchestrator) ExecInService(ctx context.Context, namespace, serviceName string, options types.ExecOptions) (types.ExecStream, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.ExecInServiceCalls = append(o.ExecInServiceCalls, ExecInServiceCall{
		Namespace:   namespace,
		ServiceName: serviceName,
		Options:     options,
	})

	if o.ExecInServiceFunc != nil {
		return o.ExecInServiceFunc(ctx, namespace, serviceName, options)
	}

	if o.ExecInServiceError != nil {
		return nil, o.ExecInServiceError
	}

	// Default behavior - verify service exists
	nsServices, ok := o.services[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	_, ok = nsServices[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found in namespace %s", serviceName, namespace)
	}

	// Verify service has at least one running instance
	instanceFound := false
	for _, nsInstances := range o.instances {
		for _, instance := range nsInstances {
			if instance.Namespace == namespace && instance.ServiceID == serviceName && instance.Status == types.InstanceStatusRunning {
				instanceFound = true
				break
			}
		}
		if instanceFound {
			break
		}
	}

	if !instanceFound {
		return nil, fmt.Errorf("no running instances found for service %s in namespace %s", serviceName, namespace)
	}

	// Return a fake exec stream
	return runner.NewFakeExecStream(o.ExecStdout, o.ExecStderr, o.ExecExitCode), nil
}

// ExecInInstance implementation for testing
func (o *FakeOrchestrator) ExecInInstance(ctx context.Context, namespace, serviceName, instanceID string, options types.ExecOptions) (types.ExecStream, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.ExecInInstanceCalls = append(o.ExecInInstanceCalls, ExecInInstanceCall{
		Namespace:   namespace,
		ServiceName: serviceName,
		InstanceID:  instanceID,
		Options:     options,
	})

	if o.ExecInInstanceFunc != nil {
		return o.ExecInInstanceFunc(ctx, namespace, serviceName, instanceID, options)
	}

	if o.ExecInInstanceError != nil {
		return nil, o.ExecInInstanceError
	}

	// Default behavior - verify instance exists
	nsInstances, ok := o.instances[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	instance, ok := nsInstances[instanceID]
	if !ok {
		return nil, fmt.Errorf("instance %s not found in namespace %s", instanceID, namespace)
	}

	// Verify service association
	if instance.ServiceName != serviceName {
		return nil, fmt.Errorf("instance %s does not belong to service %s", instanceID, serviceName)
	}

	// Verify instance is running
	if instance.Status != types.InstanceStatusRunning {
		return nil, fmt.Errorf("instance %s is not running, current status: %s", instanceID, instance.Status)
	}

	// Return a fake exec stream
	return runner.NewFakeExecStream(o.ExecStdout, o.ExecStderr, o.ExecExitCode), nil
}

// AddService adds a service to the fake storage
func (o *FakeOrchestrator) AddService(service *types.Service) {
	o.mu.Lock()
	defer o.mu.Unlock()

	namespace := service.Namespace
	if _, ok := o.services[namespace]; !ok {
		o.services[namespace] = make(map[string]*types.Service)
	}

	o.services[namespace][service.Name] = service
}

// AddInstance adds an instance to the fake storage
func (o *FakeOrchestrator) AddInstance(instance *types.Instance) {
	o.mu.Lock()
	defer o.mu.Unlock()

	namespace := instance.Namespace
	if _, ok := o.instances[namespace]; !ok {
		o.instances[namespace] = make(map[string]*types.Instance)
	}

	o.instances[namespace][instance.ID] = instance
}

// GetService gets a service from the fake storage
func (o *FakeOrchestrator) GetService(namespace, name string) (*types.Service, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	nsServices, ok := o.services[namespace]
	if !ok {
		return nil, false
	}

	service, ok := nsServices[name]
	return service, ok
}

// GetInstance gets an instance from the fake storage
func (o *FakeOrchestrator) GetInstance(namespace, id string) (*types.Instance, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	nsInstances, ok := o.instances[namespace]
	if !ok {
		return nil, false
	}

	instance, ok := nsInstances[id]
	return instance, ok
}

// ListServiceInstances gets all instances for a service
func (o *FakeOrchestrator) ListServiceInstances(namespace, serviceName string) []*types.Instance {
	o.mu.Lock()
	defer o.mu.Unlock()

	var result []*types.Instance

	for _, nsInstances := range o.instances {
		for _, instance := range nsInstances {
			if instance.Namespace == namespace && instance.ServiceID == serviceName {
				result = append(result, instance)
			}
		}
	}

	return result
}

// DeleteService implementation for testing
func (o *FakeOrchestrator) DeleteService(ctx context.Context, request *types.DeletionRequest) (*types.DeletionResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.DeleteServiceCalls = append(o.DeleteServiceCalls, DeleteServiceCall{
		Request: request,
	})

	if o.DeleteServiceFunc != nil {
		return o.DeleteServiceFunc(ctx, request)
	}

	if o.DeleteServiceError != nil {
		return nil, o.DeleteServiceError
	}

	// Default behavior - verify service exists first
	nsServices, ok := o.services[request.Namespace]
	if !ok {
		if request.IgnoreNotFound {
			return &types.DeletionResponse{
				Status:   "not_found",
				Warnings: []string{fmt.Sprintf("Service %s/%s not found", request.Namespace, request.Name)},
			}, nil
		}
		return nil, fmt.Errorf("namespace %s not found", request.Namespace)
	}

	_, ok = nsServices[request.Name]
	if !ok {
		if request.IgnoreNotFound {
			return &types.DeletionResponse{
				Status:   "not_found",
				Warnings: []string{fmt.Sprintf("Service %s/%s not found", request.Namespace, request.Name)},
			}, nil
		}
		return nil, fmt.Errorf("service %s not found in namespace %s", request.Name, request.Namespace)
	}

	// Handle dry run
	if request.DryRun {
		// Create finalizers for dry run - these show what operations would be performed
		// They are not actually pending execution, but represent the cleanup plan
		finalizers := []types.Finalizer{
			{
				ID:        "dry-run-finalizer-1",
				Type:      types.FinalizerTypeInstanceCleanup,
				Status:    types.FinalizerStatusPending, // Shows this operation would be performed
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        "dry-run-finalizer-2",
				Type:      types.FinalizerTypeServiceDeregister,
				Status:    types.FinalizerStatusPending, // Shows this operation would be performed
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		return &types.DeletionResponse{
			DeletionID: "dry-run",
			Status:     "dry_run",
			Finalizers: finalizers,
		}, nil
	}

	// Remove the service from fake storage
	delete(nsServices, request.Name)

	// Remove all instances for this service
	for _, nsInstances := range o.instances {
		for id, instance := range nsInstances {
			if instance.Namespace == request.Namespace && instance.ServiceName == request.Name {
				delete(nsInstances, id)
			}
		}
	}

	return &types.DeletionResponse{
		Status: "completed",
	}, nil
}

// GetDeletionStatus implementation for testing
func (o *FakeOrchestrator) GetDeletionStatus(ctx context.Context, deletionID string) (*types.DeletionOperation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.GetDeletionStatusCalls = append(o.GetDeletionStatusCalls, GetDeletionStatusCall{
		DeletionID: deletionID,
	})

	if o.GetDeletionStatusFunc != nil {
		return o.GetDeletionStatusFunc(ctx, deletionID)
	}

	if o.GetDeletionStatusError != nil {
		return nil, o.GetDeletionStatusError
	}

	// Default behavior - return a completed operation
	return &types.DeletionOperation{
		ID:     deletionID,
		Status: types.DeletionOperationStatusCompleted,
	}, nil
}

// RemoveService removes a service from the fake storage (legacy method for backward compatibility)
func (o *FakeOrchestrator) RemoveService(namespace, name string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	nsServices, ok := o.services[namespace]
	if !ok {
		return
	}

	delete(nsServices, name)
}

// DeleteInstance removes an instance from the fake storage
func (o *FakeOrchestrator) DeleteInstance(namespace, id string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	nsInstances, ok := o.instances[namespace]
	if !ok {
		return
	}

	delete(nsInstances, id)
}

// StopService implementation for testing
func (o *FakeOrchestrator) StopService(ctx context.Context, namespace, serviceName string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.StopServiceCalls = append(o.StopServiceCalls, StopServiceCall{
		Namespace:   namespace,
		ServiceName: serviceName,
	})

	if o.StopServiceFunc != nil {
		return o.StopServiceFunc(ctx, namespace, serviceName)
	}

	if o.StopServiceError != nil {
		return o.StopServiceError
	}

	// Default behavior - verify service exists first
	nsServices, ok := o.services[namespace]
	if !ok {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	_, ok = nsServices[serviceName]
	if !ok {
		return fmt.Errorf("service %s not found in namespace %s", serviceName, namespace)
	}

	// Find all instances for this service and set them as "stopped"
	instanceCount := 0
	for _, nsInstances := range o.instances {
		for _, instance := range nsInstances {
			if instance.Namespace == namespace && instance.ServiceID == serviceName {
				instance.Status = types.InstanceStatusStopped
				instance.StatusMessage = "Stopped by fake orchestrator"
				instance.UpdatedAt = time.Now()
				instanceCount++
			}
		}
	}

	if instanceCount == 0 {
		o.logger.Info("No instances found to stop for service",
			log.Str("namespace", namespace),
			log.Str("service", serviceName))
		return nil
	}

	return nil
}

// StopInstance implementation for testing
func (o *FakeOrchestrator) StopInstance(ctx context.Context, namespace, serviceName, instanceID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.StopInstanceCalls = append(o.StopInstanceCalls, OrchestratorStopInstanceCall{
		Namespace:   namespace,
		ServiceName: serviceName,
		InstanceID:  instanceID,
	})

	if o.StopInstanceFunc != nil {
		return o.StopInstanceFunc(ctx, namespace, serviceName, instanceID)
	}

	if o.StopInstanceError != nil {
		return o.StopInstanceError
	}

	// Default behavior - verify instance exists
	nsInstances, ok := o.instances[namespace]
	if !ok {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	instance, ok := nsInstances[instanceID]
	if !ok {
		return fmt.Errorf("instance %s not found in namespace %s", instanceID, namespace)
	}

	// Verify service association
	if instance.ServiceName != serviceName {
		return fmt.Errorf("instance %s does not belong to service %s", instanceID, serviceName)
	}

	// "Stop" the instance in our fake store
	instance.Status = types.InstanceStatusStopped
	instance.StatusMessage = "Stopped by fake orchestrator"
	instance.UpdatedAt = time.Now()

	return nil
}

// Reset clears all recorded calls and stored instances
func (o *FakeOrchestrator) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.started = false
	o.stopped = false
	o.services = make(map[string]map[string]*types.Service)
	o.instances = make(map[string]map[string]*types.Instance)
	o.StartCalls = nil
	o.StopCalls = nil
	o.GetServiceStatusCalls = nil
	o.GetInstanceStatusCalls = nil
	o.GetServiceLogsCalls = nil
	o.GetInstanceLogsCalls = nil
	o.ExecInServiceCalls = nil
	o.ExecInInstanceCalls = nil
	o.RestartServiceCalls = nil
	o.RestartInstanceCalls = nil
	o.StopServiceCalls = nil
	o.StopInstanceCalls = nil
	o.DeleteServiceCalls = nil
	o.GetDeletionStatusCalls = nil
}

// RestartService implementation for testing
func (o *FakeOrchestrator) RestartService(ctx context.Context, namespace, serviceName string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.RestartServiceCalls = append(o.RestartServiceCalls, RestartServiceCall{
		Namespace:   namespace,
		ServiceName: serviceName,
	})

	if o.RestartServiceFunc != nil {
		return o.RestartServiceFunc(ctx, namespace, serviceName)
	}

	if o.RestartServiceError != nil {
		return o.RestartServiceError
	}

	// Default behavior - verify service exists first
	nsServices, ok := o.services[namespace]
	if !ok {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	_, ok = nsServices[serviceName]
	if !ok {
		return fmt.Errorf("service %s not found in namespace %s", serviceName, namespace)
	}

	// Find all instances for this service and set them as "restarted"
	instanceCount := 0
	for _, nsInstances := range o.instances {
		for _, instance := range nsInstances {
			if instance.Namespace == namespace && instance.ServiceID == serviceName {
				instance.Status = types.InstanceStatusRunning
				instance.StatusMessage = "Restarted by fake orchestrator"
				instanceCount++
			}
		}
	}

	if instanceCount == 0 {
		return fmt.Errorf("no instances found for service %s in namespace %s", serviceName, namespace)
	}

	return nil
}

// RestartInstance implementation for testing
func (o *FakeOrchestrator) RestartInstance(ctx context.Context, namespace, serviceName, instanceID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.RestartInstanceCalls = append(o.RestartInstanceCalls, OrchestratorRestartInstanceCall{
		Namespace:   namespace,
		ServiceName: serviceName,
		InstanceID:  instanceID,
	})

	if o.RestartInstanceFunc != nil {
		return o.RestartInstanceFunc(ctx, namespace, serviceName, instanceID)
	}

	if o.RestartInstanceError != nil {
		return o.RestartInstanceError
	}

	// Default behavior - verify instance exists
	nsInstances, ok := o.instances[namespace]
	if !ok {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	instance, ok := nsInstances[instanceID]
	if !ok {
		return fmt.Errorf("instance %s not found in namespace %s", instanceID, namespace)
	}

	// Verify service association
	if instance.ServiceName != serviceName {
		return fmt.Errorf("instance %s does not belong to service %s", instanceID, serviceName)
	}

	// "Restart" the instance in our fake store
	instance.Status = types.InstanceStatusRunning
	instance.StatusMessage = "Restarted by fake orchestrator"
	instance.UpdatedAt = time.Now()

	return nil
}
