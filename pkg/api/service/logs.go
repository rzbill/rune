package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
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

// parseLogLine parses a log line from MultiLogStreamer and converts it to a LogResponse
// Format from MultiLogStreamer is: @@LOG_META|[instanceID|instanceName|timestamp]@@ content
func (s *LogService) parseLogLine(line, serviceName, fallbackInstanceName string) *generated.LogResponse {
	// Extract metadata using the orchestrator's function
	instanceID, instanceName, timestamp, content := orchestrator.ExtractLineMetadata(line)

	// Use fallback instance ID if not found in metadata
	if instanceName == "" {
		instanceName = fallbackInstanceName
	}

	// Determine log level from content
	logLevel := "info" // Default level
	if strings.Contains(strings.ToLower(content), "error") ||
		strings.Contains(strings.ToLower(content), "exception") ||
		strings.Contains(strings.ToLower(content), "failed") {
		logLevel = "error"
	} else if strings.Contains(strings.ToLower(content), "warn") {
		logLevel = "warning"
	}

	// Create and return the LogResponse
	return &generated.LogResponse{
		ServiceName:  serviceName,
		InstanceId:   instanceID,
		InstanceName: instanceName,
		Timestamp:    timestamp,
		Content:      content,
		Stream:       "stdout",
		LogLevel:     logLevel,
	}
}

// processSingleLine processes a single log line from the reader
func (s *LogService) readLogsFromReader(ctx context.Context, logReader io.ReadCloser, logCh chan<- *generated.LogResponse, errCh chan<- error, serviceName, instanceName string) {
	scanner := bufio.NewScanner(logReader)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse the log line and send it to the channel
		logCh <- s.parseLogLine(line, serviceName, instanceName)

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
			// Continue
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.Error("Failed to scan logs", log.Err(err))
		errCh <- fmt.Errorf("failed to scan logs: %v", err)
		return
	}

	s.logger.Debug("End of logs")
}

// buildLogOptions converts a log request into log options
func (s *LogService) buildLogOptions(req *generated.LogRequest) (types.LogOptions, error) {
	logOptions := types.LogOptions{
		Follow:     req.Follow,
		Tail:       int(req.Tail),
		Timestamps: req.Timestamps,
		ShowLogs:   req.ShowLogs,
		ShowEvents: req.ShowEvents,
		ShowStatus: req.ShowStatus,
	}

	// If none of the show options are specified, default to showing all
	if !req.ShowLogs && !req.ShowEvents && !req.ShowStatus {
		logOptions.ShowLogs = true
		logOptions.ShowEvents = true
		logOptions.ShowStatus = true
	}

	// Parse timestamps if provided
	if req.Since != "" {
		since, err := time.Parse(time.RFC3339, req.Since)
		if err != nil {
			return logOptions, fmt.Errorf("invalid since timestamp: %v", err)
		}
		logOptions.Since = since
	}

	if req.Until != "" {
		until, err := time.Parse(time.RFC3339, req.Until)
		if err != nil {
			return logOptions, fmt.Errorf("invalid until timestamp: %v", err)
		}
		logOptions.Until = until
	}

	return logOptions, nil
}

// getLogReader returns the appropriate log reader based on the request type
func (s *LogService) getLogReader(ctx context.Context, req *generated.LogRequest, logOptions types.LogOptions) (io.ReadCloser, string, string, error) {
	var logReader io.ReadCloser
	var err error
	var serviceName string
	var instanceName string

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	resourceType, resource, err := s.identifyResourceType(ctx, req.ResourceTarget, namespace)
	if err != nil {
		s.logger.Error("Failed to identify resource type", log.Err(err))
		return nil, "", "", status.Errorf(codes.InvalidArgument, "invalid resource target: %v", err)
	}

	if resourceType == types.ResourceTypeService {
		// Target is a service
		service, ok := resource.(types.Service)
		if !ok {
			return nil, "", "", status.Errorf(codes.Internal, "resource is not a service")
		}

		// Use orchestrator to get service logs
		logReader, err = s.orchestrator.GetServiceLogs(ctx, namespace, service.Name, logOptions)
		if err != nil {
			s.logger.Error("Failed to get service logs",
				log.Str("service", serviceName),
				log.Err(err))
			return nil, "", "", status.Errorf(codes.Internal, "failed to get service logs: %v", err)
		}
	}

	if resourceType == types.ResourceTypeInstance {

		instance, ok := resource.(types.Instance)
		if !ok {
			return nil, "", "", status.Errorf(codes.Internal, "resource is not an instance ---0")
		}

		// Use orchestrator to get instance logs
		logReader, err = s.orchestrator.GetInstanceLogs(ctx, namespace, instance.ID, logOptions)
		if err != nil {
			s.logger.Error("Failed to get instance logs",
				log.Str("instance", instanceName),
				log.Err(err))
			return nil, "", "", status.Errorf(codes.Internal, "failed to get instance logs: %v", err)
		}
	}

	if logReader == nil {
		return nil, "", "", status.Errorf(codes.Internal, "failed to create log reader")
	}

	return logReader, serviceName, instanceName, nil
}

// handleParameterUpdates listens for parameter updates from the client
func (s *LogService) handleParameterUpdates(ctx context.Context, stream generated.LogService_StreamLogsServer, errCh chan<- error) {
	for {
		newReq, err := stream.Recv()
		if err == io.EOF {
			// Client closed stream
			return
		}
		if err != nil {
			s.logger.Error("Failed to receive log request update", log.Err(err))
			errCh <- fmt.Errorf("failed to receive request update: %v", err)
			return
		}

		// Handle parameter update
		if newReq.ParameterUpdate {
			// Signal that we need to update parameters
			errCh <- fmt.Errorf("parameter update requested")
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

	// Build log options from request
	logOptions, err := s.buildLogOptions(req)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, err.Error())
	}

	// Get the appropriate log reader
	logReader, serviceName, instanceName, err := s.getLogReader(ctx, req, logOptions)
	if err != nil {
		return err
	}
	defer logReader.Close()

	// Channel to collect log output
	logCh := make(chan *generated.LogResponse, 100)
	defer close(logCh)

	// Error channel to propagate errors from goroutines
	errCh := make(chan error, 1)

	// Start a goroutine to read from logReader and send to logCh
	go s.readLogsFromReader(ctx, logReader, logCh, errCh, serviceName, instanceName)

	// Handle parameter updates from client
	go s.handleParameterUpdates(ctx, stream, errCh)

	// Stream logs to client
	return s.streamLogsToClient(ctx, stream, logCh, errCh)
}

// streamLogsToClient sends log responses to the client
func (s *LogService) streamLogsToClient(ctx context.Context, stream generated.LogService_StreamLogsServer,
	logCh <-chan *generated.LogResponse, errCh <-chan error) error {

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
	if req.ResourceTarget == "" {
		return fmt.Errorf("must specify either service name or instance ID or resource type/name")
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

// identifyResourceType attempts to identify the type of resource being queried
// It checks if the argument is service, instance, or type/name format
// Returns resource type, name, and any error that occurred
func (s *LogService) identifyResourceType(ctx context.Context, arg string, namespace string) (types.ResourceType, interface{}, error) {
	// Check if the argument is in the format TYPE/NAME
	if strings.Contains(arg, "/") {
		parts := strings.SplitN(arg, "/", 2)
		resourceType := strings.ToLower(parts[0])
		resourceName := parts[1]

		resource, err := s.getResourceByType(ctx, resourceType, resourceName, namespace)
		if err != nil {
			return "", nil, err
		}

		return types.ResourceType(resourceType), resource, nil
	}

	// Try to fetch as a service first
	var service types.Service
	err := s.store.Get(ctx, types.ResourceTypeService, namespace, arg, &service)
	if err == nil {
		// It's a service
		return types.ResourceTypeService, service, nil
	}

	// Try to fetch as an instance
	var instance types.Instance
	err = s.store.Get(ctx, types.ResourceTypeInstance, namespace, arg, &instance)
	if err != nil {
		return "", nil, err
	}

	return types.ResourceTypeInstance, instance, nil
}

func (s *LogService) getResourceByType(ctx context.Context, resourceType string, resourceName string, namespace string) (interface{}, error) {
	if resourceType == string(types.ResourceTypeService) {
		var service types.Service
		err := s.store.Get(ctx, types.ResourceTypeService, namespace, resourceName, &service)
		if err == nil {
			return service, nil
		}
	}

	if resourceType == string(types.ResourceTypeInstance) {
		var instance types.Instance
		err := s.store.Get(ctx, types.ResourceTypeInstance, namespace, resourceName, &instance)
		if err == nil {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("resource not found")
}
