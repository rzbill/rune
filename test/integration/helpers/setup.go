package helpers

import (
	"context"
	"os"
	"testing"
	"time"
)

var (
	AlpineImage = "alpine:latest"
)

// IntegrationTest provides a structured way to set up and tear down integration tests
func IntegrationTest(t *testing.T, testFunc func(ctx context.Context, h *DockerHelper)) {
	// Skip test if the SKIP_INTEGRATION_TESTS environment variable is set
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test because SKIP_INTEGRATION_TESTS=true")
	}

	// Skip if integration tests are only allowed in CI and we're not in CI
	if os.Getenv("INTEGRATION_TESTS_CI_ONLY") == "true" && os.Getenv("CI") != "true" {
		t.Skip("Skipping integration test because INTEGRATION_TESTS_CI_ONLY=true and not running in CI")
	}

	// Create a docker helper
	h, err := NewDockerHelper()
	if err != nil {
		t.Fatalf("Failed to create Docker helper: %v", err)
	}
	if h == nil {
		// Test has already been skipped in NewDockerHelper if Docker is not available
		return
	}

	// Create a context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run the test function
	testFunc(ctx, h)
}

// RequireEnvVar ensures that an environment variable is set, skipping the test if not present
func RequireEnvVar(t *testing.T, name string) string {
	value := os.Getenv(name)
	if value == "" {
		t.Skipf("Required environment variable %s is not set", name)
	}
	return value
}

// GetEnvOrDefault gets an environment variable or returns the default value if not set
func GetEnvOrDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

// IsCI returns true if running in a CI environment
func IsCI() bool {
	return os.Getenv("CI") == "true"
}
