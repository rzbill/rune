package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/manager"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LogService implements the gRPC LogService.
type LogService struct {
	generated.UnimplementedLogServiceServer

	runnerManager *manager.RunnerManager
	store         store.Store
	logger        log.Logger
}

// NewLogService creates a new LogService with the given runners, store, and logger.
func NewLogService(runnerManager *manager.RunnerManager, store store.Store, logger log.Logger) *LogService {
	return &LogService{
		runnerManager: runnerManager,
		store:         store,
		logger:        logger.WithComponent("log-service"),
	}
}

// StreamLogs provides bidirectional streaming for logs.
func (s *LogService) StreamLogs(stream generated.LogService_StreamLogsServer) error {
	// Get the initial request
	req, err := stream.Recv()
	if err != nil {
		s.logger.Error("Failed to receive initial log request", log.Err(err))
		return status.Errorf(codes.Internal, "failed to receive initial request: %v", err)
	}

	// Validate the request
	if err := s.validateLogRequest(req); err != nil {
		s.logger.Error("Invalid log request", log.Err(err))
		return status.Errorf(codes.InvalidArgument, "invalid log request: %v", err)
	}

	// Set up context with cancel
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Process request based on target (service or instance)
	var instances []*types.Instance
	var namespace string
	var serviceName string

	if req.GetServiceName() != "" {
		// Target is a service - need to get all instances
		serviceName = req.GetServiceName()
		namespace = req.Namespace
		if namespace == "" {
			namespace = DefaultNamespace
		}

		// Get the service from the store
		var service types.Service
		if err := s.store.Get(ctx, ResourceTypeService, namespace, serviceName, &service); err != nil {
			if IsNotFound(err) {
				return status.Errorf(codes.NotFound, "service not found: %s", serviceName)
			}
			s.logger.Error("Failed to get service", log.Err(err))
			return status.Errorf(codes.Internal, "failed to get service: %v", err)
		}

		// Get all instances for the service
		var listedInstances []types.Instance
		err = s.store.List(ctx, ResourceTypeInstance, namespace, &listedInstances)
		if err != nil {
			s.logger.Error("Failed to list instances", log.Err(err))
			return status.Errorf(codes.Internal, "failed to list instances: %v", err)
		}

		// Filter instances by service ID
		for _, inst := range listedInstances {
			if inst.ServiceID == serviceName {
				instances = append(instances, &inst)
			}
		}

		if len(instances) == 0 {
			return status.Errorf(codes.NotFound, "no instances found for service: %s", serviceName)
		}
	} else {
		// Target is a specific instance
		// Get the instance to determine its service and namespace
		instanceService := NewInstanceService(s.store, s.runnerManager, s.logger)
		instanceResp, err := instanceService.GetInstance(ctx, &generated.GetInstanceRequest{Id: req.GetInstanceId()})
		if err != nil {
			return err
		}

		instance, err := instanceService.ProtoInstanceToInstanceModel(instanceResp.Instance)
		if err != nil {
			return err
		}
		instances = append(instances, instance)
		serviceName = instance.ServiceID
	}

	// Set up log options
	logOptions := runner.LogOptions{
		Follow:     req.Follow,
		Tail:       int(req.Tail),
		Timestamps: req.Timestamps,
	}

	// Parse timestamps if provided
	if req.Since != "" {
		since, err := time.Parse(time.RFC3339, req.Since)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid since timestamp: %v", err)
		}
		logOptions.Since = since
	}

	if req.Until != "" {
		until, err := time.Parse(time.RFC3339, req.Until)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid until timestamp: %v", err)
		}
		logOptions.Until = until
	}

	// Channel to coordinate log streaming for multiple instances
	logCh := make(chan *generated.LogResponse, 100)
	defer close(logCh)

	// Error channel to propagate errors from goroutines
	errCh := make(chan error, len(instances))

	// Track the active readers
	activeReaders := len(instances)

	// Start a goroutine for each instance to stream logs
	for _, instance := range instances {
		go func(instance *types.Instance) {
			defer func() {
				activeReaders--
				if activeReaders == 0 && !req.Follow {
					// All readers are done, and we're not following, so we can close the channel
					close(logCh)
				}
			}()

			// We'd normally determine this from the instance's runtime
			// For simplicity, we'll try docker first, then process
			logReader, err := s.getLogReader(ctx, instance, logOptions)
			if err != nil {
				s.logger.Error("Failed to get log reader", log.Str("instanceId", instance.ID), log.Err(err))
				errCh <- fmt.Errorf("failed to get logs for instance %s: %v", instance.ID, err)
				return
			}
			defer logReader.Close()

			// Stream logs from the reader
			buf := make([]byte, 4096)
			for {
				n, err := logReader.Read(buf)
				if err != nil {
					if err == io.EOF {
						s.logger.Debug("End of logs", log.Str("instanceId", instance.ID))
						return
					}
					s.logger.Error("Failed to read logs", log.Str("instanceId", instance.ID), log.Err(err))
					errCh <- fmt.Errorf("failed to read logs for instance %s: %v", instance.ID, err)
					return
				}

				// Send the log chunk to the client
				if n > 0 {
					logCh <- &generated.LogResponse{
						InstanceId:  instance.ID,
						ServiceName: serviceName,
						Content:     string(buf[:n]),
						Timestamp:   time.Now().Format(time.RFC3339),
						Stream:      "stdout", // This is simplified - would need to determine actual stream
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
		}(instance)
	}

	// Handle parameter updates from client
	go func() {
		for {
			newReq, err := stream.Recv()
			if err == io.EOF {
				// Client closed stream
				cancel()
				return
			}
			if err != nil {
				s.logger.Error("Failed to receive log request update", log.Err(err))
				errCh <- fmt.Errorf("failed to receive request update: %v", err)
				cancel()
				return
			}

			// Handle parameter update
			if newReq.ParameterUpdate {
				// Cancel current loggers and start new ones with updated parameters
				// This is a simplified approach - a real implementation would be more sophisticated
				cancel()
				return
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

	// Stream logs to client
	for {
		select {
		case logResp, ok := <-logCh:
			if !ok {
				// Channel closed
				return nil
			}
			if err := stream.Send(logResp); err != nil {
				s.logger.Error("Failed to send log response", log.Err(err))
				return status.Errorf(codes.Internal, "failed to send log response: %v", err)
			}
		case err := <-errCh:
			s.logger.Error("Log streaming error", log.Err(err))
			return status.Errorf(codes.Internal, "log streaming error: %v", err)
		case <-ctx.Done():
			s.logger.Debug("Context cancelled")
			return nil
		}
	}
}

// validateLogRequest validates a log request.
func (s *LogService) validateLogRequest(req *generated.LogRequest) error {
	// Must specify either service name or instance ID
	if req.GetServiceName() == "" && req.GetInstanceId() == "" {
		return fmt.Errorf("must specify either service_name or instance_id")
	}

	// Can't specify both service name and instance ID
	if req.GetServiceName() != "" && req.GetInstanceId() != "" {
		return fmt.Errorf("cannot specify both service_name and instance_id")
	}

	// Validate since and until timestamps
	if req.Since != "" {
		if _, err := time.Parse(time.RFC3339, req.Since); err != nil {
			return fmt.Errorf("invalid since timestamp: %v", err)
		}
	}

	if req.Until != "" {
		if _, err := time.Parse(time.RFC3339, req.Until); err != nil {
			return fmt.Errorf("invalid until timestamp: %v", err)
		}
	}

	return nil
}

// getLogReader gets a log reader for an instance from the appropriate runner.
func (s *LogService) getLogReader(ctx context.Context, instance *types.Instance, options runner.LogOptions) (io.ReadCloser, error) {
	dockerRunner, err := s.runnerManager.GetDockerRunner()
	if err != nil {
		return nil, fmt.Errorf("failed to get docker runner: %v", err)
	}

	// Try the docker runner first
	if dockerRunner != nil {
		reader, err := dockerRunner.GetLogs(ctx, instance, options)
		if err == nil {
			return reader, nil
		}
		// If not found, try the process runner
		s.logger.Debug("Docker runner failed to get logs, trying process runner", log.Str("instanceId", instance.ID), log.Err(err))
	}

	// Try the process runner
	processRunner, err := s.runnerManager.GetProcessRunner()
	if err != nil {
		return nil, fmt.Errorf("failed to get process runner: %v", err)
	}

	if processRunner != nil {
		reader, err := processRunner.GetLogs(ctx, instance, options)
		if err == nil {
			return reader, nil
		}
		return nil, fmt.Errorf("process runner failed to get logs: %v", err)
	}

	return nil, fmt.Errorf("no runners available")
}
