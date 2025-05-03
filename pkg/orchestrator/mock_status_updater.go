package orchestrator

import (
	"context"
	"sync"

	"github.com/rzbill/rune/pkg/types"
)

// MockStatusUpdater implements StatusUpdater for testing
type MockStatusUpdater struct {
	mu                    sync.Mutex
	serviceStatusUpdates  map[string]types.ServiceStatus
	instanceStatusUpdates map[string]types.InstanceStatus
}

// NewMockStatusUpdater creates a new mock status updater
func NewMockStatusUpdater() *MockStatusUpdater {
	return &MockStatusUpdater{
		serviceStatusUpdates:  make(map[string]types.ServiceStatus),
		instanceStatusUpdates: make(map[string]types.InstanceStatus),
	}
}

// UpdateServiceStatus implements StatusUpdater
func (m *MockStatusUpdater) UpdateServiceStatus(ctx context.Context, namespace, name string, service *types.Service, status types.ServiceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	m.serviceStatusUpdates[key] = status
	return nil
}

// UpdateInstanceStatus implements StatusUpdater
func (m *MockStatusUpdater) UpdateInstanceStatus(ctx context.Context, namespace, name string, instance *types.Instance, status types.InstanceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	m.instanceStatusUpdates[key] = status
	return nil
}

// GetServiceStatus returns the last service status update for a service
func (m *MockStatusUpdater) GetServiceStatus(namespace, name string) (types.ServiceStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	status, exists := m.serviceStatusUpdates[key]
	return status, exists
}

// GetInstanceStatus returns the last instance status update for an instance
func (m *MockStatusUpdater) GetInstanceStatus(namespace, name string) (types.InstanceStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	status, exists := m.instanceStatusUpdates[key]
	return status, exists
}

// Reset clears all recorded status updates
func (m *MockStatusUpdater) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceStatusUpdates = make(map[string]types.ServiceStatus)
	m.instanceStatusUpdates = make(map[string]types.InstanceStatus)
}
