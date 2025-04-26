# Rune State Store

The state store package provides persistent storage for Rune's resources, supporting CRUD operations, versioning, watching for changes, and transactional updates.

## Overview

The store package implements a flexible, extensible storage interface with BadgerDB as the primary implementation. The store provides:

- CRUD operations for any resource type
- Namespaced resources
- Resource versioning and history
- Atomic transactions
- Watch mechanisms for reactive patterns

## Interface

The core `Store` interface provides the following operations:

```go
// Store defines the interface for state storage
type Store interface {
    Open(path string) error
    Close() error
    Create(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error
    Get(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error
    List(ctx context.Context, resourceType string, namespace string) ([]interface{}, error)
    Update(ctx context.Context, resourceType string, namespace string, name string, resource interface{}) error
    Delete(ctx context.Context, resourceType string, namespace string, name string) error
    Watch(ctx context.Context, resourceType string, namespace string) (<-chan WatchEvent, error)
    Transaction(ctx context.Context, fn func(tx Transaction) error) error
    GetHistory(ctx context.Context, resourceType string, namespace string, name string) ([]HistoricalVersion, error)
    GetVersion(ctx context.Context, resourceType string, namespace string, name string, version string) (interface{}, error)
}
```

## Usage

### Creating a Store

```go
import (
    "context"
    "log"
    "razorbill.dev/rune/pkg/store"
)

// Create a BadgerDB store
logger := log.New(os.Stdout, "", log.LstdFlags)
s := store.NewBadgerStore(logger)

// Open the store
if err := s.Open("/path/to/data"); err != nil {
    log.Fatalf("Failed to open store: %v", err)
}
defer s.Close()
```

### Working with Resources

```go
// Define a resource
type Service struct {
    Name  string   `json:"name"`
    Image string   `json:"image"`
    Ports []int    `json:"ports"`
}

// Create a resource
service := Service{
    Name:  "my-service",
    Image: "nginx:latest",
    Ports: []int{80, 443},
}

ctx := context.Background()
if err := s.Create(ctx, "services", "default", "my-service", service); err != nil {
    log.Fatalf("Failed to create service: %v", err)
}

// Get a resource
var retrievedService Service
if err := s.Get(ctx, "services", "default", "my-service", &retrievedService); err != nil {
    log.Fatalf("Failed to get service: %v", err)
}

// Update a resource
service.Image = "nginx:1.19"
if err := s.Update(ctx, "services", "default", "my-service", service); err != nil {
    log.Fatalf("Failed to update service: %v", err)
}

// List resources
services, err := s.List(ctx, "services", "default")
if err != nil {
    log.Fatalf("Failed to list services: %v", err)
}

// Delete a resource
if err := s.Delete(ctx, "services", "default", "my-service"); err != nil {
    log.Fatalf("Failed to delete service: %v", err)
}
```

### Transactions

```go
// Execute multiple operations in a transaction
err := s.Transaction(ctx, func(tx store.Transaction) error {
    // Create a service
    service := Service{Name: "service-1", Image: "image-1"}
    if err := tx.Create("services", "default", "service-1", service); err != nil {
        return err
    }
    
    // Create another service
    service2 := Service{Name: "service-2", Image: "image-2"}
    if err := tx.Create("services", "default", "service-2", service2); err != nil {
        return err
    }
    
    return nil
})
```

### Watching for Changes

```go
// Watch for changes to services in the default namespace
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

events, err := s.Watch(ctx, "services", "default")
if err != nil {
    log.Fatalf("Failed to set up watch: %v", err)
}

go func() {
    for event := range events {
        switch event.Type {
        case store.WatchEventCreated:
            log.Printf("Service created: %s", event.Name)
        case store.WatchEventUpdated:
            log.Printf("Service updated: %s", event.Name)
        case store.WatchEventDeleted:
            log.Printf("Service deleted: %s", event.Name)
        }
    }
}()
```

### Version History

```go
// Get version history
history, err := s.GetHistory(ctx, "services", "default", "my-service")
if err != nil {
    log.Fatalf("Failed to get history: %v", err)
}

// Get a specific version
version := history[1].Version
oldVersion, err := s.GetVersion(ctx, "services", "default", "my-service", version)
if err != nil {
    log.Fatalf("Failed to get version: %v", err)
}
```

## Implementation Notes

- Resources are stored as serialized JSON
- Keys are organized hierarchically by resource type, namespace, and name
- Versions are stored separately with metadata and timestamps
- Watch events are distributed through an in-memory pub/sub system

## Future Enhancements

- Addition of a metrics subsystem for monitoring store operations
- Advanced query capabilities with filtering and sorting
- Pluggable serialization formats
- Integration with external storage systems (etcd, etc.) 