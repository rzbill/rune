package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/orchestrator"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecService implements the gRPC ExecService.
type ExecService struct {
	generated.UnimplementedExecServiceServer

	store        store.Store
	logger       log.Logger
	orchestrator orchestrator.Orchestrator
}

// NewExecService creates a new ExecService with the given runners, store, and logger.
func NewExecService(store store.Store, logger log.Logger, orchestrator orchestrator.Orchestrator) *ExecService {
	return &ExecService{
		store:        store,
		logger:       logger.WithComponent("exec-service"),
		orchestrator: orchestrator,
	}
}

// getTargetInstance resolves an instance from an init request, handling both service-based and instance-based targeting
func (s *ExecService) getTargetInstance(ctx context.Context, initReq *generated.ExecInitRequest) (*types.Instance, error) {
	namespace := initReq.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	if initReq.GetServiceName() != "" {
		// Target is a service - need to select an instance
		serviceName := initReq.GetServiceName()

		// Get the service from the store
		var service types.Service
		if err := s.store.Get(ctx, types.ResourceTypeService, namespace, serviceName, &service); err != nil {
			if IsNotFound(err) {
				return nil, status.Errorf(codes.NotFound, "service not found: %s", serviceName)
			}
			s.logger.Error("Failed to get service", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
		}

		// Get all instances for the service
		var instances []types.Instance
		err := s.store.List(ctx, types.ResourceTypeInstance, namespace, &instances)
		if err != nil {
			s.logger.Error("Failed to list instances", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to list instances: %v", err)
		}

		// Filter instances by service ID and running status
		var runningInstances []string
		for _, instance := range instances {
			if instance.ServiceID != serviceName {
				continue
			}

			if instance.Status == types.InstanceStatusRunning {
				runningInstances = append(runningInstances, instance.ID)
			}
		}

		if len(runningInstances) == 0 {
			return nil, status.Errorf(codes.NotFound, "no running instances found for service: %s", serviceName)
		}

		// Just pick the first running instance for simplicity
		instanceID := runningInstances[0]

		// Get the full instance details
		var instance types.Instance
		if err := s.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
			s.logger.Error("Failed to get instance", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to get instance: %v", err)
		}

		return &instance, nil
	} else {
		// Target is a specific instance
		instanceID := initReq.GetInstanceId()

		// Get the instance details
		var instance types.Instance
		if err := s.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
			if IsNotFound(err) {
				return nil, status.Errorf(codes.NotFound, "instance not found: %s", instanceID)
			}
			s.logger.Error("Failed to get instance", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to get instance: %v", err)
		}

		// Check if the instance is running
		if instance.Status != types.InstanceStatusRunning {
			return nil, status.Errorf(codes.FailedPrecondition, "instance is not running, status: %s", instance.Status)
		}

		return &instance, nil
	}
}

// StreamExec provides bidirectional streaming for exec operations.
func (s *ExecService) StreamExec(stream generated.ExecService_StreamExecServer) error {
	// Get the initial request
	req, err := stream.Recv()
	if err != nil {
		s.logger.Error("Failed to receive initial exec request", log.Err(err))
		return status.Errorf(codes.Internal, "failed to receive initial request: %v", err)
	}

	// Validate the request
	if err := s.validateExecRequest(req); err != nil {
		s.logger.Error("Invalid exec request", log.Err(err))
		return status.Errorf(codes.InvalidArgument, "invalid exec request: %v", err)
	}

	// Get the init request from the oneof
	initReq := req.GetInit()
	if initReq == nil {
		return status.Errorf(codes.InvalidArgument, "first request must be an init request")
	}

	// Set up context with cancel
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	namespace := initReq.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Convert gRPC options to orchestrator options
	execOptions := orchestrator.ExecOptions{
		Command:        initReq.Command,
		Env:            initReq.Env,
		WorkingDir:     initReq.WorkingDir,
		TTY:            initReq.Tty,
		TerminalWidth:  initReq.TerminalSize.GetWidth(),
		TerminalHeight: initReq.TerminalSize.GetHeight(),
	}

	// Use orchestrator to execute the command
	var execStream orchestrator.ExecStream
	var instanceID string

	if initReq.GetServiceName() != "" {
		// Target is a service - use ExecInService
		serviceName := initReq.GetServiceName()
		execStream, err = s.orchestrator.ExecInService(ctx, namespace, serviceName, execOptions)
		if err != nil {
			s.logger.Error("Failed to exec in service",
				log.Str("service", serviceName),
				log.Err(err))
			return status.Errorf(codes.Internal, "failed to exec in service: %v", err)
		}
	} else {
		// Target is a specific instance
		instanceID = initReq.GetInstanceId()

		// Need to determine service ID for the instance
		var instance types.Instance
		if err := s.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
			if IsNotFound(err) {
				return status.Errorf(codes.NotFound, "instance not found: %s", instanceID)
			}
			s.logger.Error("Failed to get instance", log.Err(err))
			return status.Errorf(codes.Internal, "failed to get instance: %v", err)
		}

		serviceName := instance.ServiceName
		fmt.Printf("Before====>>>>>>ExecInInstance %+v", instance)
		execStream, err = s.orchestrator.ExecInInstance(ctx, namespace, serviceName, instanceID, execOptions)
		fmt.Printf("\nAfter====>>>>>>ExecInInstance %+v", err)
		if err != nil {
			s.logger.Error("Failed to exec in instance",
				log.Str("instance", instanceID),
				log.Err(err))
			return status.Errorf(codes.Internal, "failed to exec in instance: %v", err)
		}
		instanceID = instance.ID
	}

	if execStream == nil {
		return status.Errorf(codes.Internal, "failed to create exec stream")
	}
	defer execStream.Close()

	// Send a status message to indicate successful connection
	if err := stream.Send(&generated.ExecResponse{
		Response: &generated.ExecResponse_Status{
			Status: &generated.Status{
				Code:    int32(codes.OK),
				Message: fmt.Sprintf("Connected to instance %s", instanceID),
			},
		},
	}); err != nil {
		s.logger.Error("Failed to send status response", log.Err(err))
		return status.Errorf(codes.Internal, "failed to send status response: %v", err)
	}

	// Channel to coordinate between stdin and stdout/stderr streams
	doneCh := make(chan error, 1)
	errorCh := make(chan error, 2)

	// Handle stdin from client to stream
	go func() {
		defer func() {
			// Signal that stdin processing is done
			doneCh <- nil
		}()

		for {
			req, err := stream.Recv()
			if err == io.EOF {
				// Client closed the stream
				return
			}
			if err != nil {
				s.logger.Error("Failed to receive exec request", log.Err(err))
				errorCh <- fmt.Errorf("failed to receive exec request: %w", err)
				return
			}

			// Process request based on type
			switch {
			case req.GetStdin() != nil:
				// Write stdin data to the exec stream
				_, err := execStream.Write(req.GetStdin())
				if err != nil {
					s.logger.Error("Failed to write to exec stream", log.Err(err))
					errorCh <- fmt.Errorf("failed to write to exec stream: %w", err)
					return
				}

			case req.GetResize() != nil:
				// Resize terminal
				resize := req.GetResize()
				err := execStream.ResizeTerminal(resize.Width, resize.Height)
				if err != nil {
					s.logger.Warn("Failed to resize terminal", log.Err(err))
					// Don't fail on resize errors, just log them
				}

			case req.GetSignal() != nil:
				// Send signal to process
				signal := req.GetSignal()
				err := execStream.Signal(signal.Name)
				if err != nil {
					s.logger.Warn("Failed to send signal", log.Str("signal", signal.Name), log.Err(err))
					// Don't fail on signal errors, just log them
				}

			default:
				// Ignore unknown request types
				s.logger.Warn("Received unknown exec request type")
			}

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
				// Continue
			}
		}
	}()

	// Handle stdout from stream to client
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := execStream.Read(buf)
			if err == io.EOF {
				// End of stream
				return
			}
			if err != nil {
				s.logger.Error("Failed to read from exec stream", log.Err(err))
				errorCh <- fmt.Errorf("failed to read from exec stream: %w", err)
				return
			}

			if n > 0 {
				// Send stdout data to client
				if err := stream.Send(&generated.ExecResponse{
					Response: &generated.ExecResponse_Stdout{
						Stdout: buf[:n],
					},
				}); err != nil {
					s.logger.Error("Failed to send stdout", log.Err(err))
					errorCh <- fmt.Errorf("failed to send stdout: %w", err)
					return
				}
			}

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
				// Continue
			}
		}
	}()

	// Handle stderr from stream to client
	go func() {
		stderrReader := execStream.Stderr()
		buf := make([]byte, 4096)
		for {
			n, err := stderrReader.Read(buf)
			if err == io.EOF {
				// End of stream
				return
			}
			if err != nil {
				s.logger.Error("Failed to read from stderr", log.Err(err))
				errorCh <- fmt.Errorf("failed to read from stderr: %w", err)
				return
			}

			if n > 0 {
				// Send stderr data to client
				if err := stream.Send(&generated.ExecResponse{
					Response: &generated.ExecResponse_Stderr{
						Stderr: buf[:n],
					},
				}); err != nil {
					s.logger.Error("Failed to send stderr", log.Err(err))
					errorCh <- fmt.Errorf("failed to send stderr: %w", err)
					return
				}
			}

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
				// Continue
			}
		}
	}()

	// Wait for completion or error
	var finalErr error
	select {
	case err := <-errorCh:
		finalErr = err
		cancel() // Cancel context to stop other goroutines
	case <-doneCh:
		// Client closed the stream, wait for process to complete
		// or for context to be cancelled
		select {
		case err := <-errorCh:
			finalErr = err
		case <-ctx.Done():
			// Context cancelled
		case <-time.After(500 * time.Millisecond):
			// Give a small grace period for exit code
		}
	case <-ctx.Done():
		// Context cancelled
	}

	// Get exit code
	exitCode, err := execStream.ExitCode()
	if err == nil {
		// Send exit info
		if sendErr := stream.Send(&generated.ExecResponse{
			Response: &generated.ExecResponse_Exit{
				Exit: &generated.ExitInfo{
					Code:     int32(exitCode),
					Signaled: exitCode > 128, // A common way to check if process was terminated by signal
				},
			},
		}); sendErr != nil {
			s.logger.Error("Failed to send exit info", log.Err(sendErr))
			if finalErr == nil {
				finalErr = sendErr
			}
		}
	} else {
		s.logger.Warn("Failed to get exit code", log.Err(err))
	}

	return finalErr
}

// validateExecRequest validates an exec request.
func (s *ExecService) validateExecRequest(req *generated.ExecRequest) error {
	// First request must be an init request
	initReq := req.GetInit()
	if initReq == nil {
		return fmt.Errorf("first request must be an init request")
	}

	// Must specify either service name or instance ID
	if initReq.GetServiceName() == "" && initReq.GetInstanceId() == "" {
		return fmt.Errorf("must specify either service_name or instance_id")
	}

	// Can't specify both service name and instance ID
	if initReq.GetServiceName() != "" && initReq.GetInstanceId() != "" {
		return fmt.Errorf("cannot specify both service_name and instance_id")
	}

	// Command must be specified
	if len(initReq.Command) == 0 {
		return fmt.Errorf("command is required")
	}

	return nil
}
