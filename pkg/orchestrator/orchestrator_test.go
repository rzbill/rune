package orchestrator

import (
	"context"
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

// setupTestOrchestrator creates an orchestrator with test dependencies
func setupTestOrchestrator(t *testing.T) (context.Context, *store.TestStore, *runner.TestRunner, Orchestrator) {
	ctx := context.Background()
	testStore := store.NewTestStore()
	testRunner := runner.NewTestRunner()
	testRunnerMgr := manager.NewTestRunnerManager(nil)
	testRunnerMgr.SetDockerRunner(testRunner)
	testRunnerMgr.SetProcessRunner(testRunner)
	testLogger := log.NewLogger()

	instanceController := NewInstanceController(testStore, testRunnerMgr, testLogger)
	healthController := NewHealthController(testLogger, testStore, testRunnerMgr)

	orchestrator := NewOrchestrator(testStore, instanceController, healthController, testRunnerMgr, testLogger)
	return ctx, testStore, testRunner, orchestrator
}

// createTestServiceWithInstances creates a test service with instances in the store
func createTestServiceWithInstances(ctx context.Context, t *testing.T, testStore *store.TestStore, instanceCtrl InstanceController, name string, count int) (*types.Service, []*types.Instance) {
	service := &types.Service{
		ID:            name,
		Name:          name,
		Namespace:     "default",
		RestartPolicy: types.RestartPolicyAlways,
		Image:         "test-image:latest",
		Command:       "test-command",
		Args:          []string{"arg1", "arg2"},
		Runtime:       "container",
		Env: map[string]string{
			"ENV_VAR1": "value1",
			"ENV_VAR2": "value2",
		},
		Scale: count,
	}

	err := testStore.CreateService(ctx, service)
	require.NoError(t, err, "Failed to create test service")

	instances := make([]*types.Instance, 0, count)
	for i := 0; i < count; i++ {
		instanceName := name + "-" + time.Now().Format("20060102-150405") + "-" + string(rune(i+48))
		instance, err := instanceCtrl.CreateInstance(ctx, service, instanceName)
		require.NoError(t, err, "Failed to create test instance")
		instances = append(instances, instance)
	}

	return service, instances
}

// TestExecInService tests the ExecInService method
func TestExecInService(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Get the instance controller to create instances
	instanceCtrl, ok := orch.(interface{ GetInstanceController() InstanceController })
	var instController InstanceController
	if ok {
		instController = instanceCtrl.GetInstanceController()
	} else {
		// Create a new instance controller if the accessor isn't available
		instController = NewInstanceController(testStore, manager.NewTestRunnerManager(testRunner), log.NewLogger())
	}

	// Create a test service with 2 instances
	service, instances := createTestServiceWithInstances(ctx, t, testStore, instController, "test-service", 2)

	// Set up test output in runner
	testStdoutContent := []byte("service exec output")
	testStderrContent := []byte("service exec error output")
	testExitCode := 0

	// Configure test runner
	testRunner.ExecOutput = testStdoutContent
	testRunner.ExecErrOutput = testStderrContent
	testRunner.ExitCodeVal = testExitCode

	// Test command to execute
	command := []string{"ps", "-ef"}

	// Create exec options
	execOpts := ExecOptions{
		Command: command,
		Env:     map[string]string{"SERVICE_TEST_VAR": "service_test_value"},
	}

	// Call ExecInService
	execStream, err := orch.ExecInService(ctx, "default", service.Name, execOpts)
	require.NoError(t, err, "ExecInService should not return an error")
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

	// Verify runner call was made to at least one of the instances
	instanceIDs := []string{instances[0].ID, instances[1].ID}
	foundCall := false
	for _, id := range instanceIDs {
		if contains(testRunner.ExecCalls, id) {
			foundCall = true
			break
		}
	}
	assert.True(t, foundCall, "Runner should have been called for exec on one of the instances")
}

// TestExecInInstance tests the ExecInInstance method
func TestExecInInstance(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service directly in store
	service := &types.Service{
		ID:        "exec-test-service",
		Name:      "exec-test-service",
		Namespace: "default",
		Runtime:   "container",
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create test instance directly in store
	instance := &types.Instance{
		ID:        "exec-test-instance",
		Name:      "exec-test-instance",
		Namespace: "default",
		ServiceID: service.Name,
		Status:    types.InstanceStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance.Name, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Add instance to runner's tracking
	err = testRunner.Create(ctx, instance)
	require.NoError(t, err, "Failed to add instance to runner")

	// Set up test output in runner
	testStdoutContent := []byte("instance exec output")
	testStderrContent := []byte("instance exec error output")
	testExitCode := 0

	// Configure test runner
	testRunner.ExecOutput = testStdoutContent
	testRunner.ExecErrOutput = testStderrContent
	testRunner.ExitCodeVal = testExitCode

	// Test command to execute
	command := []string{"cat", "/etc/passwd"}

	// Create exec options
	execOpts := ExecOptions{
		Command: command,
		Env:     map[string]string{"INSTANCE_TEST_VAR": "instance_test_value"},
	}

	// Call ExecInInstance
	execStream, err := orch.ExecInInstance(ctx, "default", service.Name, instance.ID, execOpts)
	require.NoError(t, err, "ExecInInstance should not return an error")
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

	// Verify runner call was made to the specific instance
	assert.Contains(t, testRunner.ExecCalls, instance.ID, "Runner should have been called for exec on the specific instance")
}

// TestExecInInstanceWrongService tests that ExecInInstance fails if instance doesn't belong to service
func TestExecInInstanceWrongService(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create two services directly in store
	service1 := &types.Service{
		ID:        "exec-wrong-service-1",
		Name:      "exec-wrong-service-1",
		Namespace: "default",
		Runtime:   "container",
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service1.Name, service1)
	require.NoError(t, err, "Failed to create test service 1")

	service2 := &types.Service{
		ID:        "exec-wrong-service-2",
		Name:      "exec-wrong-service-2",
		Namespace: "default",
		Runtime:   "container",
	}
	err = testStore.Create(ctx, types.ResourceTypeService, "default", service2.Name, service2)
	require.NoError(t, err, "Failed to create test service 2")

	// Create instance for service1
	instance1 := &types.Instance{
		ID:        "exec-wrong-service-instance",
		Name:      "exec-wrong-service-instance",
		Namespace: "default",
		ServiceID: service1.Name, // Note: belongs to service1
		Status:    types.InstanceStatusRunning,
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance1.Name, instance1)
	require.NoError(t, err, "Failed to create test instance")

	// Add instance to runner's tracking
	err = testRunner.Create(ctx, instance1)
	require.NoError(t, err, "Failed to add instance to runner")

	// Create exec options
	execOpts := ExecOptions{
		Command: []string{"echo", "hello"},
	}

	// Try to exec in instance from service1 but via service2
	_, err = orch.ExecInInstance(ctx, "default", service2.Name, instance1.ID, execOpts)
	assert.Error(t, err, "ExecInInstance should return an error if instance doesn't belong to service")
	assert.Contains(t, err.Error(), "does not belong to service", "Error message should mention ownership")
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

// TestRestartService tests the RestartService method
func TestRestartService(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service directly in store
	service := &types.Service{
		ID:            "service-restart-test",
		Name:          "service-restart-test",
		Namespace:     "default",
		Runtime:       "container",
		RestartPolicy: types.RestartPolicyAlways,
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create test instances directly in store
	instances := make([]*types.Instance, 2)

	// First instance
	instances[0] = &types.Instance{
		ID:        "service-restart-instance-1",
		Name:      "service-restart-instance-1",
		Namespace: "default",
		ServiceID: service.Name,
		Status:    types.InstanceStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instances[0].Name, instances[0])
	require.NoError(t, err, "Failed to create test instance 1")

	// Second instance
	instances[1] = &types.Instance{
		ID:        "service-restart-instance-2",
		Name:      "service-restart-instance-2",
		Namespace: "default",
		ServiceID: service.Name,
		Status:    types.InstanceStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instances[1].Name, instances[1])
	require.NoError(t, err, "Failed to create test instance 2")

	// Add instances to runner's tracking
	for _, instance := range instances {
		err = testRunner.Create(ctx, instance)
		require.NoError(t, err, "Failed to add instance to runner")
	}

	// Call RestartService
	err = orch.RestartService(ctx, "default", service.Name)
	require.NoError(t, err, "RestartService should not return an error")

	// Verify that all instances were restarted
	for _, instance := range instances {
		assert.Contains(t, testRunner.StoppedInstances, instance.ID,
			"Instance %s should have been stopped", instance.ID)
		assert.Contains(t, testRunner.CreatedInstances, instance,
			"Instance %s should have been created", instance.ID)
		assert.Contains(t, testRunner.StartedInstances, instance.ID,
			"Instance %s should have been started", instance.ID)
	}
}

// TestRestartNonExistentService tests that RestartService fails for non-existent service
func TestRestartNonExistentService(t *testing.T) {
	ctx, testStore, _, orch := setupTestOrchestrator(t)

	// First create a dummy service to ensure the namespace exists
	dummyService := &types.Service{
		ID:        "dummy-service",
		Name:      "dummy-service",
		Namespace: "default",
	}
	err := testStore.Create(ctx, "services", "default", "dummy-service", dummyService)
	assert.NoError(t, err, "Failed to create dummy service")

	// Call RestartService with non-existent service
	err = orch.RestartService(ctx, "default", "non-existent-service")
	assert.Error(t, err, "RestartService should return an error for non-existent service")
	// The actual error message might vary, just check it's an error
}

// TestOrchestratorRestartInstance tests the RestartInstance method of Orchestrator
func TestOrchestratorRestartInstance(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service directly in store
	service := &types.Service{
		ID:            "restart-test-service",
		Name:          "restart-test-service",
		Namespace:     "default",
		Runtime:       "container",
		RestartPolicy: types.RestartPolicyAlways,
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create test instance directly in store
	instance := &types.Instance{
		ID:        "restart-test-instance",
		Name:      "restart-test-instance",
		Namespace: "default",
		ServiceID: service.Name,
		Status:    types.InstanceStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance.Name, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Add instance to runner's tracking
	err = testRunner.Create(ctx, instance)
	require.NoError(t, err, "Failed to add instance to runner")

	// Call RestartInstance
	err = orch.RestartInstance(ctx, "default", service.Name, instance.ID)
	require.NoError(t, err, "RestartInstance should not return an error")

	// Now we can verify runner calls since we've set up the runner to track our test instance
	assert.Contains(t, testRunner.StoppedInstances, instance.ID,
		"Instance should have been stopped")
	assert.Contains(t, testRunner.CreatedInstances, instance,
		"Instance should have been created")
	assert.Contains(t, testRunner.StartedInstances, instance.ID,
		"Instance should have been started")
}

// TestRestartInstanceWrongService tests that RestartInstance fails if instance doesn't belong to service
func TestRestartInstanceWrongService(t *testing.T) {
	ctx, testStore, _, orch := setupTestOrchestrator(t)

	// Create two services directly in store
	service1 := &types.Service{
		ID:        "wrong-service-1",
		Name:      "wrong-service-1",
		Namespace: "default",
		Runtime:   "container",
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service1.Name, service1)
	require.NoError(t, err, "Failed to create test service 1")

	service2 := &types.Service{
		ID:        "wrong-service-2",
		Name:      "wrong-service-2",
		Namespace: "default",
		Runtime:   "container",
	}
	err = testStore.Create(ctx, types.ResourceTypeService, "default", service2.Name, service2)
	require.NoError(t, err, "Failed to create test service 2")

	// Create instance for service1
	instance1 := &types.Instance{
		ID:        "wrong-service-instance",
		Name:      "wrong-service-instance",
		Namespace: "default",
		ServiceID: service1.Name, // Note: belongs to service1
		Status:    types.InstanceStatusRunning,
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance1.Name, instance1)
	require.NoError(t, err, "Failed to create test instance")

	// Try to restart instance from service1 but via service2
	err = orch.RestartInstance(ctx, "default", service2.Name, instance1.ID)
	assert.Error(t, err, "RestartInstance should return an error if instance doesn't belong to service")
	assert.Contains(t, err.Error(), "does not belong to service", "Error message should mention ownership")
}
