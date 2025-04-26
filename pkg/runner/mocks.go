package runner

import (
	"context"
	"io"
	"time"

	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/mock"
)

// MockRunner is a mock implementation of the runner.Runner interface for testing
type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Create(ctx context.Context, instance *types.Instance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}

func (m *MockRunner) Start(ctx context.Context, instanceID string) error {
	args := m.Called(ctx, instanceID)
	return args.Error(0)
}

func (m *MockRunner) Stop(ctx context.Context, instanceID string, timeout time.Duration) error {
	args := m.Called(ctx, instanceID, timeout)
	return args.Error(0)
}

func (m *MockRunner) Remove(ctx context.Context, instanceID string, force bool) error {
	args := m.Called(ctx, instanceID, force)
	return args.Error(0)
}

func (m *MockRunner) GetLogs(ctx context.Context, instanceID string, options LogOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, instanceID, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockRunner) Status(ctx context.Context, instanceID string) (types.InstanceStatus, error) {
	args := m.Called(ctx, instanceID)
	return args.Get(0).(types.InstanceStatus), args.Error(1)
}

func (m *MockRunner) List(ctx context.Context) ([]*types.Instance, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*types.Instance), args.Error(1)
}

func (m *MockRunner) Exec(ctx context.Context, instanceID string, options ExecOptions) (ExecStream, error) {
	args := m.Called(ctx, instanceID, options)
	return args.Get(0).(ExecStream), args.Error(1)
}

// MockExecStream is a mock implementation of the runner.ExecStream interface
type MockExecStream struct {
	mock.Mock
}

func (m *MockExecStream) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockExecStream) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockExecStream) Stderr() io.Reader {
	args := m.Called()
	return args.Get(0).(io.Reader)
}

func (m *MockExecStream) ResizeTerminal(width, height uint32) error {
	args := m.Called(width, height)
	return args.Error(0)
}

func (m *MockExecStream) Signal(sigName string) error {
	args := m.Called(sigName)
	return args.Error(0)
}

func (m *MockExecStream) ExitCode() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *MockExecStream) Close() error {
	args := m.Called()
	return args.Error(0)
}
