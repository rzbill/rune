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

// LogService implements the gRPC LogService.
type LogService struct {
	generated.UnimplementedLogServiceServer

	store        store.Store
	logger       log.Logger
	orchestrator orchestrator.Orchestrator
}

// NewLogService creates a new LogService with the given runners, store, and logger.
func NewLogService(store store.Store, logger log.Logger, orchestrator orchestrator.Orchestrator) *LogService {
	return &LogService{
		store:        store,
		logger:       logger.WithComponent("log-service"),
		orchestrator: orchestrator,
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
	var namespace string
	var serviceName string
	var instanceID string

	namespace = req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Set up log options
	logOptions := types.LogOptions{
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

	// Channel to collect log output
	logCh := make(chan *generated.LogResponse, 100)
	defer close(logCh)

	// Error channel to propagate errors from goroutines
	errCh := make(chan error, 1)

	// Get logs based on target
	var logReader io.ReadCloser
	if req.GetServiceName() != "" {
		// Target is a service
		serviceName = req.GetServiceName()

		// Verify service exists
		var service types.Service
		if err := s.store.Get(ctx, types.ResourceTypeService, namespace, serviceName, &service); err != nil {
			if IsNotFound(err) {
				return status.Errorf(codes.NotFound, "service not found: %s", serviceName)
			}
			s.logger.Error("Failed to get service", log.Err(err))
			return status.Errorf(codes.Internal, "failed to get service: %v", err)
		}

		// Use orchestrator to get service logs
		logReader, err = s.orchestrator.GetServiceLogs(ctx, namespace, serviceName, logOptions)
		if err != nil {
			s.logger.Error("Failed to get service logs",
				log.Str("service", serviceName),
				log.Err(err))
			return status.Errorf(codes.Internal, "failed to get service logs: %v", err)
		}
	} else {
		// Target is a specific instance
		instanceID = req.GetInstanceId()

		// Get the instance to determine its service
		var instance types.Instance
		if err := s.store.Get(ctx, types.ResourceTypeInstance, namespace, instanceID, &instance); err != nil {
			if IsNotFound(err) {
				return status.Errorf(codes.NotFound, "instance not found: %s", instanceID)
			}
			s.logger.Error("Failed to get instance", log.Err(err))
			return status.Errorf(codes.Internal, "failed to get instance: %v", err)
		}

		serviceName = instance.ServiceID

		// Use orchestrator to get instance logs
		logReader, err = s.orchestrator.GetInstanceLogs(ctx, namespace, serviceName, instanceID, logOptions)
		if err != nil {
			s.logger.Error("Failed to get instance logs",
				log.Str("instance", instanceID),
				log.Err(err))
			return status.Errorf(codes.Internal, "failed to get instance logs: %v", err)
		}
	}

	if logReader == nil {
		return status.Errorf(codes.Internal, "failed to create log reader")
	}
	defer logReader.Close()

	// Start a goroutine to read from logReader and send to logCh
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := logReader.Read(buf)
			if err != nil {
				if err == io.EOF {
					s.logger.Debug("End of logs")
					return
				}
				s.logger.Error("Failed to read logs", log.Err(err))
				errCh <- fmt.Errorf("failed to read logs: %v", err)
				return
			}

			// Send the log chunk to the client
			if n > 0 {
				logCh <- &generated.LogResponse{
					InstanceId:  instanceID,
					ServiceName: serviceName,
					Content:     string(buf[:n]),
					Timestamp:   time.Now().Format(time.RFC3339),
					Stream:      "stdout", // This is simplified - the orchestrator doesn't differentiate streams
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
