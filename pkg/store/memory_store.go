package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rzbill/rune/pkg/types"
)

// MemoryStore is a simple in-memory implementation of the Store interface for testing.
type MemoryStore struct {
	data  map[string]map[string]map[string]interface{}
	mutex sync.RWMutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]map[string]map[string]interface{}),
	}
}

// Open initializes the memory store.
func (m *MemoryStore) Open(dbPath string) error {
	// No-op for memory store
	return nil
}

// Close closes the memory store.
func (m *MemoryStore) Close() error {
	// No-op for memory store
	return nil
}

// Get retrieves an object from the memory store.
func (m *MemoryStore) Get(ctx context.Context, resourceType, namespace, name string, value interface{}) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	nsMap, ok := m.data[resourceType]
	if !ok {
		return fmt.Errorf("resource type not found: %s", resourceType)
	}

	objMap, ok := nsMap[namespace]
	if !ok {
		return fmt.Errorf("namespace not found: %s", namespace)
	}

	obj, ok := objMap[name]
	if !ok {
		return fmt.Errorf("object not found: %s", name)
	}

	// Convert the stored object to the target type using JSON marshaling/unmarshaling
	// This is a bit inefficient but ensures type conversion works correctly
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal stored object: %w", err)
	}

	err = json.Unmarshal(jsonData, value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal into target type: %w", err)
	}

	return nil
}

// List lists objects from the memory store.
func (m *MemoryStore) List(ctx context.Context, resourceType, namespace string) ([]interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	nsMap, ok := m.data[resourceType]
	if !ok {
		return []interface{}{}, nil
	}

	objMap, ok := nsMap[namespace]
	if !ok {
		return []interface{}{}, nil
	}

	result := make([]interface{}, 0, len(objMap))
	for _, obj := range objMap {
		result = append(result, obj)
	}

	return result, nil
}

// Create creates an object in the memory store.
func (m *MemoryStore) Create(ctx context.Context, resourceType, namespace, name string, value interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	nsMap, ok := m.data[resourceType]
	if !ok {
		nsMap = make(map[string]map[string]interface{})
		m.data[resourceType] = nsMap
	}

	objMap, ok := nsMap[namespace]
	if !ok {
		objMap = make(map[string]interface{})
		nsMap[namespace] = objMap
	}

	_, exists := objMap[name]
	if exists {
		return fmt.Errorf("object already exists: %s", name)
	}

	// For specific types, make sure we store a copy to prevent modification
	switch v := value.(type) {
	case *types.Service:
		// Create a copy to store
		obj := *v
		objMap[name] = obj
	default:
		objMap[name] = value
	}

	return nil
}

// Update updates an object in the memory store.
func (m *MemoryStore) Update(ctx context.Context, resourceType, namespace, name string, value interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	nsMap, ok := m.data[resourceType]
	if !ok {
		return fmt.Errorf("resource type not found: %s", resourceType)
	}

	objMap, ok := nsMap[namespace]
	if !ok {
		return fmt.Errorf("namespace not found: %s", namespace)
	}

	_, exists := objMap[name]
	if !exists {
		return fmt.Errorf("object not found: %s", name)
	}

	// For specific types, make sure we store a copy to prevent modification
	switch v := value.(type) {
	case *types.Service:
		// Create a copy to store
		obj := *v
		objMap[name] = obj
	default:
		objMap[name] = value
	}

	return nil
}

// Delete deletes an object from the memory store.
func (m *MemoryStore) Delete(ctx context.Context, resourceType, namespace, name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	nsMap, ok := m.data[resourceType]
	if !ok {
		return fmt.Errorf("resource type not found: %s", resourceType)
	}

	objMap, ok := nsMap[namespace]
	if !ok {
		return fmt.Errorf("namespace not found: %s", namespace)
	}

	_, exists := objMap[name]
	if !exists {
		return fmt.Errorf("object not found: %s", name)
	}

	delete(objMap, name)
	return nil
}

// For testing, we provide minimal implementations of remaining interface methods

func (m *MemoryStore) GetHistory(ctx context.Context, resourceType, namespace, name string) ([]HistoricalVersion, error) {
	return []HistoricalVersion{}, nil
}

func (m *MemoryStore) GetVersion(ctx context.Context, resourceType, namespace, name, version string) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MemoryStore) Transaction(ctx context.Context, fn func(ctx Transaction) error) error {
	return fmt.Errorf("not implemented")
}

func (m *MemoryStore) Watch(ctx context.Context, resourceType, namespace string) (<-chan WatchEvent, error) {
	ch := make(chan WatchEvent)
	close(ch)
	return ch, nil
}

// Helper methods for testing

// EnsureResourceType ensures that a resource type exists in the store
func (m *MemoryStore) EnsureResourceType(resourceType string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, ok := m.data[resourceType]; !ok {
		m.data[resourceType] = make(map[string]map[string]interface{})
	}
}

// EnsureNamespace ensures that a namespace exists for a resource type
func (m *MemoryStore) EnsureNamespace(resourceType, namespace string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First make sure resource type exists
	resourceMap, ok := m.data[resourceType]
	if !ok {
		resourceMap = make(map[string]map[string]interface{})
		m.data[resourceType] = resourceMap
	}

	// Then make sure namespace exists
	if _, ok := resourceMap[namespace]; !ok {
		resourceMap[namespace] = make(map[string]interface{})
	}
}
