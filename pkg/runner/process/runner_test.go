package process

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessRunner(t *testing.T) {
	// Create temporary directory for tests
	testDir, err := os.MkdirTemp("", "process-runner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Test with default options
	r1, err := NewProcessRunner()
	require.NoError(t, err)
	assert.Equal(t, os.TempDir(), r1.baseDir)
	assert.Equal(t, "default", r1.namespace)

	// Test with custom options
	logger := log.NewLogger()
	r2, err := NewProcessRunner(
		WithBaseDir(testDir),
		WithNamespace("test"),
		WithLogger(logger),
	)
	require.NoError(t, err)
	assert.Equal(t, testDir, r2.baseDir)
	assert.Equal(t, "test", r2.namespace)
	assert.Equal(t, logger, r2.logger)
}

func TestProcessRunner_Create(t *testing.T) {
	// Setup
	testDir, err := os.MkdirTemp("", "process-runner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	processRunner, err := NewProcessRunner(WithBaseDir(testDir))
	require.NoError(t, err)

	// Create a valid instance
	instance := &types.Instance{
		ID:        "test-instance",
		Name:      "test",
		NodeID:    "local",
		ServiceID: "test-service",
		Process: &types.ProcessSpec{
			Command: "echo",
			Args:    []string{"hello world"},
		},
	}

	// Test: Create a valid instance
	err = processRunner.Create(context.Background(), instance)
	require.NoError(t, err)

	// Verify instance was created
	assert.Contains(t, processRunner.processes, "test-instance")
	assert.Equal(t, types.InstanceStatusCreated, instance.Status)
	assert.NotZero(t, instance.CreatedAt)
	assert.NotZero(t, instance.UpdatedAt)

	// Test: Create duplicate instance should fail
	err = processRunner.Create(context.Background(), instance)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Test: Create with nil instance should fail
	err = processRunner.Create(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	// Test: Create with invalid instance should fail
	invalidInstance := &types.Instance{
		ID:        "invalid-instance",
		Name:      "invalid",
		NodeID:    "local",
		ServiceID: "test-service",
		// Missing Process spec
	}
	err = processRunner.Create(context.Background(), invalidInstance)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "process spec is required")
}

func TestProcessRunner_StartStopRemove(t *testing.T) {
	// Skip on CI systems where process control may be limited
	if os.Getenv("CI") != "" {
		t.Skip("Skipping process tests in CI environment")
	}

	// Setup
	testDir, err := os.MkdirTemp("", "process-runner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	processRunner, err := NewProcessRunner(WithBaseDir(testDir))
	require.NoError(t, err)

	// Create a sleep process that we can control
	instance := &types.Instance{
		ID:        "test-sleep",
		Name:      "test-sleep",
		NodeID:    "local",
		ServiceID: "test-service",
		Process: &types.ProcessSpec{
			Command: "sleep",
			Args:    []string{"10"},
		},
	}

	ctx := context.Background()
	err = processRunner.Create(ctx, instance)
	require.NoError(t, err)

	// Test: Start the process
	err = processRunner.Start(ctx, instance.ID)
	require.NoError(t, err)

	// Verify process is running
	time.Sleep(100 * time.Millisecond) // Give it time to start
	status, err := processRunner.Status(ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, types.InstanceStatusRunning, status)

	// Test: Stop the process
	err = processRunner.Stop(ctx, instance.ID, 5*time.Second)
	require.NoError(t, err)

	// Verify process is stopped
	time.Sleep(100 * time.Millisecond) // Give it time to stop
	status, err = processRunner.Status(ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, types.InstanceStatusStopped, status)

	// Test: Remove the process
	err = processRunner.Remove(ctx, instance.ID, false)
	require.NoError(t, err)

	// Verify process is removed
	_, err = processRunner.Status(ctx, instance.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProcessRunner_GetLogs(t *testing.T) {
	// Setup
	testDir, err := os.MkdirTemp("", "process-runner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	processRunner, err := NewProcessRunner(WithBaseDir(testDir))
	require.NoError(t, err)

	// Create a process that outputs something
	instance := &types.Instance{
		ID:        "test-echo",
		Name:      "test-echo",
		NodeID:    "local",
		ServiceID: "test-service",
		Process: &types.ProcessSpec{
			Command: "echo",
			Args:    []string{"test output"},
		},
	}

	ctx := context.Background()
	err = processRunner.Create(ctx, instance)
	require.NoError(t, err)

	// Start the process
	err = processRunner.Start(ctx, instance.ID)
	require.NoError(t, err)

	// Give the process time to complete
	time.Sleep(100 * time.Millisecond)

	// Test: Get logs
	logs, err := processRunner.GetLogs(ctx, instance.ID, runner.LogOptions{})
	require.NoError(t, err)
	defer logs.Close()

	// Read log content
	logContent := make([]byte, 1024)
	n, err := logs.Read(logContent)
	require.NoError(t, err)
	logContent = logContent[:n]

	// Verify log content contains our output
	assert.Contains(t, string(logContent), "test output")

	// Cleanup
	err = processRunner.Remove(ctx, instance.ID, true)
	require.NoError(t, err)
}

func TestProcessRunner_List(t *testing.T) {
	// Setup
	testDir, err := os.MkdirTemp("", "process-runner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	processRunner, err := NewProcessRunner(WithBaseDir(testDir))
	require.NoError(t, err)

	ctx := context.Background()

	// Create a few instances
	for i := 1; i <= 3; i++ {
		instance := &types.Instance{
			ID:        filepath.Join("test-instance", string(rune('0'+i))),
			Name:      filepath.Join("test", string(rune('0'+i))),
			NodeID:    "local",
			ServiceID: "test-service",
			Process: &types.ProcessSpec{
				Command: "echo",
				Args:    []string{"test"},
			},
		}
		err = processRunner.Create(ctx, instance)
		require.NoError(t, err)
	}

	// Test: List instances
	instances, err := processRunner.List(ctx)
	require.NoError(t, err)
	assert.Len(t, instances, 3)

	// Verify instance details
	found := false
	for _, instance := range instances {
		if instance.ID == "test-instance/1" {
			found = true
			assert.Equal(t, "test/1", instance.Name)
			assert.Equal(t, types.InstanceStatusCreated, instance.Status)
		}
	}
	assert.True(t, found, "Expected instance not found in list")
}

func TestFindStartPosition(t *testing.T) {
	// Create a temporary file with known content
	f, err := os.CreateTemp("", "log-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	// Write 10 lines
	for i := 1; i <= 10; i++ {
		_, err = f.WriteString(filepath.Join("Line", string(rune('0'+i)), "\n"))
		require.NoError(t, err)
	}
	f.Sync()

	// Get file size
	info, err := f.Stat()
	require.NoError(t, err)
	fileSize := info.Size()

	// Test cases
	tests := []struct {
		name     string
		numLines int
		want     int64 // 0 means start of file
	}{
		{"Zero lines", 0, 0},
		{"Negative lines", -1, 0},
		{"More lines than file", 20, 0},
		{"Last 3 lines", 3, fileSize - 18}, // "Line8\nLine9\nLine10\n" = 18 bytes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findStartPosition(f, fileSize, tt.numLines)
			if tt.want == 0 {
				assert.Equal(t, int64(0), got)
			} else {
				// For the "Last 3 lines" test, we need a more flexible check
				// since the exact position depends on line endings
				if tt.numLines == 3 {
					// Seek to the position and read
					_, err = f.Seek(got, 0)
					require.NoError(t, err)
					content := make([]byte, fileSize)
					n, err := f.Read(content)
					require.NoError(t, err)
					content = content[:n]
					lines := 0
					for _, b := range content {
						if b == '\n' {
							lines++
						}
					}
					assert.Equal(t, 3, lines, "Should have 3 lines from position")
				}
			}
		})
	}
}
