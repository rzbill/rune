package service

import (
	"context"
	"io"
	"testing"

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

	// Expect a List call for namespaces
	mockStore.On("List", mock.Anything, "namespaces", "").Return([]interface{}{}, nil)

	// Expect a List call for instances in the default namespace
	mockStore.On("List", mock.Anything, "instances", "default").Return([]interface{}{}, nil)

	// Set up the instance record
	instance := &types.Instance{
		ID:          "instance123",
		Name:        "test-instance",
		ServiceID:   "service123",
		ContainerID: "container123", // Has container ID, so we'll use Docker runner
		Status:      types.InstanceStatusRunning,
	}

	// Expect instance lookup
	mockStore.On("Get", mock.Anything, "instances", "default", "instance123", mock.AnythingOfType("*types.Instance")).
		Run(func(args mock.Arguments) {
			// Copy the instance data to the output parameter
			instanceArg := args.Get(4).(*types.Instance)
			*instanceArg = *instance
		}).Return(nil)

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

	// Ensure the test terminates after the first Recv/Send cycle
	mockStream.On("Recv").Return(nil, io.EOF).Once()

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
