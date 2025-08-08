package controllers

import (
	"fmt"
	"strings"

	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

// WorkItemKey is a key for a work item
type workItemKey struct {
	ResourceType types.ResourceType
	Namespace    string
	Name         string
	EventType    string
}

// buildWorkItemKey builds a key for a work item
func buildWorkItemKey(event store.WatchEvent) string {
	return fmt.Sprintf("%s/%s/%s/%s", event.ResourceType, event.Namespace, event.Name, event.Type)
}

// parseWorkItemKey parses a key for a work item
func parseWorkItemKey(key string) (*workItemKey, error) {
	parts := strings.Split(key, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid work item key format: %s, expected at least 4 segments (resourceType/namespace/name/eventType)", key)
	}
	return &workItemKey{
		ResourceType: types.ResourceType(parts[0]),
		Namespace:    parts[1],
		Name:         parts[2],
		EventType:    parts[3],
	}, nil
}

// areStringSlicesEqual checks if two string slices are equal
func areStringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

// generateInstanceName generates a unique instance name for a service
func generateInstanceName(service *types.Service, index int) string {
	return fmt.Sprintf("%s-%d", service.Name, index)
}

// determineRequiredFinalizers determines which finalizers are needed for a service
func determineRequiredFinalizers(service *types.Service) []types.FinalizerType {
	var finalizers []types.FinalizerType

	// Always cleanup instances first
	finalizers = append(finalizers, types.FinalizerTypeInstanceCleanup)

	// Then deregister the service
	finalizers = append(finalizers, types.FinalizerTypeServiceDeregister)

	// Add other finalizers based on service configuration
	// TODO: Add volume cleanup, network cleanup, etc. when those features are implemented

	return finalizers
}
