package store

import (
	"fmt"
	"strings"
)

// MakeKey creates a standardized key for a resource.
func MakeKey(resourceType, namespace, name string) []byte {
	return []byte(fmt.Sprintf("%s/%s/%s", resourceType, namespace, name))
}

// MakeVersionKey creates a standardized key for a resource version.
func MakeVersionKey(resourceType, namespace, name, version string) []byte {
	return []byte(fmt.Sprintf("%s-versions/%s/%s/%s", resourceType, namespace, name, version))
}

// MakePrefix creates a prefix for listing resources by type and namespace.
func MakePrefix(resourceType, namespace string) []byte {
	return []byte(fmt.Sprintf("%s/%s/", resourceType, namespace))
}

// MakeVersionPrefix creates a prefix for listing resource versions.
func MakeVersionPrefix(resourceType, namespace, name string) []byte {
	return []byte(fmt.Sprintf("%s-versions/%s/%s/", resourceType, namespace, name))
}

// ParseKey parses a key into its components.
func ParseKey(key []byte) (resourceType, namespace, name string, ok bool) {
	parts := strings.Split(string(key), "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// ParseVersionKey parses a version key into its components.
func ParseVersionKey(key []byte) (resourceType, namespace, name, version string, ok bool) {
	s := string(key)
	if !strings.Contains(s, "-versions/") {
		return "", "", "", "", false
	}

	// Remove the -versions part
	resourceType = strings.Split(s, "-versions/")[0]

	// Get the rest of the path
	rest := s[len(resourceType)+len("-versions/"):]
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return "", "", "", "", false
	}

	return resourceType, parts[0], parts[1], parts[2], true
}

// ResourceTypes returns a slice of the standard resource types used in the system.
func ResourceTypes() []string {
	return []string{
		"services",
		"instances",
		"configs",
		"secrets",
		"functions",
		"jobs",
		"jobruns",
		"networks",
		"volumes",
	}
}
