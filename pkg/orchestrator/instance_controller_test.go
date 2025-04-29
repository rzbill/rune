package orchestrator

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/manager"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestController creates a controller with test dependencies
func setupTestController(t *testing.T) (context.Context, *store.TestStore, *runner.TestRunner, InstanceController) {
	ctx := context.Background()
	testStore := store.NewTestStore()
	testRunner := runner.NewTestRunner()
	testRunnerMgr := manager.NewTestRunnerManager(nil)
	testRunnerMgr.SetDockerRunner(testRunner)
	testRunnerMgr.SetProcessRunner(testRunner)
	testLogger := log.NewLogger()

	controller := NewInstanceController(testStore, testRunnerMgr, testLogger)
	return ctx, testStore, testRunner, controller
}

// createTestService creates a test service in the store
func createTestService(ctx context.Context, t *testing.T, testStore *store.TestStore, name string, restartPolicy types.RestartPolicy) *types.Service {
	service := &types.Service{
		ID:            name,
		Name:          name,
		Namespace:     "default",
		RestartPolicy: restartPolicy,
		Image:         "test-image:latest",
		Command:       "test-command",
		Args:          []string{"arg1", "arg2"},
		Runtime:       "container",
		Env: map[string]string{
			"ENV_VAR1": "value1",
			"ENV_VAR2": "value2",
		},
	}

	err := testStore.CreateService(ctx, service)
	require.NoError(t, err, "Failed to create test service")
	return service
}

// TestCreateInstance tests the CreateInstance method
func TestCreateInstance(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service
	service := createTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Test creating an instance for the service
	instance, err := controller.CreateInstance(ctx, service, "test-instance")
	require.NoError(t, err, "CreateInstance should not return an error")
	assert.NotNil(t, instance, "Instance should not be nil")
	assert.Equal(t, "test-instance", instance.Name, "Instance Name should match")
	assert.Equal(t, service.ID, instance.ServiceID, "Instance should reference the service")
	assert.Equal(t, types.InstanceStatusRunning, instance.Status, "Instance should be running")

	// Verify instance was stored
	storedInstance, err := testStore.GetInstance(ctx, "default", instance.ID)
	require.NoError(t, err, "Instance should be in the store")
	assert.Equal(t, instance.ID, storedInstance.ID, "Stored instance ID should match")

	// Verify runner calls were made
	assert.Contains(t, testRunner.CreatedInstances, instance, "Runner should have created the instance")
	assert.Contains(t, testRunner.StartedInstances, instance.ID, "Runner should have started the instance")

	// Verify environment variables
	assert.Contains(t, instance.Environment, "RUNE_SERVICE_NAME", "Should have service name env var")
	assert.Contains(t, instance.Environment, "ENV_VAR1", "Should have service env vars")
	assert.Equal(t, "value1", instance.Environment["ENV_VAR1"], "Env var should have correct value")
}

// TestDeleteInstance tests the DeleteInstance method
func TestDeleteInstance(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := createTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Create instance directly in store and runner
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		Namespace: "default",
		ServiceID: service.ID,
		Status:    types.InstanceStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := testStore.CreateInstance(ctx, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Add to test runner's tracked instances
	err = testRunner.Create(ctx, instance)
	require.NoError(t, err, "Failed to add instance to runner")

	// Test deleting the instance
	err = controller.DeleteInstance(ctx, instance)
	require.NoError(t, err, "DeleteInstance should not return an error")

	// Verify instance status was updated
	storedInstance, err := testStore.GetInstance(ctx, "default", "test-instance")
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, storedInstance.Status, "Instance status should be stopped")

	// Verify runner calls were made
	assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Runner should have stopped the instance")
	assert.Contains(t, testRunner.RemovedInstances, instance.ID, "Runner should have removed the instance")
}

// TestGetInstanceStatus tests the GetInstanceStatus method
func TestGetInstanceStatus(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := createTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Create instance directly in store
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		Namespace: "default",
		ServiceID: service.ID,
		Status:    types.InstanceStatusRunning,
		NodeID:    "test-node",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := testStore.CreateInstance(ctx, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Set up test runner status
	testRunner.StatusResults[instance.ID] = types.InstanceStatusRunning

	// Test getting the instance status
	statusInfo, err := controller.GetInstanceStatus(ctx, instance)
	require.NoError(t, err, "GetInstanceStatus should not return an error")
	assert.Equal(t, types.InstanceStatusRunning, statusInfo.State, "Status should be running")
	assert.Equal(t, instance.ID, statusInfo.InstanceID, "Instance ID should match")
	assert.Equal(t, instance.NodeID, statusInfo.NodeID, "Node ID should match")

	// Verify runner call was made
	assert.Contains(t, testRunner.StatusCalls, instance.ID, "Runner should have been called for status")
}

// TestGetInstanceLogs tests the GetInstanceLogs method
func TestGetInstanceLogs(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := createTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Create instance directly in store
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		Namespace: "default",
		ServiceID: service.ID,
		Status:    types.InstanceStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := testStore.CreateInstance(ctx, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Set up test log output in runner
	testLogContent := []byte("Test log output\nLine 2\nLine 3")
	testRunner.LogOutput = testLogContent

	// Test getting the instance logs
	logOpts := LogOptions{
		Follow:     false,
		Tail:       10,
		Timestamps: false,
	}

	logs, err := controller.GetInstanceLogs(ctx, instance, logOpts)
	require.NoError(t, err, "GetInstanceLogs should not return an error")
	defer logs.Close()

	// Read log content
	content, err := io.ReadAll(logs)
	require.NoError(t, err, "Should be able to read logs")
	assert.Equal(t, testLogContent, content, "Log content should match")

	// Verify runner call was made
	assert.Contains(t, testRunner.LogCalls, instance.ID, "Runner should have been called for logs")
}

// TestRestartInstance tests the RestartInstance method
func TestRestartInstance(t *testing.T) {
	// Create test objects
	testStore := store.NewTestStore()
	testRunner := runner.NewTestRunner()
	testRunnerMgr := manager.NewTestRunnerManager(nil)
	testRunnerMgr.SetDockerRunner(testRunner)
	testRunnerMgr.SetProcessRunner(testRunner)
	testLogger := log.NewLogger()

	// Create the instance controller
	controller := NewInstanceController(testStore, testRunnerMgr, testLogger)

	// Create test instances
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		Namespace: "default",
		ServiceID: "test-service",
		Status:    types.InstanceStatusRunning,
	}

	// Create test service with Always restart policy
	serviceAlways := &types.Service{
		ID:            "test-service",
		Name:          "test-service",
		Namespace:     "default",
		RestartPolicy: types.RestartPolicyAlways,
		Runtime:       "container",
	}

	// Add service to store
	ctx := context.Background()
	err := testStore.CreateService(ctx, serviceAlways)
	assert.NoError(t, err)

	// Add instance to store
	err = testStore.CreateInstance(ctx, instance)
	assert.NoError(t, err)

	// Call the method
	err = controller.(interface {
		RestartInstance(context.Context, *types.Instance, string) error
	}).
		RestartInstance(context.Background(), instance, "test-restart")

	// Verify results
	assert.NoError(t, err)
	assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Instance should have been stopped")
	assert.Contains(t, testRunner.CreatedInstances, instance, "Instance should have been created")
	assert.Contains(t, testRunner.StartedInstances, instance.ID, "Instance should have been started")

	// Test with OnFailure restart policy
	serviceOnFailure := &types.Service{
		ID:            "test-service",
		Name:          "test-service",
		Namespace:     "default",
		RestartPolicy: types.RestartPolicyOnFailure,
		Runtime:       "container",
	}

	// Reset between tests
	testStore.Reset()
	testRunner = runner.NewTestRunner() // Create a fresh runner
	testRunnerMgr = manager.NewTestRunnerManager(nil)
	testRunnerMgr.SetDockerRunner(testRunner)
	testRunnerMgr.SetProcessRunner(testRunner)
	controller = NewInstanceController(testStore, testRunnerMgr, testLogger)

	// Set up test data for OnFailure policy
	err = testStore.CreateService(ctx, serviceOnFailure)
	assert.NoError(t, err)

	err = testStore.CreateInstance(ctx, instance)
	assert.NoError(t, err)

	// Call with non-failure reason - should skip restart
	err = controller.(interface {
		RestartInstance(context.Context, *types.Instance, string) error
	}).
		RestartInstance(context.Background(), instance, "manual-restart")

	// Should not have any operations performed
	assert.NoError(t, err)
	assert.Empty(t, testRunner.StoppedInstances, "Instance should not have been stopped")
	assert.Empty(t, testRunner.CreatedInstances, "Instance should not have been created")
	assert.Empty(t, testRunner.StartedInstances, "Instance should not have been started")

	// Now call with failure reason - should restart
	err = controller.(interface {
		RestartInstance(context.Context, *types.Instance, string) error
	}).
		RestartInstance(context.Background(), instance, "failure")

	// Verify that operations were performed
	assert.NoError(t, err)
	assert.Len(t, testRunner.StoppedInstances, 1, "Instance should have been stopped once")
	assert.Len(t, testRunner.CreatedInstances, 1, "Instance should have been created once")
	assert.Len(t, testRunner.StartedInstances, 1, "Instance should have been started once")
}

// TestUpdateInstance tests the UpdateInstance method
func TestUpdateInstance(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service
	service := createTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Create instance directly in store
	originalUpdateTime := time.Now().Add(-1 * time.Hour) // Old update time
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		Namespace: "default",
		ServiceID: service.ID,
		Status:    types.InstanceStatusRunning,
		Environment: map[string]string{
			"RUNE_SERVICE_NAME":   "test-service",
			"RUNE_ORIGINAL_IMAGE": "test-image:latest",
			"ENV_VAR1":            "old-value", // This will be updated
		},
		CreatedAt: time.Now(),
		UpdatedAt: originalUpdateTime,
	}

	err := testStore.CreateInstance(ctx, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Set up test runner status
	testRunner.StatusResults[instance.ID] = types.InstanceStatusRunning

	// Modify the service to test updating the instance
	service.Env["ENV_VAR1"] = "new-value"   // Changed value
	service.Env["ENV_VAR3"] = "added-value" // New env var

	// Test updating the instance
	err = controller.UpdateInstance(ctx, service, instance)
	require.NoError(t, err, "UpdateInstance should not return an error")

	// Verify instance was updated in store
	updatedInstance, err := testStore.GetInstance(ctx, "default", instance.ID)
	require.NoError(t, err, "Instance should be in the store")

	// Check that the environment was updated correctly
	assert.Equal(t, "new-value", updatedInstance.Environment["ENV_VAR1"], "ENV_VAR1 should be updated")
	assert.Equal(t, "added-value", updatedInstance.Environment["ENV_VAR3"], "ENV_VAR3 should be added")

	// Check that updateAt time is newer
	assert.NotEqual(t, originalUpdateTime, updatedInstance.UpdatedAt, "UpdatedAt should be changed")

	// Separate test for incompatible update
	TestUpdateInstanceIncompatible(t)
}

// TestUpdateInstanceIncompatible tests instance update with incompatible changes
func TestUpdateInstanceIncompatible(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create service and instance that won't be compatible after changes
	service := &types.Service{
		ID:            "test-service-incompatible",
		Name:          "test-service-incompatible",
		Namespace:     "default",
		RestartPolicy: types.RestartPolicyAlways,
		Image:         "original-image:latest",
		Runtime:       "docker",
	}

	err := testStore.CreateService(ctx, service)
	require.NoError(t, err, "Failed to create test service")

	// Create instance with original image
	instance := &types.Instance{
		ID:          "instance-to-recreate",
		Name:        "instance-to-recreate",
		Namespace:   "default",
		ServiceID:   service.ID,
		Status:      types.InstanceStatusRunning,
		ContainerID: "container123", // Important for docker runtime incompatibility check
		Environment: map[string]string{
			"RUNE_ORIGINAL_IMAGE": "original-image:latest",
		},
	}

	err = testStore.CreateInstance(ctx, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Set up test runner status
	testRunner.StatusResults[instance.ID] = types.InstanceStatusRunning

	// Create a modified service with different image
	modifiedService := &types.Service{
		ID:            service.ID,
		Name:          service.Name,
		Namespace:     service.Namespace,
		RestartPolicy: service.RestartPolicy,
		Image:         "different-image:latest", // This should trigger incompatibility
		Runtime:       "docker",
	}

	// Update should fail due to incompatibility
	err = controller.UpdateInstance(ctx, modifiedService, instance)
	assert.Error(t, err, "UpdateInstance should return an error for incompatible changes")
	assert.Contains(t, err.Error(), "requires recreation", "Error should indicate recreation is needed")
}
