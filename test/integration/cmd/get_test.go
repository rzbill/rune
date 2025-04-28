//go:build integration
// +build integration

package cmd_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rzbill/rune/pkg/types"
	"github.com/rzbill/rune/test/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommand(t *testing.T) {
	helper := helpers.NewTestHelper(t)
	defer helper.Cleanup()

	// Start API server
	apiServer := helper.StartAPIServer()

	// Set up test data
	ctx := context.Background()
	store := apiServer.GetStore()

	// Create test services in multiple namespaces
	services := []*types.Service{
		{
			ID:        "test-container-svc",
			Name:      "web",
			Namespace: "default",
			Image:     "nginx:latest",
			Scale:     2,
			Status:    types.ServiceStatusRunning,
		},
		{
			ID:        "test-process-svc",
			Name:      "logger",
			Namespace: "default",
			Runtime:   "process",
			Process: &types.ProcessSpec{
				Command: "/usr/bin/logger",
				Args:    []string{"--verbose"},
			},
			Scale:  1,
			Status: types.ServiceStatusRunning,
		},
		{
			ID:        "test-svc-prod",
			Name:      "api",
			Namespace: "prod",
			Image:     "api:v1",
			Scale:     3,
			Status:    types.ServiceStatusRunning,
		},
	}

	for _, svc := range services {
		err := store.SaveService(ctx, svc)
		require.NoError(t, err)
	}

	// Test cases
	tests := []struct {
		name           string
		args           []string
		expectedOutput []string
		notExpected    []string
	}{
		{
			name: "list all services",
			args: []string{"get", "services"},
			expectedOutput: []string{
				"web", "container", "Running", "nginx:latest",
				"logger", "process", "Running", "/usr/bin/logger",
			},
			notExpected: []string{"api", "prod"},
		},
		{
			name:           "get specific service",
			args:           []string{"get", "service", "web"},
			expectedOutput: []string{"web", "container", "Running", "nginx:latest"},
			notExpected:    []string{"logger", "api", "prod"},
		},
		{
			name: "get service with yaml output",
			args: []string{"get", "service", "web", "--output=yaml"},
			expectedOutput: []string{
				"name: web",
				"namespace: default",
				"image: nginx:latest",
			},
		},
		{
			name: "get service with json output",
			args: []string{"get", "service", "web", "--output=json"},
			expectedOutput: []string{
				"\"name\": \"web\"",
				"\"namespace\": \"default\"",
				"\"image\": \"nginx:latest\"",
			},
		},
		{
			name:           "list services with no headers",
			args:           []string{"get", "services", "--no-headers"},
			expectedOutput: []string{"web", "logger"},
			notExpected:    []string{"NAME", "TYPE", "STATUS", "api", "prod"},
		},
		{
			name: "list services across all namespaces",
			args: []string{"get", "services", "--all-namespaces"},
			expectedOutput: []string{
				"NAMESPACE", "NAME", "TYPE", "STATUS",
				"default", "web", "Running",
				"default", "logger", "Running",
				"prod", "api", "Running",
			},
		},
		{
			name:           "list services in specific namespace",
			args:           []string{"get", "services", "--namespace=prod"},
			expectedOutput: []string{"api", "Running"},
			notExpected:    []string{"web", "logger", "default"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := helper.RunCommand(tc.args...)
			require.NoError(t, err)

			for _, expected := range tc.expectedOutput {
				assert.Contains(t, output, expected)
			}

			for _, notExpected := range tc.notExpected {
				assert.NotContains(t, output, notExpected)
			}
		})
	}
}

func TestGetCommandErrors(t *testing.T) {
	helper := helpers.NewTestHelper(t)
	defer helper.Cleanup()

	// Start API server
	helper.StartAPIServer()

	// Test cases
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "invalid resource type",
			args:          []string{"get", "foo"},
			expectedError: "unsupported resource type: foo",
		},
		{
			name:          "missing resource type",
			args:          []string{"get"},
			expectedError: "requires at least 1 arg",
		},
		{
			name:          "service not found",
			args:          []string{"get", "service", "nonexistent"},
			expectedError: "not found",
		},
		{
			name:          "invalid output format",
			args:          []string{"get", "services", "--output=invalid"},
			expectedError: "unsupported output format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := helper.RunCommand(tc.args...)
			assert.Error(t, err)
			assert.Contains(t, output+err.Error(), tc.expectedError)
		})
	}
}

func TestGetCommandFilterAndSort(t *testing.T) {
	helper := helpers.NewTestHelper(t)
	defer helper.Cleanup()

	// Start API server
	apiServer := helper.StartAPIServer()

	// Set up test data with different statuses
	ctx := context.Background()
	store := apiServer.GetStore()

	// Create test services with different statuses
	services := []*types.Service{
		{
			ID:        "svc1",
			Name:      "web",
			Namespace: "default",
			Image:     "nginx:latest",
			Scale:     2,
			Status:    types.ServiceStatusRunning,
		},
		{
			ID:        "svc2",
			Name:      "api",
			Namespace: "default",
			Image:     "api:v1",
			Scale:     1,
			Status:    types.ServiceStatusPending,
		},
		{
			ID:        "svc3",
			Name:      "db",
			Namespace: "default",
			Image:     "postgres:13",
			Scale:     1,
			Status:    types.ServiceStatusFailed,
		},
	}

	for _, svc := range services {
		err := store.SaveService(ctx, svc)
		require.NoError(t, err)
	}

	// Test cases
	tests := []struct {
		name          string
		args          []string
		expectedOrder []string
		notInOutput   []string
	}{
		{
			name:          "sort by name",
			args:          []string{"get", "services", "--sort-by=name"},
			expectedOrder: []string{"api", "db", "web"},
		},
		{
			name:          "sort by status",
			args:          []string{"get", "services", "--sort-by=status"},
			expectedOrder: []string{"Failed", "Pending", "Running"},
		},
		{
			name:          "filter by field selector (status)",
			args:          []string{"get", "services", "--field-selector=status=Running"},
			expectedOrder: []string{"web"},
			notInOutput:   []string{"api", "db"},
		},
		{
			name:          "filter by field selector (name)",
			args:          []string{"get", "services", "--field-selector=name=api"},
			expectedOrder: []string{"api"},
			notInOutput:   []string{"web", "db"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := helper.RunCommand(tc.args...)
			require.NoError(t, err)

			// Check expected order
			lastPos := -1
			for i, name := range tc.expectedOrder {
				pos := strings.Index(output, name)
				assert.Greater(t, pos, lastPos, "Expected %s to appear after previous item", name)
				lastPos = pos
			}

			// Check items that shouldn't be in output
			for _, notExpected := range tc.notInOutput {
				assert.NotContains(t, output, notExpected)
			}
		})
	}
}

// TestGetCommandWatch tests the service watch functionality
func TestGetCommandWatch(t *testing.T) {
	helper := helpers.NewTestHelper(t)
	defer helper.Cleanup()

	// Start API server
	apiServer := helper.StartAPIServer()

	// Set up test data
	ctx := context.Background()
	store := apiServer.GetStore()

	// Create initial test service
	initialService := &types.Service{
		ID:        "watch-test-svc",
		Name:      "watch-app",
		Namespace: "default",
		Image:     "watch:latest",
		Scale:     1,
		Status:    types.ServiceStatusPending,
	}
	err := store.SaveService(ctx, initialService)
	require.NoError(t, err)

	// Run the watch command with a timeout
	// This tests that we can start the watch and receive initial events
	timeout := 3 // seconds
	output, err := helper.RunCommandWithTimeout(timeout, "get", "services", "--watch")

	// Check results - the command should be killed by the timeout, which is expected
	assert.Contains(t, err.Error(), "killed")

	// Verify that the watch output contains the service and headers
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "watch-app")
	assert.Contains(t, output, "Pending")

	// Test specific service watch
	output, err = helper.RunCommandWithTimeout(timeout, "get", "service", "watch-app", "--watch")

	// Check results - again, command should be killed by timeout
	assert.Contains(t, err.Error(), "killed")

	// Verify that the service is displayed in watch mode
	assert.Contains(t, output, "watch-app")
	assert.Contains(t, output, "Pending")

	// Ensure we don't see other services
	assert.NotContains(t, output, "web")
	assert.NotContains(t, output, "logger")
}
