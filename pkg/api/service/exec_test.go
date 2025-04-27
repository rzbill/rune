package service

import (
	"context"
	"io"
	"testing"

	"bytes"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
)

func TestExecServiceStreamExec(t *testing.T) {
	// Create mocks
	mockStore := new(store.MockStore)
	mockDockerRunner := new(runner.MockRunner)
	mockProcessRunner := new(runner.MockRunner)
	mockCtx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))

	// Set up stream with initial request
	mockStream := NewMockExecServiceStream(mockCtx)

	// Set up the instance record
	instance := &types.Instance{
		ID:          "instance123",
		Name:        "test-instance",
		ServiceID:   "service123",
		ContainerID: "container123", // Has container ID, so we'll use Docker runner
		Status:      types.InstanceStatusRunning,
	}

	// Create a mock exec stream for the Docker runner
	mockExecStream := new(runner.MockExecStream)
	mockExecStream.On("Close").Return(nil)
	mockExecStream.On("Read", mock.Anything).Return(0, io.EOF)

	// Create and set up the stderr reader
	mockStdErrReader := new(MockStdErrReader)
	mockStdErrReader.On("Read", mock.Anything).Return(0, io.EOF)
	mockExecStream.On("Stderr").Return(mockStdErrReader)

	mockExecStream.On("ExitCode").Return(0, nil)

	// Set up Docker runner to expect an Exec call
	mockDockerRunner.On("Exec", mock.Anything, "instance123", mock.AnythingOfType("runner.ExecOptions")).
		Return(mockExecStream, nil)

	// Expect instance lookup - using direct Get with namespace
	mockStore.On("Get", mock.Anything, "instances", "default", "instance123", mock.AnythingOfType("*types.Instance")).
		Run(func(args mock.Arguments) {
			// Copy the instance data to the output parameter
			instanceArg := args.Get(4).(*types.Instance)
			*instanceArg = *instance
		}).Return(nil, nil)

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
	execService := NewExecService(mockDockerRunner, mockProcessRunner, mockStore, logger)

	// Call StreamExec
	err := execService.StreamExec(mockStream)

	// Verify the results
	assert.NoError(t, err)
	mockStream.AssertExpectations(t)
	mockStore.AssertExpectations(t)
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
	execService := &ExecService{
		dockerRunner:  new(runner.MockRunner),
		processRunner: new(runner.MockRunner),
		logger:        log.NewLogger(),
	}

	// This is just a placeholder test to ensure our interfaces are correct
	assert.NotNil(t, execService)
}

func TestExecServiceSimple(t *testing.T) {
	// Create a simple exec service without using the store
	execService := &ExecService{
		dockerRunner:  new(runner.MockRunner),
		processRunner: new(runner.MockRunner),
		logger:        log.NewLogger(),
	}

	// This is just a placeholder test to ensure our Reader interface is working correctly
	assert.NotNil(t, execService)
}

// TestExecServiceWithTestRunner tests the ExecService using the TestRunner instead of mocks
func TestExecServiceWithTestRunner(t *testing.T) {
	// Create test components
	mockStore := new(store.MockStore)
	testRunner := runner.NewTestRunner()
	mockCtx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))
	mockStream := NewMockExecServiceStream(mockCtx)

	// Set up the instance record for the test
	instance := &types.Instance{
		ID:          "instance123",
		Name:        "test-instance",
		ServiceID:   "service123",
		ContainerID: "container123", // Has container ID, so we'll use Docker runner
		Status:      types.InstanceStatusRunning,
	}

	// Configure the test runner's output
	testRunner.ExecOutput = []byte("sample command output")
	testRunner.ExecErrOutput = []byte("sample error output")
	testRunner.ExitCodeVal = 0

	// Expect instance lookup with the store
	mockStore.On("Get", mock.Anything, "instances", "default", "instance123", mock.AnythingOfType("*types.Instance")).
		Run(func(args mock.Arguments) {
			// Copy the instance data to the output parameter
			instanceArg := args.Get(4).(*types.Instance)
			*instanceArg = *instance
		}).Return(nil, nil)

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
		return ok && bytes.Equal(stdoutResp.Stdout, testRunner.ExecOutput)
	})).Return(nil).Once()

	// Expect stderr response with our sample error output
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		stderrResp, ok := resp.Response.(*generated.ExecResponse_Stderr)
		return ok && bytes.Equal(stderrResp.Stderr, testRunner.ExecErrOutput)
	})).Return(nil).Once()

	// Expect exit code response
	mockStream.On("Send", mock.MatchedBy(func(resp *generated.ExecResponse) bool {
		exitResp, ok := resp.Response.(*generated.ExecResponse_Exit)
		return ok && exitResp.Exit.Code == int32(0)
	})).Return(nil).Once()

	// Second receive will terminate the stream
	var nilReq *generated.ExecRequest = nil
	mockStream.On("Recv").Return(nilReq, io.EOF).Once()

	// Create ExecService - using TestRunner for Docker and nil for processRunner
	logger := log.NewLogger()
	execService := NewExecService(testRunner, nil, mockStore, logger)

	// Call StreamExec
	err := execService.StreamExec(mockStream)

	// Verify the results
	assert.NoError(t, err)
	mockStream.AssertExpectations(t)
	mockStore.AssertExpectations(t)

	// Verify that the TestRunner recorded the correct calls
	assert.Equal(t, 1, len(testRunner.ExecCalls))
	assert.Equal(t, "instance123", testRunner.ExecCalls[0])

	// Verify exec options were captured
	assert.Equal(t, 1, len(testRunner.ExecOptions))
	assert.Equal(t, []string{"ls", "-la"}, testRunner.ExecOptions[0].Command)
	assert.True(t, testRunner.ExecOptions[0].TTY)
}
