package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rzbill/rune/pkg/types"
)

// Validate that TestStore implements the Store interface
var _ Store = &TestStore{}

// TestStore provides a simple in-memory implementation for testing purposes.
// Unlike MockStore, it doesn't require setting up expectations and is more convenient
// for basic tests that need a functional store.
type TestStore struct {
	data       map[string]map[string]map[string]interface{}
	history    map[string]map[string]map[string][]HistoricalVersion
	mutex      sync.RWMutex
	watchChans map[string][]chan WatchEvent
	watchMutex sync.RWMutex
	opened     bool
}

// NewTestStore creates a new TestStore instance.
func NewTestStore() *TestStore {
	return &TestStore{
		data:       make(map[string]map[string]map[string]interface{}),
		history:    make(map[string]map[string]map[string][]HistoricalVersion),
		watchChans: make(map[string][]chan WatchEvent),
		opened:     true, // Consider it already opened for simplicity
	}
}

// Open implements the Store interface.
func (s *TestStore) Open(path string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.opened {
		return nil
	}

	s.opened = true
	return nil
}

// Close implements the Store interface.
func (s *TestStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.opened {
		return nil
	}

	// Close all watch channels
	s.watchMutex.Lock()
	for key, chans := range s.watchChans {
		for _, ch := range chans {
			close(ch)
		}
		delete(s.watchChans, key)
	}
	s.watchMutex.Unlock()

	s.opened = false
	return nil
}

func (s *TestStore) CreateResource(ctx context.Context, resourceType string, resource interface{}) error {
	// Special case for Namespace resources
	if resourceType == types.ResourceTypeNamespace {
		namespace, ok := resource.(*types.Namespace)
		if !ok {
			return fmt.Errorf("expected Namespace type for namespace resource")
		}

		// Use the namespace name as both namespace and name
		// This effectively stores namespaces in a pseudo-namespace called "system"
		return s.Create(ctx, resourceType, "system", namespace.Name, namespace)
	}

	// Normal handling for other resource types
	namespacedResource, ok := resource.(types.NamespacedResource)
	if !ok {
		return fmt.Errorf("resource must implement NamespacedResource interface")
	}

	nn := namespacedResource.NamespacedName()
	return s.Create(ctx, resourceType, nn.Namespace, nn.Name, resource)
}

// Create implements the Store interface.
func (s *TestStore) Create(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	fmt.Println("Create", resourceType, namespace, name, resource)

	if !s.opened {
		return errors.New("store is not opened")
	}

	fmt.Println("Before Create", resourceType, s.data[resourceType])
	// Initialize maps if they don't exist
	if _, exists := s.data[resourceType]; !exists {
		s.data[resourceType] = make(map[string]map[string]interface{})
		s.history[resourceType] = make(map[string]map[string][]HistoricalVersion)
	}

	if _, exists := s.data[resourceType][namespace]; !exists {
		s.data[resourceType][namespace] = make(map[string]interface{})
		s.history[resourceType][namespace] = make(map[string][]HistoricalVersion)
	}

	// Check if resource already exists
	if _, exists := s.data[resourceType][namespace][name]; exists {
		return fmt.Errorf("resource %s/%s/%s already exists", resourceType, namespace, name)
	}

	// Make a copy of the resource to avoid reference issues
	s.data[resourceType][namespace][name] = resource

	// Record history
	version := fmt.Sprintf("%d", time.Now().UnixNano())
	s.history[resourceType][namespace][name] = []HistoricalVersion{
		{
			Version:   version,
			Timestamp: time.Now(),
			Resource:  resource,
		},
	}

	fmt.Println("After Create", s.data[resourceType])

	// Send watch event
	s.sendWatchEvent(resourceType, namespace, WatchEventCreated, name, resource)

	return nil
}

// Get implements the Store interface.
func (s *TestStore) Get(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if !s.opened {
		return errors.New("store is not opened")
	}

	if _, exists := s.data[resourceType]; !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	if _, exists := s.data[resourceType][namespace]; !exists {
		return fmt.Errorf("namespace %s not found in resource type %s", namespace, resourceType)
	}

	if data, exists := s.data[resourceType][namespace][name]; exists {
		// For testing purposes, we're implementing a simplified struct copying approach
		// This works with common test patterns where you pass a pointer to a struct

		// If it implements our Set interface, use that
		if setter, ok := resource.(interface{ Set(interface{}) }); ok {
			setter.Set(data)
			return nil
		}

		// For test convenience, let's try to handle common test types directly
		switch storedData := data.(type) {
		case map[string]interface{}:
			// If the target is a map
			if targetMap, ok := resource.(*map[string]interface{}); ok {
				*targetMap = storedData
				return nil
			}

		case *types.Service:
			// If the target is a Service pointer
			if targetSvc, ok := resource.(*types.Service); ok && storedData != nil {
				*targetSvc = *storedData
				return nil
			}

		case *types.Instance:
			// If the target is an Instance pointer
			if targetInst, ok := resource.(*types.Instance); ok && storedData != nil {
				*targetInst = *storedData
				return nil
			}

		case types.Service:
			// If stored as value but target is pointer
			if targetSvc, ok := resource.(*types.Service); ok {
				*targetSvc = storedData
				return nil
			}

		case types.Instance:
			// If stored as value but target is pointer
			if targetInst, ok := resource.(*types.Instance); ok {
				*targetInst = storedData
				return nil
			}
		}

		// Store the data - for testing we assume this will work
		// This is intentionally simplified for testing purposes
		return fmt.Errorf("cannot set resource data: incompatible types (try using a pointer to the correct type)")
	}

	return fmt.Errorf("resource %s/%s/%s not found", resourceType, namespace, name)
}

// List implements the Store interface.
func (s *TestStore) List(ctx context.Context, resourceType string, namespace string, resource interface{}) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	fmt.Println("Before List", resourceType, s.data[resourceType])

	if !s.opened {
		return errors.New("store is not opened")
	}

	if _, exists := s.data[resourceType]; !exists {
		result := make([]interface{}, 0)
		return UnmarshalResource(result, resource)
	}

	if _, exists := s.data[resourceType][namespace]; !exists {
		return fmt.Errorf("namespace %s not found in resource type %s", namespace, resourceType)
	}

	result := make([]interface{}, 0, len(s.data[resourceType][namespace]))
	for _, resource := range s.data[resourceType][namespace] {
		result = append(result, resource)
	}

	return UnmarshalResource(result, resource)
}

// ListAll retrieves all resources of a given type in all namespaces.
func (s *TestStore) ListAll(ctx context.Context, resourceType string, resource interface{}) error {
	return s.List(ctx, resourceType, "", resource)
}

// Update implements the Store interface.
func (s *TestStore) Update(ctx context.Context, resourceType string, namespace string, name string, resource interface{}, opts ...UpdateOption) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Parse update options
	options := ParseUpdateOptions(opts...)

	if !s.opened {
		return errors.New("store is not opened")
	}

	if _, exists := s.data[resourceType]; !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	if _, exists := s.data[resourceType][namespace]; !exists {
		return fmt.Errorf("namespace %s not found in resource type %s", namespace, resourceType)
	}

	if _, exists := s.data[resourceType][namespace][name]; !exists {
		return fmt.Errorf("resource %s/%s/%s not found", resourceType, namespace, name)
	}

	// Update resource
	s.data[resourceType][namespace][name] = resource

	// Record history
	version := fmt.Sprintf("%d", time.Now().UnixNano())
	s.history[resourceType][namespace][name] = append(s.history[resourceType][namespace][name], HistoricalVersion{
		Version:   version,
		Timestamp: time.Now(),
		Resource:  resource,
	})

	// Send watch event with source info
	s.sendWatchEventWithSource(resourceType, namespace, WatchEventUpdated, name, resource, options.Source)

	return nil
}

// Delete implements the Store interface.
func (s *TestStore) Delete(ctx context.Context, resourceType string, namespace string, name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.opened {
		return errors.New("store is not opened")
	}

	if _, exists := s.data[resourceType]; !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	if _, exists := s.data[resourceType][namespace]; !exists {
		return fmt.Errorf("namespace %s not found in resource type %s", namespace, resourceType)
	}

	if _, exists := s.data[resourceType][namespace][name]; !exists {
		return fmt.Errorf("resource %s/%s/%s not found", resourceType, namespace, name)
	}

	// Get resource before deleting for watch event
	resource := s.data[resourceType][namespace][name]

	// Delete resource
	delete(s.data[resourceType][namespace], name)

	// Send watch event
	s.sendWatchEvent(resourceType, namespace, WatchEventDeleted, name, resource)

	return nil
}

// Watch implements the Store interface.
func (s *TestStore) Watch(ctx context.Context, resourceType string, namespace string) (<-chan WatchEvent, error) {
	s.watchMutex.Lock()
	defer s.watchMutex.Unlock()

	if !s.opened {
		return nil, errors.New("store is not opened")
	}

	// Create a buffered channel to avoid blocking
	ch := make(chan WatchEvent, 100)

	// Generate a watch key
	watchKey := fmt.Sprintf("%s/%s", resourceType, namespace)

	// Add the channel to the watch channels
	if _, exists := s.watchChans[watchKey]; !exists {
		s.watchChans[watchKey] = make([]chan WatchEvent, 0)
	}
	s.watchChans[watchKey] = append(s.watchChans[watchKey], ch)

	// Set up cancellation handling
	go func() {
		<-ctx.Done()
		s.watchMutex.Lock()
		defer s.watchMutex.Unlock()

		// Find and remove the channel
		for i, c := range s.watchChans[watchKey] {
			if c == ch {
				s.watchChans[watchKey] = append(s.watchChans[watchKey][:i], s.watchChans[watchKey][i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}

// GetHistory implements the Store interface.
func (s *TestStore) GetHistory(ctx context.Context, resourceType string, namespace string, name string) ([]HistoricalVersion, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if !s.opened {
		return nil, errors.New("store is not opened")
	}

	if _, exists := s.history[resourceType]; !exists {
		return []HistoricalVersion{}, nil
	}

	if _, exists := s.history[resourceType][namespace]; !exists {
		return []HistoricalVersion{}, nil
	}

	if versions, exists := s.history[resourceType][namespace][name]; exists {
		// Return a copy to avoid mutation
		result := make([]HistoricalVersion, len(versions))
		copy(result, versions)
		return result, nil
	}

	return []HistoricalVersion{}, nil
}

// GetVersion implements the Store interface.
func (s *TestStore) GetVersion(ctx context.Context, resourceType string, namespace string, name string, version string) (interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if !s.opened {
		return nil, errors.New("store is not opened")
	}

	if _, exists := s.history[resourceType]; !exists {
		return nil, fmt.Errorf("resource type %s not found", resourceType)
	}

	if _, exists := s.history[resourceType][namespace]; !exists {
		return nil, fmt.Errorf("namespace %s not found in resource type %s", namespace, resourceType)
	}

	if versions, exists := s.history[resourceType][namespace][name]; exists {
		for _, v := range versions {
			if v.Version == version {
				return v.Resource, nil
			}
		}
		return nil, fmt.Errorf("version %s not found for resource %s/%s/%s", version, resourceType, namespace, name)
	}

	return nil, fmt.Errorf("resource %s/%s/%s not found", resourceType, namespace, name)
}

// Transaction implements the Store interface.
func (s *TestStore) Transaction(ctx context.Context, fn func(tx Transaction) error) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.opened {
		return errors.New("store is not opened")
	}

	// Create a transaction
	tx := &testTransaction{
		store: s,
		ctx:   ctx,
	}

	// Execute the transaction function
	return fn(tx)
}

// Helper method to send watch events with source info
func (s *TestStore) sendWatchEventWithSource(resourceType, namespace string, eventType WatchEventType, name string, resource interface{}, source EventSource) {
	s.watchMutex.RLock()
	defer s.watchMutex.RUnlock()

	watchKey := fmt.Sprintf("%s/%s", resourceType, namespace)

	event := WatchEvent{
		Type:         eventType,
		ResourceType: resourceType,
		Namespace:    namespace,
		Name:         name,
		Resource:     resource,
		Source:       source,
	}

	// Send the event to all watching channels
	if chans, exists := s.watchChans[watchKey]; exists {
		for _, ch := range chans {
			// Non-blocking send
			select {
			case ch <- event:
				// Sent successfully
			default:
				// Channel is full, log this in a real implementation
			}
		}
	}
}

// Helper method to send watch events
func (s *TestStore) sendWatchEvent(resourceType, namespace string, eventType WatchEventType, name string, resource interface{}) {
	s.sendWatchEventWithSource(resourceType, namespace, eventType, name, resource, "")
}

// testTransaction implements the Transaction interface
type testTransaction struct {
	store *TestStore
	ctx   context.Context
}

// Create implements the Transaction interface
func (tx *testTransaction) Create(resourceType string, namespace string, name string, resource interface{}) error {
	return tx.store.Create(tx.ctx, resourceType, namespace, name, resource)
}

// Get implements the Transaction interface
func (tx *testTransaction) Get(resourceType string, namespace string, name string, resource interface{}) error {
	return tx.store.Get(tx.ctx, resourceType, namespace, name, resource)
}

// Update implements the Transaction interface
func (tx *testTransaction) Update(resourceType string, namespace string, name string, resource interface{}) error {
	return tx.store.Update(tx.ctx, resourceType, namespace, name, resource)
}

// Delete implements the Transaction interface
func (tx *testTransaction) Delete(resourceType string, namespace string, name string) error {
	return tx.store.Delete(tx.ctx, resourceType, namespace, name)
}

// Helper functions for testing

// SetupTestData adds predefined test data to the store
func (s *TestStore) SetupTestData(resources map[string]map[string]map[string]interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for resourceType, namespaces := range resources {
		if _, exists := s.data[resourceType]; !exists {
			s.data[resourceType] = make(map[string]map[string]interface{})
			s.history[resourceType] = make(map[string]map[string][]HistoricalVersion)
		}

		for namespace, items := range namespaces {
			if _, exists := s.data[resourceType][namespace]; !exists {
				s.data[resourceType][namespace] = make(map[string]interface{})
				s.history[resourceType][namespace] = make(map[string][]HistoricalVersion)
			}

			for name, resource := range items {
				s.data[resourceType][namespace][name] = resource
				s.history[resourceType][namespace][name] = []HistoricalVersion{
					{
						Version:   "1",
						Timestamp: time.Now(),
						Resource:  resource,
					},
				}
			}
		}
	}

	return nil
}

// Reset clears all data in the store
func (s *TestStore) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data = make(map[string]map[string]map[string]interface{})
	s.history = make(map[string]map[string]map[string][]HistoricalVersion)

	// Close all watch channels
	s.watchMutex.Lock()
	for key, chans := range s.watchChans {
		for _, ch := range chans {
			close(ch)
		}
		delete(s.watchChans, key)
	}
	s.watchMutex.Unlock()
}

// Additional helper methods for more convenient testing

// CreateService adds a service to the test store
func (s *TestStore) CreateService(ctx context.Context, service *types.Service) error {
	if service.Namespace == "" {
		service.Namespace = "default"
	}
	return s.Create(ctx, "services", service.Namespace, service.Name, service)
}

// GetService retrieves a service from the test store
func (s *TestStore) GetService(ctx context.Context, namespace, name string) (*types.Service, error) {
	if namespace == "" {
		namespace = "default"
	}

	service := &types.Service{}
	err := s.Get(ctx, "services", namespace, name, service)
	if err != nil {
		return nil, err
	}
	return service, nil
}

// CreateInstance adds an instance to the test store
func (s *TestStore) CreateInstance(ctx context.Context, instance *types.Instance) error {
	if instance.Namespace == "" {
		instance.Namespace = "default"
	}
	return s.Create(ctx, types.ResourceTypeInstance, instance.Namespace, instance.ID, instance)
}

// GetInstance retrieves an instance from the test store
func (s *TestStore) GetInstance(ctx context.Context, namespace, id string) (*types.Instance, error) {
	if namespace == "" {
		namespace = "default"
	}

	instance := &types.Instance{}
	err := s.Get(ctx, types.ResourceTypeInstance, namespace, id, instance)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

// ListServices returns all services in a namespace
func (s *TestStore) ListServices(ctx context.Context, namespace string) ([]*types.Service, error) {
	if namespace == "" {
		namespace = "default"
	}

	var services []*types.Service
	err := s.List(ctx, "services", namespace, &services)
	if err != nil {
		return nil, err
	}

	return services, nil
}

// ListInstances returns all instances in a namespace
func (s *TestStore) ListInstances(ctx context.Context, namespace string) ([]types.Instance, error) {
	if namespace == "" {
		namespace = "default"
	}

	var instances []types.Instance
	err := s.List(ctx, types.ResourceTypeInstance, namespace, &instances)
	if err != nil {
		return nil, err
	}

	return instances, nil
}
