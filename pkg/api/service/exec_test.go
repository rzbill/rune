package service

import (
	"context"
	"io"
	"testing"

	"bytes"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/orchestrator"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/manager"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
)

func TestExecServiceStreamExec(t *testing.T) {

	testStore := store.NewTestStore()
	testDockerRunner := runner.NewTestRunner()
	testProcessRunner := runner.NewTestRunner()
	mockCtx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))

	// Create a test runner manager
	testRunnerMgr := manager.NewTestRunnerManager(nil)
	testRunnerMgr.SetDockerRunner(testDockerRunner)
	testRunnerMgr.SetProcessRunner(testProcessRunner)

	// Create a fake orchestrator
	testOrchestrator := orchestrator.NewFakeOrchestrator()
	testOrchestrator.ExecStdout = []byte("sample command output")
	testOrchestrator.ExecStderr = []byte("sample error output")
	testOrchestrator.ExecExitCode = 0

	// Set up stream with initial request
	mockStream := NewMockExecServiceStream(mockCtx)

	// Set up the instance record
	instance := &types.Instance{
		ID:          "instance123",
		Namespace:   "default",
		Name:        "test-instance",
		ServiceID:   "service123",
		ServiceName: "test-service",
		ContainerID: "container123", // Has container ID, so we'll use Docker runner
		Status:      types.InstanceStatusRunning,
	}

	// Add the instance to the TestStore
	err := testStore.Create(context.Background(), types.ResourceTypeInstance, "default", "instance123", instance)
	assert.NoError(t, err)

	// Add the instance to the orchestrator as well
	testOrchestrator.AddInstance(instance)

	// Configure the test runner's output and exit code
	testDockerRunner.ExecOutput = []byte("sample command output")
	testDockerRunner.ExecErrOutput = []byte("sample error output")
	testDockerRunner.ExitCodeVal = 0

	// First request will be the init request
	mockStream.On("Recv").Return(&generated.ExecRequest{
		Request: &generated.ExecRequest_Init{
			Init: &generated.ExecInitRequest{
				Target: &generated.ExecInitRequest_InstanceId{
					InstanceId: "instance123",
				},
				Command: []string{"ls", "-la"},
				Tty:     true,
			},
		},
	}, nil).Once()

	// We'll return success status
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		statusResp, ok := resp.Response.(*generated.ExecResponse_Status)
		return ok && statusResp.Status.Code == int32(0)
	})).Return(nil).Once()

	// Expect stdout response with our sample output
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		stdoutResp, ok := resp.Response.(*generated.ExecResponse_Stdout)
		return ok && bytes.Equal(stdoutResp.Stdout, testDockerRunner.ExecOutput)
	})).Return(nil).Once()

	// Expect stderr response with our sample error output
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		stderrResp, ok := resp.Response.(*generated.ExecResponse_Stderr)
		return ok && bytes.Equal(stderrResp.Stderr, testDockerRunner.ExecErrOutput)
	})).Return(nil).Once()

	// Expect exit code response
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		exitResp, ok := resp.Response.(*generated.ExecResponse_Exit)
		return ok && exitResp.Exit.Code == int32(0)
	})).Return(nil).Once()

	// Ensure the test terminates after the first Recv/Send cycle
	// Need to return nil for the *ExecRequest but with an io.EOF error
	var nilReq *generated.ExecRequest = nil
	mockStream.On("Recv").Return(nilReq, io.EOF).Once()

	// Create ExecService
	logger := log.NewLogger()
	execService := NewExecService(testStore, logger, testOrchestrator)

	// Call StreamExec
	err = execService.StreamExec(mockStream)

	// Verify the results
	assert.NoError(t, err)
	mockStream.AssertExpectations(t)

	// Verify the orchestrator received the call
	assert.Equal(t, 1, len(testOrchestrator.ExecInInstanceCalls))
	assert.Equal(t, "instance123", testOrchestrator.ExecInInstanceCalls[0].InstanceID)
}

// MockStdErrReader mocks the stderr reader
type MockStdErrReader struct {
	mock.Mock
}

func (m *MockStdErrReader) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

// TestSimpleExecService is a minimal test to verify basic functionality
func TestSimpleExecService(t *testing.T) {
	// Create a simple exec service without using the store
	testStore := store.NewTestStore()
	logger := log.NewLogger()
	testOrchestrator := orchestrator.NewFakeOrchestrator()

	execService := NewExecService(testStore, logger, testOrchestrator)

	// This is just a placeholder test to ensure our interfaces are correct
	assert.NotNil(t, execService)
}

func TestExecServiceSimple(t *testing.T) {
	// Create a simple exec service without using the store
	testStore := store.NewTestStore()
	logger := log.NewLogger()
	testOrchestrator := orchestrator.NewFakeOrchestrator()

	execService := NewExecService(testStore, logger, testOrchestrator)

	// This is just a placeholder test to ensure our Reader interface is working correctly
	assert.NotNil(t, execService)
}

// TestExecServiceWithTestRunner tests the ExecService using the TestRunner instead of mocks
func TestExecServiceWithTestRunner(t *testing.T) {
	// Create test components
	testStore := store.NewTestStore()
	logger := log.NewLogger()
	testOrchestrator := orchestrator.NewFakeOrchestrator()
	mockCtx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))
	mockStream := NewMockExecServiceStream(mockCtx)

	// Configure the orchestrator
	testOrchestrator.ExecStdout = []byte("sample command output")
	testOrchestrator.ExecStderr = []byte("sample error output")
	testOrchestrator.ExecExitCode = 0

	// Set up the instance record for the test
	instance := &types.Instance{
		ID:          "instance123",
		Namespace:   "default",
		Name:        "test-instance",
		ServiceID:   "service123",
		ServiceName: "test-service",
		ContainerID: "container123", // Has container ID, so we'll use Docker runner
		Status:      types.InstanceStatusRunning,
	}

	// Add the instance to the TestStore
	err := testStore.Create(context.Background(), types.ResourceTypeInstance, "default", "instance123", instance)
	assert.NoError(t, err)

	// Add the instance to the orchestrator
	testOrchestrator.AddInstance(instance)

	// First request will be the init request
	mockStream.On("Recv").Return(&generated.ExecRequest{
		Request: &generated.ExecRequest_Init{
			Init: &generated.ExecInitRequest{
				Target: &generated.ExecInitRequest_InstanceId{
					InstanceId: "instance123",
				},
				Command: []string{"ls", "-la"},
				Tty:     true,
			},
		},
	}, nil).Once()

	// We'll return success status
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		statusResp, ok := resp.Response.(*generated.ExecResponse_Status)
		return ok && statusResp.Status.Code == int32(0)
	})).Return(nil).Once()

	// Expect stdout response with our sample output
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		stdoutResp, ok := resp.Response.(*generated.ExecResponse_Stdout)
		return ok && bytes.Equal(stdoutResp.Stdout, testOrchestrator.ExecStdout)
	})).Return(nil).Once()

	// Expect stderr response with our sample error output
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		stderrResp, ok := resp.Response.(*generated.ExecResponse_Stderr)
		return ok && bytes.Equal(stderrResp.Stderr, testOrchestrator.ExecStderr)
	})).Return(nil).Once()

	// Expect exit code response
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		exitResp, ok := resp.Response.(*generated.ExecResponse_Exit)
		return ok && exitResp.Exit.Code == int32(0)
	})).Return(nil).Once()

	// Second receive will terminate the stream
	var nilReq *generated.ExecRequest = nil
	mockStream.On("Recv").Return(nilReq, io.EOF).Once()

	// Create ExecService using the orchestrator
	execService := NewExecService(testStore, logger, testOrchestrator)

	// Call StreamExec
	err = execService.StreamExec(mockStream)

	// Verify the results
	assert.NoError(t, err)
	mockStream.AssertExpectations(t)

	// Verify the orchestrator received the call
	assert.Equal(t, 1, len(testOrchestrator.ExecInInstanceCalls))
	assert.Equal(t, "instance123", testOrchestrator.ExecInInstanceCalls[0].InstanceID)
	assert.Equal(t, []string{"ls", "-la"}, testOrchestrator.ExecInInstanceCalls[0].Options.Command)
	assert.True(t, testOrchestrator.ExecInInstanceCalls[0].Options.TTY)
}
