// Package process implements a Runner interface for local processes
package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/process/security"
	"github.com/rzbill/rune/pkg/types"
)

// Validate that ProcessRunner implements the runner.Runner interface
var _ runner.Runner = &ProcessRunner{}

// ProcessRunner implements the runner.Runner interface for local processes
type ProcessRunner struct {
	baseDir   string
	logger    log.Logger
	processes map[string]*managedProcess
	mu        sync.RWMutex
}

// managedProcess holds internal state for a process managed by ProcessRunner
type managedProcess struct {
	instance     *types.Instance
	cmd          *exec.Cmd
	status       runner.InstanceStatus
	logFile      *os.File
	workDir      string
	mu           sync.RWMutex
	stopCh       chan struct{}
	cleanupFiles []string
}

// ProcessOption is a function that configures a ProcessRunner
type ProcessOption func(*ProcessRunner)

// WithBaseDir sets the base directory for process workspaces and logs
func WithBaseDir(dir string) ProcessOption {
	return func(r *ProcessRunner) {
		r.baseDir = dir
	}
}

// WithLogger sets the logger for the runner
func WithLogger(logger log.Logger) ProcessOption {
	return func(r *ProcessRunner) {
		r.logger = logger
	}
}

// NewProcessRunner creates a new ProcessRunner with the given options
func NewProcessRunner(options ...ProcessOption) (*ProcessRunner, error) {
	// Create a default logger
	defaultLogger := log.NewLogger()

	runner := &ProcessRunner{
		baseDir:   os.TempDir(),
		logger:    defaultLogger,
		processes: make(map[string]*managedProcess),
	}

	for _, option := range options {
		option(runner)
	}

	// Ensure base directory exists
	if err := os.MkdirAll(runner.baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return runner, nil
}

// Create creates a new process but does not start it
func (r *ProcessRunner) Create(ctx context.Context, instance *types.Instance) error {
	if instance == nil {
		return fmt.Errorf("instance cannot be nil")
	}

	if instance.ID == "" {
		instance.ID = uuid.New().String()
	}

	if instance.Namespace == "" {
		instance.Namespace = instance.ServiceID
	}

	if err := instance.Validate(); err != nil {
		return fmt.Errorf("invalid instance: %w", err)
	}

	// Ensure we have process spec
	if instance.Process == nil {
		return fmt.Errorf("process spec is required")
	}

	// Use our enhanced validation that also checks executable path
	if err := ValidateProcessSpec(instance.Process); err != nil {
		return fmt.Errorf("invalid process spec: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if instance already exists
	if _, exists := r.processes[instance.ID]; exists {
		return fmt.Errorf("instance with ID %s already exists", instance.ID)
	}

	// Create workspace directory
	workDir := filepath.Join(r.baseDir, "rune", instance.Namespace, instance.ID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create log file
	logPath := filepath.Join(workDir, "process.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		// Clean up the workspace if we fail
		_ = os.RemoveAll(workDir)
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Prepare command (but don't start it)
	cmd := exec.CommandContext(ctx, instance.Process.Command, instance.Process.Args...)

	// Set environment variables
	if instance.Environment != nil && len(instance.Environment) > 0 {
		// Convert map to slice of KEY=VALUE strings
		env := os.Environ() // Include parent environment
		for key, value := range instance.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		cmd.Env = env
	}

	// Set working directory if specified
	if instance.Process.WorkingDir != "" {
		cmd.Dir = instance.Process.WorkingDir
	} else {
		cmd.Dir = workDir
	}

	// Apply security context if specified
	if instance.Process.SecurityContext != nil {
		if err := security.ApplySecurityContext(cmd, instance.Process.SecurityContext, r.logger); err != nil {
			// Clean up resources if security context fails
			_ = logFile.Close()
			_ = os.RemoveAll(workDir)
			return fmt.Errorf("failed to apply security context: %w", err)
		}
	}

	// Record creation time
	now := time.Now()

	// Create managed process entry
	proc := &managedProcess{
		instance:     instance,
		cmd:          cmd,
		workDir:      workDir,
		logFile:      logFile,
		cleanupFiles: []string{logPath},
		stopCh:       make(chan struct{}),
		status: runner.InstanceStatus{
			State:      types.InstanceStatusCreated,
			InstanceID: instance.ID,
			CreatedAt:  now,
		},
	}

	// Store the process
	r.processes[instance.ID] = proc

	// Update instance status
	instance.Status = types.InstanceStatusCreated
	instance.UpdatedAt = now
	instance.CreatedAt = now

	r.logger.Info("Created process instance",
		log.Str("instance_id", instance.ID),
		log.Str("name", instance.Name),
		log.Str("command", instance.Process.Command))

	return nil
}

// Start starts an existing process instance
func (r *ProcessRunner) Start(ctx context.Context, instance *types.Instance) error {
	r.mu.RLock()
	proc, exists := r.processes[instance.ID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance with ID %s not found", instance.ID)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// Ensure instance is in a valid state to start
	if proc.status.State != types.InstanceStatusCreated &&
		proc.status.State != types.InstanceStatusStopped {
		return fmt.Errorf("instance in state %s cannot be started", proc.status.State)
	}

	// Set up command stdout/stderr redirection to log file
	proc.cmd.Stdout = proc.logFile
	proc.cmd.Stderr = proc.logFile

	// Start the command
	if err := proc.cmd.Start(); err != nil {
		proc.status.State = types.InstanceStatusFailed
		proc.status.ErrorMessage = err.Error()
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Update status
	now := time.Now()
	proc.status.State = types.InstanceStatusRunning
	proc.status.StartedAt = now

	// Update instance status
	proc.instance.Status = types.InstanceStatusRunning
	proc.instance.UpdatedAt = now

	// Monitor the process completion to update status
	go func() {
		// Wait for process to complete
		err := proc.cmd.Wait()

		proc.mu.Lock()
		defer proc.mu.Unlock()

		// Get exit code and update status
		exitCode := 0
		if err != nil {
			// Check if it's an exit error
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()
				}
			}
			proc.status.ErrorMessage = err.Error()
		}

		now := time.Now()
		proc.status.State = types.InstanceStatusStopped
		proc.status.ExitCode = exitCode
		proc.status.FinishedAt = now

		// Update instance
		proc.instance.Status = types.InstanceStatusStopped
		proc.instance.UpdatedAt = now

		r.logger.Info("Process completed",
			log.Str("instance_id", proc.instance.ID),
			log.Str("name", proc.instance.Name),
			log.Int("exit_code", exitCode))
	}()

	r.logger.Info("Started process instance",
		log.Str("instance_id", proc.instance.ID),
		log.Str("name", proc.instance.Name),
		log.Int("pid", proc.cmd.Process.Pid))

	return nil
}

// Stop stops a running process instance
func (r *ProcessRunner) Stop(ctx context.Context, instance *types.Instance, timeout time.Duration) error {
	r.mu.RLock()
	proc, exists := r.processes[instance.ID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance with ID %s not found", instance.ID)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// Check if the process is running
	if proc.status.State != types.InstanceStatusRunning {
		return nil // Already stopped
	}

	if proc.cmd.Process == nil {
		return fmt.Errorf("process is not running")
	}

	// Try graceful shutdown first (SIGTERM)
	if err := proc.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		r.logger.Warn("Failed to send SIGTERM to process",
			log.Str("instance_id", proc.instance.ID),
			log.Err(err))
	}

	// Set up timer for timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Channel to indicate process has exited
	done := make(chan error, 1)
	go func() {
		// Wait for the process to exit
		_, err := proc.cmd.Process.Wait()
		done <- err
	}()

	// Wait for process to exit or timeout
	select {
	case <-timer.C:
		// Timeout occurred, forcefully kill the process
		if err := proc.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process after timeout: %w", err)
		}

		r.logger.Warn("Process killed after timeout",
			log.Str("instance_id", proc.instance.ID),
			log.Str("name", proc.instance.Name))

	case err := <-done:
		if err != nil {
			r.logger.Warn("Process exited with error",
				log.Str("instance_id", proc.instance.ID),
				log.Err(err))
		} else {
			r.logger.Info("Process stopped gracefully",
				log.Str("instance_id", proc.instance.ID),
				log.Str("name", proc.instance.Name))
		}
	}

	// Update status
	now := time.Now()
	proc.status.State = types.InstanceStatusStopped
	proc.status.FinishedAt = now

	// Update instance
	proc.instance.Status = types.InstanceStatusStopped
	proc.instance.UpdatedAt = now

	return nil
}

// Remove removes a process instance
func (r *ProcessRunner) Remove(ctx context.Context, instance *types.Instance, force bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	proc, exists := r.processes[instance.ID]
	if !exists {
		return fmt.Errorf("instance with ID %s not found", instance.ID)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// If the process is still running and force is not set, return error
	if proc.status.State == types.InstanceStatusRunning && !force {
		return fmt.Errorf("cannot remove running instance, use force to override")
	}

	// If running and force is set, try to kill it
	if proc.status.State == types.InstanceStatusRunning && force {
		if proc.cmd.Process != nil {
			if err := proc.cmd.Process.Kill(); err != nil {
				r.logger.Warn("Failed to kill process during forced removal",
					log.Str("instance_id", proc.instance.ID),
					log.Err(err))
			}
		}
	}

	// Close log file
	if proc.logFile != nil {
		_ = proc.logFile.Close()
	}

	// Remove workspace directory
	if err := os.RemoveAll(proc.workDir); err != nil {
		r.logger.Warn("Failed to remove workspace directory",
			log.Str("instance_id", proc.instance.ID),
			log.Err(err))
	}

	// Remove from process map
	delete(r.processes, instance.ID)

	r.logger.Info("Removed process instance",
		log.Str("instance_id", proc.instance.ID),
		log.Str("name", proc.instance.Name))

	return nil
}

// GetLogs retrieves logs from a process instance
func (r *ProcessRunner) GetLogs(ctx context.Context, instance *types.Instance, options runner.LogOptions) (io.ReadCloser, error) {
	r.mu.RLock()
	proc, exists := r.processes[instance.ID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("instance with ID %s not found", instance.ID)
	}

	proc.mu.RLock()
	defer proc.mu.RUnlock()

	// Ensure log file exists
	if proc.logFile == nil {
		return nil, fmt.Errorf("log file not found for instance %s", instance.ID)
	}

	// Get log file path
	logPath := proc.logFile.Name()

	// Open log file for reading
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// If Follow is not set, just return the file
	if !options.Follow {
		return file, nil
	}

	// For follow mode, we need a custom reader that will continuously read
	// until the process completes
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer pipeWriter.Close()

		// Start at the end if tail is 0
		var startPos int64 = 0
		if options.Tail > 0 {
			// Get file size
			info, err := file.Stat()
			if err != nil {
				r.logger.Error("Failed to get log file info", log.Err(err))
				return
			}

			// Find start position by counting lines from the end
			startPos = findStartPosition(file, info.Size(), options.Tail)
			_, err = file.Seek(startPos, 0)
			if err != nil {
				r.logger.Error("Failed to seek log file", log.Err(err))
				return
			}
		}

		buffer := make([]byte, 4096)
		stopCh := make(chan struct{})

		// Setup context cancellation
		go func() {
			<-ctx.Done()
			close(stopCh)
		}()

		for {
			select {
			case <-stopCh:
				return
			default:
				n, err := file.Read(buffer)
				if n > 0 {
					_, writeErr := pipeWriter.Write(buffer[:n])
					if writeErr != nil {
						r.logger.Error("Failed to write to pipe", log.Err(writeErr))
						return
					}
				}
				if err == io.EOF {
					// Check if process is still running
					if proc.status.State != types.InstanceStatusRunning {
						return
					}
					// If still running, wait a bit and try again
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if err != nil {
					r.logger.Error("Error reading log file", log.Err(err))
					return
				}
			}
		}
	}()

	return pipeReader, nil
}

// Status retrieves the current status of a process instance
func (r *ProcessRunner) Status(ctx context.Context, instance *types.Instance) (types.InstanceStatus, error) {
	r.mu.RLock()
	proc, exists := r.processes[instance.ID]
	r.mu.RUnlock()

	if !exists {
		return types.InstanceStatusFailed, fmt.Errorf("instance with ID %s not found", instance.ID)
	}

	proc.mu.RLock()
	defer proc.mu.RUnlock()

	return proc.status.State, nil
}

// List lists all process instances
func (r *ProcessRunner) List(ctx context.Context, namespace string) ([]*types.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*types.Instance, 0, len(r.processes))
	for _, proc := range r.processes {
		proc.mu.RLock()
		// Make a copy of the instance to avoid concurrent access issues
		instance := *proc.instance
		proc.mu.RUnlock()
		instances = append(instances, &instance)
	}

	return instances, nil
}

// findStartPosition finds the position to start reading to get the last n lines
func findStartPosition(file *os.File, fileSize int64, numLines int) int64 {
	// Return to start after function
	defer file.Seek(0, 0)

	// If numLines is 0 or negative, return the start of the file
	if numLines <= 0 {
		return 0
	}

	// If the file is empty, return 0
	if fileSize == 0 {
		return 0
	}

	// Buffer for reading the file backwards
	buf := make([]byte, 1)
	lineCount := 0
	var pos int64

	// Start from the end of the file
	for pos = fileSize - 1; pos >= 0; pos-- {
		// Seek to the current position
		_, err := file.Seek(pos, 0)
		if err != nil {
			return 0
		}

		// Read one byte
		_, err = file.Read(buf)
		if err != nil {
			return 0
		}

		// Check if we found a newline
		if buf[0] == '\n' {
			lineCount++
			if lineCount > numLines {
				// We found the position, return the next byte
				return pos + 1
			}
		}
	}

	// If we reached here, we didn't find enough lines, return the start of the file
	return 0
}

// Exec creates an interactive exec session within a running process.
func (r *ProcessRunner) Exec(ctx context.Context, instance *types.Instance, options runner.ExecOptions) (runner.ExecStream, error) {
	r.mu.RLock()
	_, exists := r.processes[instance.ID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("instance not found: %s", instance.ID)
	}

	// Verify instance is running
	status, err := r.Status(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance status: %w", err)
	}

	if status != types.InstanceStatusRunning {
		return nil, fmt.Errorf("instance is not running, status: %s", status)
	}

	// Create the exec stream
	// For a process, exec means starting a new process (we can't exec inside an existing process)
	// We could potentially use different methods like ptrace for more advanced use cases
	execStream, err := NewProcessExecStream(ctx, instance.ID, options, r.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec stream: %w", err)
	}

	return execStream, nil
}
