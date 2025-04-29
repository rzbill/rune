package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HealthService implements the gRPC HealthService.
type HealthService struct {
	generated.UnimplementedHealthServiceServer

	store  store.Store
	logger log.Logger
}

// NewHealthService creates a new HealthService with the given store and logger.
func NewHealthService(store store.Store, logger log.Logger) *HealthService {
	return &HealthService{
		store:  store,
		logger: logger.WithComponent("health-service"),
	}
}

// GetHealth retrieves health status of platform components.
func (s *HealthService) GetHealth(ctx context.Context, req *generated.GetHealthRequest) (*generated.GetHealthResponse, error) {
	s.logger.Debug("GetHealth called", log.Str("component", req.ComponentType))

	// If no component type specified, default to API server health
	if req.ComponentType == "" {
		return s.getAPIServerHealth(ctx, req)
	}

	// Handle based on component type
	switch req.ComponentType {
	case "service":
		return s.getServiceHealth(ctx, req)
	case "instance":
		return s.getInstanceHealth(ctx, req)
	case "node":
		return s.getNodeHealth(ctx, req)
	case "api-server":
		return s.getAPIServerHealth(ctx, req)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown component type: %s", req.ComponentType)
	}
}

// getServiceHealth retrieves health for a service or services.
func (s *HealthService) getServiceHealth(ctx context.Context, req *generated.GetHealthRequest) (*generated.GetHealthResponse, error) {
	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	var components []*generated.ComponentHealth

	if req.Name != "" {
		// Get health for a specific service
		var service types.Service
		if err := s.store.Get(ctx, ResourceTypeService, namespace, req.Name, &service); err != nil {
			if IsNotFound(err) {
				return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Name)
			}
			s.logger.Error("Failed to get service", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
		}

		// Get instances for this service
		healthStatus, healthMessage := s.computeServiceHealthFromInstances(ctx, service.Name, namespace)

		component := &generated.ComponentHealth{
			ComponentType: "service",
			Id:            service.ID,
			Name:          service.Name,
			Namespace:     namespace,
			Status:        healthStatus,
			Message:       healthMessage,
			Timestamp:     time.Now().Format(time.RFC3339),
		}

		if req.IncludeChecks {
			// Include detailed check results if requested
			// This would extract from service.Health definitions and actual check results
			component.CheckResults = generateMockHealthCheckResults()
		}

		components = append(components, component)
	} else {
		// Get health for all services in the namespace
		var services []types.Service
		err := s.store.List(ctx, ResourceTypeService, namespace, &services)
		if err != nil {
			s.logger.Error("Failed to list services", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to list services: %v", err)
		}

		for _, service := range services {

			healthStatus, healthMessage := s.computeServiceHealthFromInstances(ctx, service.Name, namespace)

			component := &generated.ComponentHealth{
				ComponentType: "service",
				Id:            service.ID,
				Name:          service.Name,
				Namespace:     namespace,
				Status:        healthStatus,
				Message:       healthMessage,
				Timestamp:     time.Now().Format(time.RFC3339),
			}

			if req.IncludeChecks {
				// Include detailed check results if requested
				component.CheckResults = generateMockHealthCheckResults()
			}

			components = append(components, component)
		}
	}

	return &generated.GetHealthResponse{
		Components: components,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: fmt.Sprintf("Retrieved health for %d services", len(components)),
		},
	}, nil
}

// getInstanceHealth retrieves health for an instance or instances.
func (s *HealthService) getInstanceHealth(ctx context.Context, req *generated.GetHealthRequest) (*generated.GetHealthResponse, error) {
	// For now, we'll just return a mock response
	// In a real implementation, we would query the runners for instance health

	var components []*generated.ComponentHealth

	component := &generated.ComponentHealth{
		ComponentType: "instance",
		Id:            req.Name, // Using name as ID
		Name:          req.Name,
		Namespace:     req.Namespace,
		Status:        generated.HealthStatus_HEALTH_STATUS_HEALTHY,
		Message:       "Instance is running normally",
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	if req.IncludeChecks {
		component.CheckResults = generateMockHealthCheckResults()
	}

	components = append(components, component)

	return &generated.GetHealthResponse{
		Components: components,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: fmt.Sprintf("Retrieved health for %d instances", len(components)),
		},
	}, nil
}

// getNodeHealth retrieves health for a node or nodes.
func (s *HealthService) getNodeHealth(ctx context.Context, req *generated.GetHealthRequest) (*generated.GetHealthResponse, error) {
	// For now, we'll just return a mock response
	// In a real implementation, we would query agents for node health

	var components []*generated.ComponentHealth

	component := &generated.ComponentHealth{
		ComponentType: "node",
		Id:            req.Name, // Using name as ID
		Name:          req.Name,
		Status:        generated.HealthStatus_HEALTH_STATUS_HEALTHY,
		Message:       "Node is operating normally",
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	if req.IncludeChecks {
		component.CheckResults = []*generated.HealthCheckResult{
			{
				Type:                 generated.HealthCheckType_HEALTH_CHECK_TYPE_LIVENESS,
				Status:               generated.HealthStatus_HEALTH_STATUS_HEALTHY,
				Message:              "Node is reachable",
				Timestamp:            time.Now().Format(time.RFC3339),
				ConsecutiveSuccesses: 10,
				ConsecutiveFailures:  0,
			},
			{
				Type:                 generated.HealthCheckType_HEALTH_CHECK_TYPE_READINESS,
				Status:               generated.HealthStatus_HEALTH_STATUS_HEALTHY,
				Message:              "Node has available resources",
				Timestamp:            time.Now().Format(time.RFC3339),
				ConsecutiveSuccesses: 5,
				ConsecutiveFailures:  0,
			},
		}
	}

	components = append(components, component)

	return &generated.GetHealthResponse{
		Components: components,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: fmt.Sprintf("Retrieved health for %d nodes", len(components)),
		},
	}, nil
}

// getAPIServerHealth retrieves health for the API server.
func (s *HealthService) getAPIServerHealth(ctx context.Context, req *generated.GetHealthRequest) (*generated.GetHealthResponse, error) {
	// API server is always healthy if we're here to respond
	component := &generated.ComponentHealth{
		ComponentType: "api-server",
		Id:            "api-server",
		Name:          "api-server",
		Status:        generated.HealthStatus_HEALTH_STATUS_HEALTHY,
		Message:       "API server is operating normally",
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	if req.IncludeChecks {
		component.CheckResults = []*generated.HealthCheckResult{
			{
				Type:                 generated.HealthCheckType_HEALTH_CHECK_TYPE_LIVENESS,
				Status:               generated.HealthStatus_HEALTH_STATUS_HEALTHY,
				Message:              "API server is responding to requests",
				Timestamp:            time.Now().Format(time.RFC3339),
				ConsecutiveSuccesses: 100,
				ConsecutiveFailures:  0,
			},
			{
				Type:                 generated.HealthCheckType_HEALTH_CHECK_TYPE_READINESS,
				Status:               generated.HealthStatus_HEALTH_STATUS_HEALTHY,
				Message:              "API server is ready to process requests",
				Timestamp:            time.Now().Format(time.RFC3339),
				ConsecutiveSuccesses: 100,
				ConsecutiveFailures:  0,
			},
		}
	}

	return &generated.GetHealthResponse{
		Components: []*generated.ComponentHealth{component},
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: "API server is healthy",
		},
	}, nil
}

// computeServiceHealthFromInstances computes the health status of a service from its instances.
func (s *HealthService) computeServiceHealthFromInstances(ctx context.Context, serviceName, namespace string) (generated.HealthStatus, string) {
	// Get all instances for the service
	var instances []types.Instance
	err := s.store.List(ctx, ResourceTypeInstance, namespace, &instances)
	if err != nil {
		s.logger.Error("Failed to list instances", log.Err(err))
		return generated.HealthStatus_HEALTH_STATUS_UNKNOWN, "Failed to retrieve instance data"
	}

	// Count instances by status
	var totalInstances, runningInstances, failedInstances int
	for _, instance := range instances {
		if instance.ServiceID != serviceName {
			continue
		}

		totalInstances++
		if instance.Status == types.InstanceStatusRunning {
			runningInstances++
		} else if instance.Status == types.InstanceStatusFailed {
			failedInstances++
		}
	}

	// Determine overall health
	if totalInstances == 0 {
		return generated.HealthStatus_HEALTH_STATUS_UNKNOWN, "No instances found"
	}

	if runningInstances == totalInstances {
		return generated.HealthStatus_HEALTH_STATUS_HEALTHY, fmt.Sprintf("All %d instances are running", totalInstances)
	}

	if failedInstances == totalInstances {
		return generated.HealthStatus_HEALTH_STATUS_UNHEALTHY, fmt.Sprintf("All %d instances have failed", totalInstances)
	}

	if runningInstances > 0 {
		return generated.HealthStatus_HEALTH_STATUS_DEGRADED,
			fmt.Sprintf("%d/%d instances are running", runningInstances, totalInstances)
	}

	return generated.HealthStatus_HEALTH_STATUS_UNHEALTHY,
		fmt.Sprintf("No instances are running, %d/%d instances have failed", failedInstances, totalInstances)
}

// generateMockHealthCheckResults generates mock health check results for demonstration.
func generateMockHealthCheckResults() []*generated.HealthCheckResult {
	now := time.Now().Format(time.RFC3339)

	return []*generated.HealthCheckResult{
		{
			Type:                 generated.HealthCheckType_HEALTH_CHECK_TYPE_LIVENESS,
			Status:               generated.HealthStatus_HEALTH_STATUS_HEALTHY,
			Message:              "Health check passed",
			Timestamp:            now,
			ConsecutiveSuccesses: 5,
			ConsecutiveFailures:  0,
		},
		{
			Type:                 generated.HealthCheckType_HEALTH_CHECK_TYPE_READINESS,
			Status:               generated.HealthStatus_HEALTH_STATUS_HEALTHY,
			Message:              "Ready to serve traffic",
			Timestamp:            now,
			ConsecutiveSuccesses: 3,
			ConsecutiveFailures:  0,
		},
	}
}
