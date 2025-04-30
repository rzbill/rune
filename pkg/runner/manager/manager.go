package manager

import (
	"sync"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/docker"
	"github.com/rzbill/rune/pkg/runner/process"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is a simple error type
type Error string

func (e Error) Error() string { return string(e) }

// Define errors
var (
	ErrNotInitialized            = Error("runner manager not initialized")
	ErrDockerRunnerNotAvailable  = Error("docker runner not available")
	ErrProcessRunnerNotAvailable = Error("process runner not available")
)

// RunnerManager provides centralized access to various runners
type RunnerManager struct {
	dockerRunner  runner.Runner
	processRunner runner.Runner
	logger        log.Logger
	mutex         sync.RWMutex
	initialized   bool
}

// NewRunnerManager creates a new runner manager
func NewRunnerManager(logger log.Logger) *RunnerManager {
	return &RunnerManager{
		logger: logger.WithComponent("runner-manager"),
	}
}

// Initialize initializes all runners
func (m *RunnerManager) Initialize() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.initialized {
		return nil
	}

	// Create Docker runner
	dockerRunner, err := docker.NewDockerRunner(m.logger.WithComponent("docker-runner"))
	if err != nil {
		m.logger.Warn("Failed to initialize Docker runner", log.Err(err))
	} else {
		m.dockerRunner = dockerRunner
	}

	// Create Process runner
	processRunner, err := process.NewProcessRunner(
		process.WithLogger(m.logger.WithComponent("process-runner")),
	)
	if err != nil {
		m.logger.Warn("Failed to initialize Process runner", log.Err(err))
	} else {
		m.processRunner = processRunner
	}

	m.initialized = true
	return nil
}

// GetDockerRunner returns the Docker runner
func (m *RunnerManager) GetDockerRunner() (runner.Runner, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !m.initialized {
		return nil, ErrNotInitialized
	}

	if m.dockerRunner == nil {
		return nil, ErrDockerRunnerNotAvailable
	}

	return m.dockerRunner, nil
}

// GetProcessRunner returns the Process runner
func (m *RunnerManager) GetProcessRunner() (runner.Runner, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !m.initialized {
		return nil, ErrNotInitialized
	}

	if m.processRunner == nil {
		return nil, ErrProcessRunnerNotAvailable
	}

	return m.processRunner, nil
}

// Close closes all runners
func (m *RunnerManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Close runners if they implement a Close method
	if closer, ok := m.dockerRunner.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			m.logger.Warn("Error closing Docker runner", log.Err(err))
		}
	}

	if closer, ok := m.processRunner.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			m.logger.Warn("Error closing Process runner", log.Err(err))
		}
	}

	m.initialized = false
	return nil
}

// GetInstanceRunner returns the appropriate runner to use for the given instance.
func (m *RunnerManager) GetInstanceRunner(instance *types.Instance) (runner.Runner, error) {
	switch instance.Runner {
	case types.RunnerTypeDocker:
		dockerRunner, err := m.GetDockerRunner()
		if err != nil {
			return nil, status.Error(codes.Unavailable, "docker runner not available")
		}
		return dockerRunner, nil

	case types.RunnerTypeProcess:
		processRunner, err := m.GetProcessRunner()
		if err != nil {
			return nil, status.Error(codes.Unavailable, "process runner not available")
		}
		return processRunner, nil

	default:
		// Fallback to existing heuristic for backward compatibility
		if instance.ContainerID != "" {
			dockerRunner, err := m.GetDockerRunner()
			if err != nil {
				return nil, status.Error(codes.Unavailable, "docker runner not available")
			}
			return dockerRunner, nil
		} else {
			processRunner, err := m.GetProcessRunner()
			if err != nil {
				return nil, status.Error(codes.Unavailable, "process runner not available")
			}
			return processRunner, nil
		}
	}
}

// GetServiceRunner returns the appropriate runner to use for the given service.
func (m *RunnerManager) GetServiceRunner(service *types.Service) (runner.Runner, error) {
	if service.Runtime == types.RuntimeTypeProcess {
		processRunner, err := m.GetProcessRunner()
		if err != nil {
			return nil, status.Error(codes.Unavailable, "process runner not available")
		}
		return processRunner, nil
	}

	dockerRunner, err := m.GetDockerRunner()
	if err != nil {
		return nil, status.Error(codes.Unavailable, "docker runner not available")
	}

	return dockerRunner, nil
}

/*
// SetDockerRunner sets a custom Docker runner (for testing)
func (m *RunnerManager) SetDockerRunner(runner runner.Runner) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.dockerRunner = runner
	if !m.initialized {
		m.initialized = true
	}
}

// SetProcessRunner sets a custom Process runner (for testing)
func (m *RunnerManager) SetProcessRunner(runner runner.Runner) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.processRunner = runner
	if !m.initialized {
		m.initialized = true
	}
}*/
