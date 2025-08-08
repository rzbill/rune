package controllers

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
func instanceControllerCreateTestService(ctx context.Context, t *testing.T, testStore *store.TestStore, name string, restartPolicy types.RestartPolicy) *types.Service {
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
		Metadata: &types.ServiceMetadata{
			Generation: 1,
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
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Test creating an instance for the service
	instance, err := controller.CreateInstance(ctx, service, "test-instance-0")
	require.NoError(t, err, "CreateInstance should not return an error")
	assert.NotNil(t, instance, "Instance should not be nil")
	assert.Equal(t, "test-instance-0", instance.Name, "Instance Name should match")
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
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

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
	assert.Equal(t, types.InstanceStatusDeleted, storedInstance.Status, "Instance status should be deleted")

	// Verify runner calls were made
	assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Runner should have stopped the instance")
	assert.Contains(t, testRunner.RemovedInstances, instance.ID, "Runner should have removed the instance")
}

// TestStopInstance tests the StopInstance method
func TestStopInstance(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Create instance directly in store and runner
	instance := &types.Instance{
		ID:        "test-instance-stop",
		Name:      "test-instance-stop",
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

	// Test stopping the instance
	err = controller.StopInstance(ctx, instance)
	require.NoError(t, err, "StopInstance should not return an error")

	// Verify instance status was updated to stopped
	storedInstance, err := testStore.GetInstance(ctx, "default", "test-instance-stop")
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, storedInstance.Status, "Instance status should be stopped")
	assert.Equal(t, "Stopped by user", storedInstance.StatusMessage, "Status message should indicate stopped by user")

	// Verify runner stop call was made
	assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Runner should have stopped the instance")
}

// TestGetInstanceStatus tests the GetInstanceStatus method
func TestGetInstanceStatus(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

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
	assert.Equal(t, types.InstanceStatusRunning, statusInfo.Status, "Status should be running")
	assert.Equal(t, instance.ID, statusInfo.InstanceID, "Instance ID should match")
	assert.Equal(t, instance.NodeID, statusInfo.NodeID, "Node ID should match")

	// Verify runner call was made
	assert.Contains(t, testRunner.StatusCalls, instance.ID, "Runner should have been called for status")
}

// TestGetInstanceLogs tests the GetInstanceLogs method
func TestGetInstanceLogs(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

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
	logOpts := types.LogOptions{
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

// TestExec tests the Exec method
func TestExec(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service and instance
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

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

	// Test command to execute
	command := []string{"ls", "-la"}

	// Set up test output in runner
	testStdoutContent := []byte("total 123\ndrwxr-xr-x 7 user group 224 Feb 10 12:34 .\n")
	testStderrContent := []byte("Warning: some files not accessible\n")
	testExitCode := 0

	// Configure test runner's exec behavior
	testRunner.ExecOutput = testStdoutContent
	testRunner.ExecErrOutput = testStderrContent
	testRunner.ExitCodeVal = testExitCode

	// Create exec options
	execOpts := types.ExecOptions{
		Command:        command,
		Env:            map[string]string{"TEST_VAR": "test_value"},
		WorkingDir:     "/app",
		TTY:            true,
		TerminalWidth:  80,
		TerminalHeight: 24,
	}

	// Call Exec
	execStream, err := controller.Exec(ctx, instance, execOpts)
	require.NoError(t, err, "Exec should not return an error")
	require.NotNil(t, execStream, "ExecStream should not be nil")
	defer execStream.Close()

	// Read stdout
	stdoutBuf := make([]byte, 1024)
	n, err := execStream.Read(stdoutBuf)
	require.NoError(t, err, "Should be able to read stdout")
	assert.Equal(t, testStdoutContent, stdoutBuf[:n], "Stdout content should match")

	// Read stderr
	stderrReader := execStream.Stderr()
	stderrBuf := make([]byte, 1024)
	n, err = stderrReader.Read(stderrBuf)
	require.NoError(t, err, "Should be able to read stderr")
	assert.Equal(t, testStderrContent, stderrBuf[:n], "Stderr content should match")

	// Get exit code
	exitCode, err := execStream.ExitCode()
	require.NoError(t, err, "Should be able to get exit code")
	assert.Equal(t, testExitCode, exitCode, "Exit code should match")

	// Verify runner call was made
	assert.Contains(t, testRunner.ExecCalls, instance.ID, "Runner should have been called for exec")

	// Verify command was passed correctly - index is a number since we use a slice
	lastExecOpts := testRunner.ExecOptions[len(testRunner.ExecOptions)-1]
	assert.Equal(t, command, lastExecOpts.Command, "Command should match")
	assert.Equal(t, execOpts.Env, lastExecOpts.Env, "Environment variables should match")
	assert.Equal(t, execOpts.WorkingDir, lastExecOpts.WorkingDir, "Working directory should match")
	assert.Equal(t, execOpts.TTY, lastExecOpts.TTY, "TTY setting should match")
	assert.Equal(t, execOpts.TerminalWidth, lastExecOpts.TerminalWidth, "Terminal width should match")
	assert.Equal(t, execOpts.TerminalHeight, lastExecOpts.TerminalHeight, "Terminal height should match")
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

	ctx := context.Background()

	t.Run("RestartPolicy=Always", func(t *testing.T) {
		// Create test service with Always restart policy
		serviceAlways := &types.Service{
			ID:            "test-service",
			Name:          "test-service",
			Namespace:     "default",
			RestartPolicy: types.RestartPolicyAlways,
			Runtime:       "container",
		}

		// Add service to store
		err := testStore.CreateService(ctx, serviceAlways)
		assert.NoError(t, err)

		// Add instance to store
		err = testStore.CreateInstance(ctx, instance)
		assert.NoError(t, err)

		// Call the method
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonManual)

		// Verify results
		assert.NoError(t, err)
		assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Instance should have been stopped")
		assert.Contains(t, testRunner.CreatedInstances, instance, "Instance should have been created")
		assert.Contains(t, testRunner.StartedInstances, instance.ID, "Instance should have been started")
	})

	// Test with OnFailure restart policy
	t.Run("RestartPolicy=OnFailure", func(t *testing.T) {
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
		err := testStore.CreateService(ctx, serviceOnFailure)
		assert.NoError(t, err)

		err = testStore.CreateInstance(ctx, instance)
		assert.NoError(t, err)

		// Call with non-failure reason - should skip restart
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonUpdate)

		// Should not have any operations performed
		assert.NoError(t, err)
		assert.Empty(t, testRunner.StoppedInstances, "Instance should not have been stopped with update reason")
		assert.Empty(t, testRunner.CreatedInstances, "Instance should not have been created with update reason")
		assert.Empty(t, testRunner.StartedInstances, "Instance should not have been started with update reason")

		// Now call with failure reason - should restart
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonFailure)

		// Verify that operations were performed
		assert.NoError(t, err)
		assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Instance should have been stopped with failure reason")
		assert.Contains(t, testRunner.CreatedInstances, instance, "Instance should have been created with failure reason")
		assert.Contains(t, testRunner.StartedInstances, instance.ID, "Instance should have been started with failure reason")

		// Reset runner for next test
		testRunner = runner.NewTestRunner()
		testRunnerMgr.SetDockerRunner(testRunner)
		testRunnerMgr.SetProcessRunner(testRunner)

		// Call with health check failure reason - should restart
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonHealthCheckFailure)

		// Verify that operations were performed for health check failure
		assert.NoError(t, err)
		assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Instance should have been stopped with health check failure reason")
		assert.Contains(t, testRunner.CreatedInstances, instance, "Instance should have been created with health check failure reason")
		assert.Contains(t, testRunner.StartedInstances, instance.ID, "Instance should have been started with health check failure reason")

		// Reset runner for next test
		testRunner = runner.NewTestRunner()
		testRunnerMgr.SetDockerRunner(testRunner)
		testRunnerMgr.SetProcessRunner(testRunner)

		// Now call with manual restart - should always restart regardless of policy
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonManual)
		assert.NoError(t, err)
		assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Instance should have been stopped with manual reason")
		assert.Contains(t, testRunner.CreatedInstances, instance, "Instance should have been created with manual reason")
		assert.Contains(t, testRunner.StartedInstances, instance.ID, "Instance should have been started with manual reason")
	})

	// Test with Never restart policy
	t.Run("RestartPolicy=Never", func(t *testing.T) {
		serviceOnNever := &types.Service{
			ID:            "test-service",
			Name:          "test-service",
			Namespace:     "default",
			RestartPolicy: types.RestartPolicyNever,
			Runtime:       "container",
		}

		// Reset between tests
		testStore.Reset()
		testRunner = runner.NewTestRunner() // Create a fresh runner
		testRunnerMgr = manager.NewTestRunnerManager(nil)
		testRunnerMgr.SetDockerRunner(testRunner)
		testRunnerMgr.SetProcessRunner(testRunner)
		controller = NewInstanceController(testStore, testRunnerMgr, testLogger)

		err := testStore.CreateService(ctx, serviceOnNever)
		assert.NoError(t, err)

		err = testStore.CreateInstance(ctx, instance)
		assert.NoError(t, err)

		// Call with automatic restart reasons - should not restart
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonFailure)
		assert.NoError(t, err)
		assert.Empty(t, testRunner.StoppedInstances, "Instance should not have been stopped with failure reason")
		assert.Empty(t, testRunner.CreatedInstances, "Instance should not have been created with failure reason")
		assert.Empty(t, testRunner.StartedInstances, "Instance should not have been started with failure reason")

		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonHealthCheckFailure)
		assert.NoError(t, err)
		assert.Empty(t, testRunner.StoppedInstances, "Instance should not have been stopped with health check failure reason")
		assert.Empty(t, testRunner.CreatedInstances, "Instance should not have been created with health check failure reason")
		assert.Empty(t, testRunner.StartedInstances, "Instance should not have been started with health check failure reason")

		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonUpdate)
		assert.NoError(t, err)
		assert.Empty(t, testRunner.StoppedInstances, "Instance should not have been stopped with update reason")
		assert.Empty(t, testRunner.CreatedInstances, "Instance should not have been created with update reason")
		assert.Empty(t, testRunner.StartedInstances, "Instance should not have been started with update reason")

		// Reset runner for next test
		testRunner = runner.NewTestRunner()
		testRunnerMgr.SetDockerRunner(testRunner)
		testRunnerMgr.SetProcessRunner(testRunner)

		// Call with manual restart - should restart even with Never policy
		err = controller.RestartInstance(context.Background(), instance, InstanceRestartReasonManual)
		assert.NoError(t, err)
		assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Instance should have been stopped with manual reason")
		assert.Contains(t, testRunner.CreatedInstances, instance, "Instance should have been created with manual reason")
		assert.Contains(t, testRunner.StartedInstances, instance.ID, "Instance should have been started with manual reason")
	})
}

// TestUpdateInstance tests the UpdateInstance method
func TestUpdateInstance(t *testing.T) {
	ctx, testStore, testRunner, controller := setupTestController(t)

	// Create a test service
	service := instanceControllerCreateTestService(ctx, t, testStore, "test-service", types.RestartPolicyAlways)

	// Create instance directly in store
	originalUpdateTime := time.Now().Add(-1 * time.Hour) // Old update time
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test-instance",
		Namespace: "default",
		ServiceID: service.ID,
		Status:    types.InstanceStatusRunning,
		Environment: map[string]string{
			"RUNE_SERVICE_NAME": "test-service",
			"ENV_VAR1":          "old-value", // This will be updated
		},
		Metadata: &types.InstanceMetadata{
			Image:             "original-image:latest",
			ServiceGeneration: 1, // Match the service generation
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
		Metadata: &types.ServiceMetadata{
			Generation: 1,
		},
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
		Metadata: &types.InstanceMetadata{
			Image: "original-image:latest",
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
		Metadata: &types.ServiceMetadata{
			Generation: 2, // Increment generation to trigger incompatibility
		},
	}

	// Update should fail due to incompatibility
	err = controller.UpdateInstance(ctx, modifiedService, instance)
	assert.Error(t, err, "UpdateInstance should return an error for incompatible changes")
	assert.Contains(t, err.Error(), "requires recreation", "Error should indicate recreation is needed")
}
