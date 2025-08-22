package controllers

import (
	"fmt"
	"strings"
	"unicode"

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

// trimWhitespaces trims all whitespace from a string
func trimWhitespaces(value string) string {
	if value == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
}
