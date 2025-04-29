// Package store provides a state storage interface and implementations for the Rune platform.
package store

import (
	"context"
	"time"
)

// Store defines the interface for state storage operations.
type Store interface {
	// Open initializes and opens the store.
	Open(path string) error

	// Close closes the store and releases resources.
	Close() error

	// Create creates a new resource.
	Create(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error

	// Get retrieves a resource by type, namespace, and name.
	Get(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error

	// List retrieves all resources of a given type in a namespace.
	List(ctx context.Context, resourceType string, namespace string, resource interface{}) error

	// Update updates an existing resource.
	Update(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error

	// Delete deletes a resource.
	Delete(ctx context.Context, resourceType string, namespace string, name string) error

	// Watch sets up a watch for changes to resources of a given type.
	Watch(ctx context.Context, resourceType string, namespace string) (<-chan WatchEvent, error)

	// Transaction executes multiple operations in a single transaction.
	Transaction(ctx context.Context, fn func(tx Transaction) error) error

	// GetHistory retrieves historical versions of a resource.
	GetHistory(ctx context.Context, resourceType string, namespace string, name string) ([]HistoricalVersion, error)

	// GetVersion retrieves a specific version of a resource.
	GetVersion(ctx context.Context, resourceType string, namespace string, name string, version string) (interface{}, error)
}

// Transaction represents a store transaction.
type Transaction interface {
	// Create creates a new resource within the transaction.
	Create(resourceType string, namespace string, name string, resource interface{}) error

	// Get retrieves a resource within the transaction.
	Get(resourceType string, namespace string, name string, resource interface{}) error

	// Update updates a resource within the transaction.
	Update(resourceType string, namespace string, name string, resource interface{}) error

	// Delete deletes a resource within the transaction.
	Delete(resourceType string, namespace string, name string) error
}

// WatchEventType defines the type of watch event.
type WatchEventType string

const (
	// WatchEventCreated indicates a resource was created.
	WatchEventCreated WatchEventType = "CREATED"

	// WatchEventUpdated indicates a resource was updated.
	WatchEventUpdated WatchEventType = "UPDATED"

	// WatchEventDeleted indicates a resource was deleted.
	WatchEventDeleted WatchEventType = "DELETED"
)

// WatchEvent represents a change to a resource.
type WatchEvent struct {
	// Type is the type of event (created, updated, deleted).
	Type WatchEventType

	// ResourceType is the type of resource affected.
	ResourceType string

	// Namespace is the namespace of the resource.
	Namespace string

	// Name is the name of the resource.
	Name string

	// Resource is the resource data.
	Resource interface{}
}

// HistoricalVersion represents a historical version of a resource.
type HistoricalVersion struct {
	// Version is the version identifier.
	Version string

	// Timestamp is when this version was created.
	Timestamp time.Time

	// Resource is the resource data for this version.
	Resource interface{}
}
