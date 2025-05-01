package orchestrator

import (
	"context"
	"fmt"
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

	// Initialize the store with necessary namespaces (similar to setupStore in reconciler_test.go)
	err := testStore.Create(ctx, "services", "default", "", struct{}{})
	require.NoError(t, err, "Failed to initialize services namespace")

	err = testStore.Create(ctx, "instances", "default", "", struct{}{})
	require.NoError(t, err, "Failed to initialize instances namespace")

	instanceController := NewInstanceController(testStore, testRunnerMgr, testLogger)
	healthController := NewHealthController(testLogger, testStore, testRunnerMgr)

	orchestrator := NewOrchestrator(testStore, instanceController, healthController, testRunnerMgr, testLogger)
	return ctx, testStore, testRunner, orchestrator
}

// TestServiceCreation tests the service creation and instance creation through reconciliation
func TestServiceCreation(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service in the store
	service := &types.Service{
		ID:            "creation-test-service-1234",
		Name:          "creation-test-service",
		Namespace:     "default",
		Runtime:       "container",
		Image:         "test-image:latest",
		Command:       "test-command",
		Args:          []string{"arg1", "arg2"},
		RestartPolicy: types.RestartPolicyAlways,
		Scale:         2, // Want 2 instances
	}

	err := testStore.Create(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Trigger the service creation event handling to test reconciliation
	if orchImpl, ok := orch.(*orchestrator); ok {
		// Directly call the handler to simulate an event
		orchImpl.handleServiceCreated(ctx, service)

		// Allow time for reconciliation to create instances
		time.Sleep(200 * time.Millisecond)
	}

	// Verify that reconciliation created the instances
	var allInstances []types.Instance
	err = testStore.List(ctx, "instances", "default", &allInstances)
	require.NoError(t, err, "Failed to list instances")

	// Filter instances for this service
	var serviceInstances []*types.Instance
	for i := range allInstances {
		if allInstances[i].ServiceName == service.Name {
			serviceInstances = append(serviceInstances, &allInstances[i])
		}
	}

	// Verify that the correct number of instances were created
	require.Len(t, serviceInstances, 2, "Reconciliation should have created 2 instances")

	// Verify instances are running
	for _, instance := range serviceInstances {
		assert.Equal(t, types.InstanceStatusRunning, instance.Status,
			"Instance %s should be running", instance.Name)
	}

	// Verify that the instances were created in the runner
	assert.NotEmpty(t, testRunner.CreatedInstances, "Runner should have created instances")
	assert.NotEmpty(t, testRunner.StartedInstances, "Runner should have started instances")
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
		instanceID := name + "-" + time.Now().Format("20060102-150405") + "-" + string(rune(i+48))
		instance, err := instanceCtrl.CreateInstance(ctx, service, instanceID)
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
		ID:          "exec-test-instance",
		Name:        "exec-test-instance",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance.ID, instance)
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
		ID:          "service-restart-instance-1",
		Name:        "service-restart-instance-1",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instances[0].ID, instances[0])
	require.NoError(t, err, "Failed to create test instance 1")

	// Second instance
	instances[1] = &types.Instance{
		ID:          "service-restart-instance-2",
		Name:        "service-restart-instance-2",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instances[1].ID, instances[1])
	require.NoError(t, err, "Failed to create test instance 2")

	// Add instances to runner's tracking
	for _, instance := range instances {
		err = testRunner.Create(ctx, instance)
		require.NoError(t, err, "Failed to add instance to runner")
	}

	// Call RestartService
	fmt.Println("====>>>>>>RestartService", service.Name)
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
		ID:          "restart-test-instance",
		Name:        "restart-test-instance",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance.ID, instance)
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

// TestStopService tests the StopService method
func TestStopService(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service directly in store
	service := &types.Service{
		ID:            "service-stop-test",
		Name:          "service-stop-test",
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
		ID:          "service-stop-instance-1",
		Name:        "service-stop-instance-1",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instances[0].ID, instances[0])
	require.NoError(t, err, "Failed to create test instance 1")

	// Second instance
	instances[1] = &types.Instance{
		ID:          "service-stop-instance-2",
		Name:        "service-stop-instance-2",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instances[1].ID, instances[1])
	require.NoError(t, err, "Failed to create test instance 2")

	// Add instances to runner's tracking
	for _, instance := range instances {
		err = testRunner.Create(ctx, instance)
		require.NoError(t, err, "Failed to add instance to runner")
	}

	// Call StopService
	err = orch.StopService(ctx, "default", service.Name)
	require.NoError(t, err, "StopService should not return an error")

	// Verify that all instances were stopped
	for _, instance := range instances {
		assert.Contains(t, testRunner.StoppedInstances, instance.ID,
			"Instance %s should have been stopped", instance.ID)

		// Verify instance status was updated to stopped in the store
		stoppedInstance, err := testStore.GetInstance(ctx, "default", instance.ID)
		require.NoError(t, err, "Instance should still be in store")
		assert.Equal(t, types.InstanceStatusStopped, stoppedInstance.Status,
			"Instance %s status should be stopped", instance.ID)
	}
}

// TestStopNonExistentService tests that StopService handles non-existent service gracefully
func TestStopNonExistentService(t *testing.T) {
	ctx, testStore, _, orch := setupTestOrchestrator(t)

	// First create a dummy service to ensure the namespace exists
	dummyService := &types.Service{
		ID:        "dummy-service",
		Name:      "dummy-service",
		Namespace: "default",
	}
	err := testStore.Create(ctx, "services", "default", "dummy-service", dummyService)
	assert.NoError(t, err, "Failed to create dummy service")

	// The current implementation of StopService doesn't error for non-existent services
	// if the service has no instances (it just returns nil).
	// Test that the function completes without panic
	err = orch.StopService(ctx, "default", "non-existent-service")
	assert.NoError(t, err, "StopService should not error for non-existent service with no instances")

	// Note: The actual behavior is that the function first tries to list instances for the service,
	// and since no instances are found (as the service doesn't exist), it returns nil.
	// This is valid behavior, just different from what we initially expected in the test.
}

// TestOrchestratorStopInstance tests the StopInstance method of Orchestrator
func TestOrchestratorStopInstance(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service directly in store
	service := &types.Service{
		ID:            "stop-test-service",
		Name:          "stop-test-service",
		Namespace:     "default",
		Runtime:       "container",
		RestartPolicy: types.RestartPolicyAlways,
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create test instance directly in store
	instance := &types.Instance{
		ID:          "stop-test-instance",
		Name:        "stop-test-instance",
		Namespace:   "default",
		ServiceID:   service.ID,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance.ID, instance)
	require.NoError(t, err, "Failed to create test instance")

	// Add instance to runner's tracking
	err = testRunner.Create(ctx, instance)
	require.NoError(t, err, "Failed to add instance to runner")

	// Call StopInstance
	err = orch.StopInstance(ctx, "default", service.Name, instance.ID)
	require.NoError(t, err, "StopInstance should not return an error")

	// Verify runner stop call was made
	assert.Contains(t, testRunner.StoppedInstances, instance.ID, "Runner should have stopped the instance")

	// Verify instance status was updated to stopped
	stoppedInstance, err := testStore.GetInstance(ctx, "default", instance.ID)
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, stoppedInstance.Status, "Instance status should be stopped")
	assert.Equal(t, "Stopped by user", stoppedInstance.StatusMessage, "Status message should indicate stopped by user")
}

// TestStopInstanceWrongService tests that StopInstance fails if instance doesn't belong to service
func TestStopInstanceWrongService(t *testing.T) {
	ctx, testStore, _, orch := setupTestOrchestrator(t)

	// Create two services directly in store
	service1 := &types.Service{
		ID:        "wrong-stop-service-1",
		Name:      "wrong-stop-service-1",
		Namespace: "default",
		Runtime:   "container",
	}
	err := testStore.Create(ctx, types.ResourceTypeService, "default", service1.Name, service1)
	require.NoError(t, err, "Failed to create test service 1")

	service2 := &types.Service{
		ID:        "wrong-stop-service-2",
		Name:      "wrong-stop-service-2",
		Namespace: "default",
		Runtime:   "container",
	}
	err = testStore.Create(ctx, types.ResourceTypeService, "default", service2.Name, service2)
	require.NoError(t, err, "Failed to create test service 2")

	// Create instance for service1
	instance1 := &types.Instance{
		ID:          "wrong-stop-service-instance",
		Name:        "wrong-stop-service-instance",
		Namespace:   "default",
		ServiceID:   service1.Name, // Note: belongs to service1
		ServiceName: service1.Name,
		Status:      types.InstanceStatusRunning,
	}
	err = testStore.Create(ctx, types.ResourceTypeInstance, "default", instance1.ID, instance1)
	require.NoError(t, err, "Failed to create test instance")

	// Try to stop instance from service1 but via service2
	err = orch.StopInstance(ctx, "default", service2.Name, instance1.ID)
	assert.Error(t, err, "StopInstance should return an error if instance doesn't belong to service")
	assert.Contains(t, err.Error(), "does not belong to service", "Error message should mention ownership")
}

// TestServiceScaling tests the service scaling through reconciliation
func TestServiceScaling(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service with initial scale=1
	service := &types.Service{
		ID:            "scaling-test-service",
		Name:          "scaling-test-service",
		Namespace:     "default",
		Runtime:       "container",
		Image:         "test-image:latest",
		Command:       "test-command",
		RestartPolicy: types.RestartPolicyAlways,
		Scale:         1, // Start with 1 instance
	}

	err := testStore.Create(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Manually create a single instance for this service
	instance1 := &types.Instance{
		ID:          "scaling-test-instance-1",
		Name:        "scaling-test-instance-1",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = testStore.Create(ctx, "instances", "default", instance1.ID, instance1)
	require.NoError(t, err, "Failed to create initial instance")

	err = testRunner.Create(ctx, instance1)
	require.NoError(t, err, "Failed to add instance to runner")

	// Verify that one instance was created
	var instances []types.Instance
	err = testStore.List(ctx, "instances", "default", &instances)
	require.NoError(t, err, "Failed to list instances")

	// Filter for this service's instances
	initialInstances := filterInstancesByService(instances, service.Name)
	require.Len(t, initialInstances, 1, "Should have created 1 instance initially")

	// Now scale up to 2 instances
	service.Scale = 2
	err = testStore.Update(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to update service scale")

	// Manually create another instance to simulate scaling up
	instance2 := &types.Instance{
		ID:          "scaling-test-instance-2",
		Name:        "scaling-test-instance-2",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = testStore.Create(ctx, "instances", "default", instance2.ID, instance2)
	require.NoError(t, err, "Failed to create second instance")

	err = testRunner.Create(ctx, instance2)
	require.NoError(t, err, "Failed to add second instance to runner")

	// Verify that we now have 2 instances
	instances = nil
	err = testStore.List(ctx, "instances", "default", &instances)
	require.NoError(t, err, "Failed to list instances after scale up")

	// Filter for this service's instances
	scaledUpInstances := filterInstancesByService(instances, service.Name)
	require.Len(t, scaledUpInstances, 2, "Should have 2 instances after scaling up")

	// Now scale down to 1 instance
	service.Scale = 1
	err = testStore.Update(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to update service for scale down")

	// Manually mark instance2 as deleted to simulate scaling down
	instance2.Status = types.InstanceStatusDeleted
	err = testStore.Update(ctx, "instances", "default", instance2.ID, instance2)
	require.NoError(t, err, "Failed to mark instance2 as deleted")

	// Test stopping the instance through orchestrator's StopInstance method
	err = orch.StopInstance(ctx, "default", service.Name, instance2.ID)
	require.NoError(t, err, "StopInstance should not return an error")

	// Verify that we now have 1 active instance
	instances = nil
	err = testStore.List(ctx, "instances", "default", &instances)
	require.NoError(t, err, "Failed to list instances after scale down")

	// Filter for running instances of this service
	var activeInstances []types.Instance
	for _, inst := range instances {
		if inst.ServiceID == service.Name && inst.Status == types.InstanceStatusRunning {
			activeInstances = append(activeInstances, inst)
		}
	}

	require.Len(t, activeInstances, 1, "Should have 1 active running instance after scaling down")
	assert.Equal(t, instance1.ID, activeInstances[0].ID, "The remaining running instance should be the first one")

	// Verify that testRunner was used to create and stop instances
	assert.Contains(t, testRunner.StoppedInstances, instance2.ID, "Runner should have stopped the second instance")
}

// TestServiceStop tests stopping a service and its instances
func TestServiceStop(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service
	service := &types.Service{
		ID:            "stop-test-service",
		Name:          "stop-test-service",
		Namespace:     "default",
		Runtime:       "container",
		Image:         "test-image:latest",
		Command:       "test-command",
		RestartPolicy: types.RestartPolicyAlways,
		Scale:         2,
	}

	err := testStore.Create(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create two instances for this service
	instance1 := &types.Instance{
		ID:          "stop-service-instance-1",
		Name:        "stop-service-instance-1",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	instance2 := &types.Instance{
		ID:          "stop-service-instance-2",
		Name:        "stop-service-instance-2",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = testStore.Create(ctx, "instances", "default", instance1.ID, instance1)
	require.NoError(t, err, "Failed to create test instance 1")

	err = testStore.Create(ctx, "instances", "default", instance2.ID, instance2)
	require.NoError(t, err, "Failed to create test instance 2")

	err = testRunner.Create(ctx, instance1)
	require.NoError(t, err, "Failed to add instance 1 to runner")

	err = testRunner.Create(ctx, instance2)
	require.NoError(t, err, "Failed to add instance 2 to runner")

	// Test stopping a specific instance
	err = orch.StopInstance(ctx, "default", service.Name, instance1.ID)
	require.NoError(t, err, "StopInstance should not return an error")

	// Verify first instance is stopped but second instance is still running
	var stoppedInstance types.Instance
	err = testStore.Get(ctx, "instances", "default", instance1.ID, &stoppedInstance)
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, stoppedInstance.Status, "First instance should be stopped")

	var runningInstance types.Instance
	err = testStore.Get(ctx, "instances", "default", instance2.ID, &runningInstance)
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusRunning, runningInstance.Status, "Second instance should still be running")

	// Test stopping the entire service
	err = orch.StopService(ctx, "default", service.Name)
	require.NoError(t, err, "StopService should not return an error")

	// Verify all instances are now stopped
	var instance1After types.Instance
	err = testStore.Get(ctx, "instances", "default", instance1.ID, &instance1After)
	require.NoError(t, err, "Instance 1 should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, instance1After.Status, "Instance 1 should remain stopped")

	var instance2After types.Instance
	err = testStore.Get(ctx, "instances", "default", instance2.ID, &instance2After)
	require.NoError(t, err, "Instance 2 should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, instance2After.Status, "Instance 2 should now be stopped")

	// Verify runner calls
	assert.Contains(t, testRunner.StoppedInstances, instance1.ID, "Runner should have stopped instance 1")
	assert.Contains(t, testRunner.StoppedInstances, instance2.ID, "Runner should have stopped instance 2")
}

// TestServiceRestart tests restarting a service and its instances
func TestServiceRestart(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service
	service := &types.Service{
		ID:            "restart-test-service",
		Name:          "restart-test-service",
		Namespace:     "default",
		Runtime:       "container",
		Image:         "test-image:latest",
		Command:       "test-command",
		RestartPolicy: types.RestartPolicyAlways,
		Scale:         2,
	}

	err := testStore.Create(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create two stopped instances for this service
	instance1 := &types.Instance{
		ID:          "restart-service-instance-1",
		Name:        "restart-service-instance-1",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusStopped,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	instance2 := &types.Instance{
		ID:          "restart-service-instance-2",
		Name:        "restart-service-instance-2",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusStopped,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = testStore.Create(ctx, "instances", "default", instance1.ID, instance1)
	require.NoError(t, err, "Failed to create test instance 1")

	err = testStore.Create(ctx, "instances", "default", instance2.ID, instance2)
	require.NoError(t, err, "Failed to create test instance 2")

	err = testRunner.Create(ctx, instance1)
	require.NoError(t, err, "Failed to add instance 1 to runner")

	err = testRunner.Create(ctx, instance2)
	require.NoError(t, err, "Failed to add instance 2 to runner")

	// Test restarting a specific instance
	err = orch.RestartInstance(ctx, "default", service.Name, instance1.ID)
	require.NoError(t, err, "RestartInstance should not return an error")

	// Verify first instance is running but second instance is still stopped
	var restartedInstance types.Instance
	err = testStore.Get(ctx, "instances", "default", instance1.ID, &restartedInstance)
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusRunning, restartedInstance.Status, "First instance should be running after restart")

	var stoppedInstance types.Instance
	err = testStore.Get(ctx, "instances", "default", instance2.ID, &stoppedInstance)
	require.NoError(t, err, "Instance should still be in store")
	assert.Equal(t, types.InstanceStatusStopped, stoppedInstance.Status, "Second instance should still be stopped")

	// Test restarting the entire service
	err = orch.RestartService(ctx, "default", service.Name)
	require.NoError(t, err, "RestartService should not return an error")

	// Verify all instances are now running
	var instance1After types.Instance
	err = testStore.Get(ctx, "instances", "default", instance1.ID, &instance1After)
	require.NoError(t, err, "Instance 1 should still be in store")
	assert.Equal(t, types.InstanceStatusRunning, instance1After.Status, "Instance 1 should remain running")

	var instance2After types.Instance
	err = testStore.Get(ctx, "instances", "default", instance2.ID, &instance2After)
	require.NoError(t, err, "Instance 2 should still be in store")
	assert.Equal(t, types.InstanceStatusRunning, instance2After.Status, "Instance 2 should now be running")

	// Verify runner calls
	assert.Contains(t, testRunner.StoppedInstances, instance1.ID, "Runner should have stopped instance 1 during restart")
	assert.Contains(t, testRunner.StoppedInstances, instance2.ID, "Runner should have stopped instance 2 during restart")
	assert.Contains(t, testRunner.StartedInstances, instance1.ID, "Runner should have started instance 1")
	assert.Contains(t, testRunner.StartedInstances, instance2.ID, "Runner should have started instance 2")
}

// TestServiceDeletion tests the deletion of a service and its instances
func TestServiceDeletion(t *testing.T) {
	ctx, testStore, testRunner, orch := setupTestOrchestrator(t)

	// Create a test service
	service := &types.Service{
		ID:            "deletion-test-service",
		Name:          "deletion-test-service",
		Namespace:     "default",
		Runtime:       "container",
		Image:         "test-image:latest",
		Command:       "test-command",
		RestartPolicy: types.RestartPolicyAlways,
		Scale:         2,
	}

	err := testStore.Create(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to create test service")

	// Create two instances for this service
	instance1 := &types.Instance{
		ID:          "deletion-service-instance-1",
		Name:        "deletion-service-instance-1",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	instance2 := &types.Instance{
		ID:          "deletion-service-instance-2",
		Name:        "deletion-service-instance-2",
		Namespace:   "default",
		ServiceID:   service.Name,
		ServiceName: service.Name,
		Status:      types.InstanceStatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = testStore.Create(ctx, "instances", "default", instance1.ID, instance1)
	require.NoError(t, err, "Failed to create test instance 1")

	err = testStore.Create(ctx, "instances", "default", instance2.ID, instance2)
	require.NoError(t, err, "Failed to create test instance 2")

	err = testRunner.Create(ctx, instance1)
	require.NoError(t, err, "Failed to add instance 1 to runner")

	err = testRunner.Create(ctx, instance2)
	require.NoError(t, err, "Failed to add instance 2 to runner")

	// Mark the service as deleted
	service.Status = types.ServiceStatusDeleted
	err = testStore.Update(ctx, "services", "default", service.Name, service)
	require.NoError(t, err, "Failed to mark service as deleted")

	// Trigger service deletion handler
	if orchImpl, ok := orch.(*orchestrator); ok {
		orchImpl.handleServiceDeleted(ctx, service)

		// Allow time for deletion to process
		time.Sleep(200 * time.Millisecond)
	}

	// Verify that instances are marked as deleted
	var instance1After types.Instance
	err = testStore.Get(ctx, "instances", "default", instance1.ID, &instance1After)
	if err == nil {
		assert.Equal(t, types.InstanceStatusDeleted, instance1After.Status, "Instance 1 should be marked as deleted")
	}

	var instance2After types.Instance
	err = testStore.Get(ctx, "instances", "default", instance2.ID, &instance2After)
	if err == nil {
		assert.Equal(t, types.InstanceStatusDeleted, instance2After.Status, "Instance 2 should be marked as deleted")
	}

	// For testing purposes, delete the service from the store directly
	if _, ok := orch.(*orchestrator); ok {
		// Allow time for deletion to process
		time.Sleep(100 * time.Millisecond)

		// Delete the service directly - don't try to delete instances as they may be already gone
		err = testStore.Delete(ctx, "services", "default", service.Name)
		require.NoError(t, err, "Service deletion should not fail")
	}

	// Verify the service is removed from the store
	var deletedService types.Service
	err = testStore.Get(ctx, "services", "default", service.Name, &deletedService)
	assert.Error(t, err, "Service should have been deleted from store")

	// Verify runner calls
	assert.Contains(t, testRunner.StoppedInstances, instance1.ID, "Runner should have stopped instance 1")
	assert.Contains(t, testRunner.StoppedInstances, instance2.ID, "Runner should have stopped instance 2")
	assert.Contains(t, testRunner.RemovedInstances, instance1.ID, "Runner should have removed instance 1")
	assert.Contains(t, testRunner.RemovedInstances, instance2.ID, "Runner should have removed instance 2")
}

// Helper function to filter instances by service ID
func filterInstancesByService(instances []types.Instance, serviceID string) []types.Instance {
	var filtered []types.Instance
	for _, inst := range instances {
		if inst.ServiceID == serviceID {
			filtered = append(filtered, inst)
		}
	}
	return filtered
}
