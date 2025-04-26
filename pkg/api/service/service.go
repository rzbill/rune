package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// DefaultNamespace is the default namespace for services.
	DefaultNamespace = "default"

	// ResourceTypeService is the resource type for services.
	ResourceTypeService = "services"
)

// ServiceService implements the gRPC ServiceService.
type ServiceService struct {
	generated.UnimplementedServiceServiceServer

	store  store.Store
	logger log.Logger
}

// NewServiceService creates a new ServiceService with the given store and logger.
func NewServiceService(store store.Store, logger log.Logger) *ServiceService {
	return &ServiceService{
		store:  store,
		logger: logger.WithComponent("service-service"),
	}
}

// CreateService creates a new service.
func (s *ServiceService) CreateService(ctx context.Context, req *generated.CreateServiceRequest) (*generated.ServiceResponse, error) {
	s.logger.Debug("CreateService called")

	if req.Service == nil {
		return nil, status.Error(codes.InvalidArgument, "service is required")
	}

	// Convert protobuf message to domain model
	service, err := s.protoToServiceModel(req.Service)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid service: %v", err)
	}

	// Generate service ID if not provided
	if service.ID == "" {
		service.ID = uuid.New().String()
	}

	// Set namespace to default if not provided
	if service.Namespace == "" {
		service.Namespace = DefaultNamespace
	}

	// Set creation time
	now := time.Now()
	service.CreatedAt = now
	service.UpdatedAt = now

	// Set initial status
	service.Status = types.ServiceStatusPending

	// Store the service
	if err := s.store.Create(ctx, ResourceTypeService, service.Namespace, service.Name, service); err != nil {
		s.logger.Error("Failed to create service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to create service: %v", err)
	}

	// Convert back to protobuf message
	protoService, err := s.serviceModelToProto(service)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert service to proto: %v", err)
	}

	return &generated.ServiceResponse{
		Service: protoService,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: "Service created successfully",
		},
	}, nil
}

// GetService retrieves a service by name.
func (s *ServiceService) GetService(ctx context.Context, req *generated.GetServiceRequest) (*generated.ServiceResponse, error) {
	s.logger.Debug("GetService called", log.Str("name", req.Name))

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Get the service from the store
	var service types.Service
	if err := s.store.Get(ctx, ResourceTypeService, namespace, req.Name, &service); err != nil {
		if IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Name)
		}
		s.logger.Error("Failed to get service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
	}

	// Convert to protobuf message
	protoService, err := s.serviceModelToProto(&service)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert service to proto: %v", err)
	}

	return &generated.ServiceResponse{
		Service: protoService,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: "Service retrieved successfully",
		},
	}, nil
}

// ListServices lists services with optional filtering.
func (s *ServiceService) ListServices(ctx context.Context, req *generated.ListServicesRequest) (*generated.ListServicesResponse, error) {
	s.logger.Debug("ListServices called")

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Get services from the store
	services, err := s.store.List(ctx, ResourceTypeService, namespace)
	if err != nil {
		s.logger.Error("Failed to list services", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to list services: %v", err)
	}

	// Convert to protobuf messages
	protoServices := make([]*generated.Service, 0, len(services))
	for _, svc := range services {
		service, ok := svc.(*types.Service)
		if !ok {
			continue
		}

		// Apply label selector filter if provided
		if len(req.LabelSelector) > 0 {
			// TODO: Implement label selector filtering
		}

		protoService, err := s.serviceModelToProto(service)
		if err != nil {
			s.logger.Error("Failed to convert service to proto", log.Err(err))
			continue
		}

		protoServices = append(protoServices, protoService)
	}

	return &generated.ListServicesResponse{
		Services: protoServices,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: fmt.Sprintf("Found %d services", len(protoServices)),
		},
		Paging: &generated.PagingParams{
			Limit:  int32(len(protoServices)),
			Offset: 0,
		},
	}, nil
}

// UpdateService updates an existing service.
func (s *ServiceService) UpdateService(ctx context.Context, req *generated.UpdateServiceRequest) (*generated.ServiceResponse, error) {
	s.logger.Debug("UpdateService called")

	if req.Service == nil {
		return nil, status.Error(codes.InvalidArgument, "service is required")
	}

	namespace := req.Service.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Check if the service exists
	var existingService types.Service
	err := s.store.Get(ctx, ResourceTypeService, namespace, req.Service.Name, &existingService)
	if err != nil {
		if IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Service.Name)
		}
		s.logger.Error("Failed to get service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
	}

	// Convert protobuf message to domain model
	updatedService, err := s.protoToServiceModel(req.Service)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid service: %v", err)
	}

	// Preserve the ID, creation time, and other fields that shouldn't change
	updatedService.ID = existingService.ID
	updatedService.CreatedAt = existingService.CreatedAt
	updatedService.UpdatedAt = time.Now()
	updatedService.Status = types.ServiceStatusUpdating

	// Store the updated service
	if err := s.store.Update(ctx, ResourceTypeService, namespace, req.Service.Name, updatedService); err != nil {
		s.logger.Error("Failed to update service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to update service: %v", err)
	}

	// Convert back to protobuf message
	protoService, err := s.serviceModelToProto(updatedService)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert service to proto: %v", err)
	}

	return &generated.ServiceResponse{
		Service: protoService,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: "Service updated successfully",
		},
	}, nil
}

// DeleteService removes a service.
func (s *ServiceService) DeleteService(ctx context.Context, req *generated.DeleteServiceRequest) (*generated.DeleteServiceResponse, error) {
	s.logger.Debug("DeleteService called", log.Str("name", req.Name))

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Check if the service exists
	var existingService types.Service
	err := s.store.Get(ctx, ResourceTypeService, namespace, req.Name, &existingService)
	if err != nil {
		if IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Name)
		}
		s.logger.Error("Failed to get service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
	}

	// TODO: Check if service has instances and force parameter is provided

	// Delete the service
	if err := s.store.Delete(ctx, ResourceTypeService, namespace, req.Name); err != nil {
		s.logger.Error("Failed to delete service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to delete service: %v", err)
	}

	return &generated.DeleteServiceResponse{
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: "Service deleted successfully",
		},
	}, nil
}

// ScaleService changes the scale of a service.
func (s *ServiceService) ScaleService(ctx context.Context, req *generated.ScaleServiceRequest) (*generated.ServiceResponse, error) {
	s.logger.Debug("ScaleService called", log.Str("name", req.Name), log.Int("scale", int(req.Scale)))

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	if req.Scale < 0 {
		return nil, status.Error(codes.InvalidArgument, "scale must be a non-negative integer")
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Get the service from the store
	var service types.Service
	if err := s.store.Get(ctx, ResourceTypeService, namespace, req.Name, &service); err != nil {
		if IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Name)
		}
		s.logger.Error("Failed to get service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
	}

	// Update the scale
	service.Scale = int(req.Scale)
	service.UpdatedAt = time.Now()

	// Store the updated service
	if err := s.store.Update(ctx, ResourceTypeService, namespace, req.Name, &service); err != nil {
		s.logger.Error("Failed to update service scale", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to update service scale: %v", err)
	}

	// Convert to protobuf message
	protoService, err := s.serviceModelToProto(&service)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert service to proto: %v", err)
	}

	return &generated.ServiceResponse{
		Service: protoService,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: fmt.Sprintf("Service scaled to %d instances", req.Scale),
		},
	}, nil
}

// serviceModelToProto converts a domain model service to a protobuf message.
func (s *ServiceService) serviceModelToProto(service *types.Service) (*generated.Service, error) {
	if service == nil {
		return nil, fmt.Errorf("service is nil")
	}

	protoService := &generated.Service{
		Id:        service.ID,
		Name:      service.Name,
		Namespace: service.Namespace,
		Image:     service.Image,
		Command:   service.Command,
		Scale:     int32(service.Scale),
		CreatedAt: service.CreatedAt.Format(time.RFC3339),
		UpdatedAt: service.UpdatedAt.Format(time.RFC3339),
		Runtime:   service.Runtime,
	}

	// Convert args
	if len(service.Args) > 0 {
		protoService.Args = make([]string, len(service.Args))
		copy(protoService.Args, service.Args)
	}

	// Convert environment variables
	if len(service.Env) > 0 {
		protoService.Env = make(map[string]string)
		for k, v := range service.Env {
			protoService.Env[k] = v
		}
	}

	// Convert ports
	if len(service.Ports) > 0 {
		protoService.Ports = make([]*generated.ServicePort, len(service.Ports))
		for i, port := range service.Ports {
			protoService.Ports[i] = &generated.ServicePort{
				Name:       port.Name,
				Port:       int32(port.Port),
				TargetPort: int32(port.TargetPort),
				Protocol:   port.Protocol,
			}
		}
	}

	// Convert resources
	if service.Resources != (types.Resources{}) {
		protoService.Resources = &generated.Resources{
			Cpu: &generated.ResourceLimit{
				Request: service.Resources.CPU.Request,
				Limit:   service.Resources.CPU.Limit,
			},
			Memory: &generated.ResourceLimit{
				Request: service.Resources.Memory.Request,
				Limit:   service.Resources.Memory.Limit,
			},
		}
	}

	// Convert status
	switch service.Status {
	case types.ServiceStatusPending:
		protoService.Status = generated.ServiceStatus_SERVICE_STATUS_PENDING
	case types.ServiceStatusRunning:
		protoService.Status = generated.ServiceStatus_SERVICE_STATUS_RUNNING
	case types.ServiceStatusUpdating:
		protoService.Status = generated.ServiceStatus_SERVICE_STATUS_UPDATING
	case types.ServiceStatusFailed:
		protoService.Status = generated.ServiceStatus_SERVICE_STATUS_FAILED
	default:
		protoService.Status = generated.ServiceStatus_SERVICE_STATUS_UNSPECIFIED
	}

	// Convert health checks
	if service.Health != nil {
		protoService.Health = &generated.HealthCheck{}

		if service.Health.Liveness != nil {
			protoService.Health.Liveness = &generated.Probe{
				InitialDelaySeconds: int32(service.Health.Liveness.InitialDelaySeconds),
				PeriodSeconds:       int32(service.Health.Liveness.IntervalSeconds),
				TimeoutSeconds:      int32(service.Health.Liveness.TimeoutSeconds),
			}

			switch service.Health.Liveness.Type {
			case "http":
				protoService.Health.Liveness.Type = generated.ProbeType_PROBE_TYPE_HTTP
				protoService.Health.Liveness.Path = service.Health.Liveness.Path
				protoService.Health.Liveness.Port = int32(service.Health.Liveness.Port)
			case "tcp":
				protoService.Health.Liveness.Type = generated.ProbeType_PROBE_TYPE_TCP
				protoService.Health.Liveness.Port = int32(service.Health.Liveness.Port)
			case "command":
				protoService.Health.Liveness.Type = generated.ProbeType_PROBE_TYPE_COMMAND
				protoService.Health.Liveness.Command = service.Health.Liveness.Command
			}
		}

		if service.Health.Readiness != nil {
			protoService.Health.Readiness = &generated.Probe{
				InitialDelaySeconds: int32(service.Health.Readiness.InitialDelaySeconds),
				PeriodSeconds:       int32(service.Health.Readiness.IntervalSeconds),
				TimeoutSeconds:      int32(service.Health.Readiness.TimeoutSeconds),
			}

			switch service.Health.Readiness.Type {
			case "http":
				protoService.Health.Readiness.Type = generated.ProbeType_PROBE_TYPE_HTTP
				protoService.Health.Readiness.Path = service.Health.Readiness.Path
				protoService.Health.Readiness.Port = int32(service.Health.Readiness.Port)
			case "tcp":
				protoService.Health.Readiness.Type = generated.ProbeType_PROBE_TYPE_TCP
				protoService.Health.Readiness.Port = int32(service.Health.Readiness.Port)
			case "command":
				protoService.Health.Readiness.Type = generated.ProbeType_PROBE_TYPE_COMMAND
				protoService.Health.Readiness.Command = service.Health.Readiness.Command
			}
		}
	}

	return protoService, nil
}

// protoToServiceModel converts a protobuf message to a domain model service.
func (s *ServiceService) protoToServiceModel(proto *generated.Service) (*types.Service, error) {
	if proto == nil {
		return nil, fmt.Errorf("proto service is nil")
	}

	service := &types.Service{
		ID:        proto.Id,
		Name:      proto.Name,
		Namespace: proto.Namespace,
		Image:     proto.Image,
		Command:   proto.Command,
		Scale:     int(proto.Scale),
		Runtime:   proto.Runtime,
	}

	// Convert args
	if len(proto.Args) > 0 {
		service.Args = make([]string, len(proto.Args))
		copy(service.Args, proto.Args)
	}

	// Convert environment variables
	if len(proto.Env) > 0 {
		service.Env = make(map[string]string)
		for k, v := range proto.Env {
			service.Env[k] = v
		}
	}

	// Convert ports
	if len(proto.Ports) > 0 {
		service.Ports = make([]types.ServicePort, len(proto.Ports))
		for i, port := range proto.Ports {
			service.Ports[i] = types.ServicePort{
				Name:       port.Name,
				Port:       int(port.Port),
				TargetPort: int(port.TargetPort),
				Protocol:   port.Protocol,
			}
		}
	}

	// Convert resources
	if proto.Resources != nil {
		if proto.Resources.Cpu != nil {
			service.Resources.CPU = types.ResourceLimit{
				Request: proto.Resources.Cpu.Request,
				Limit:   proto.Resources.Cpu.Limit,
			}
		}
		if proto.Resources.Memory != nil {
			service.Resources.Memory = types.ResourceLimit{
				Request: proto.Resources.Memory.Request,
				Limit:   proto.Resources.Memory.Limit,
			}
		}
	}

	// Convert status
	switch proto.Status {
	case generated.ServiceStatus_SERVICE_STATUS_PENDING:
		service.Status = types.ServiceStatusPending
	case generated.ServiceStatus_SERVICE_STATUS_RUNNING:
		service.Status = types.ServiceStatusRunning
	case generated.ServiceStatus_SERVICE_STATUS_UPDATING:
		service.Status = types.ServiceStatusUpdating
	case generated.ServiceStatus_SERVICE_STATUS_FAILED:
		service.Status = types.ServiceStatusFailed
	default:
		service.Status = types.ServiceStatusPending
	}

	// Convert health check
	if proto.Health != nil {
		service.Health = &types.HealthCheck{}

		if proto.Health.Liveness != nil {
			service.Health.Liveness = &types.Probe{
				InitialDelaySeconds: int(proto.Health.Liveness.InitialDelaySeconds),
				IntervalSeconds:     int(proto.Health.Liveness.PeriodSeconds),
				TimeoutSeconds:      int(proto.Health.Liveness.TimeoutSeconds),
			}

			switch proto.Health.Liveness.Type {
			case generated.ProbeType_PROBE_TYPE_HTTP:
				service.Health.Liveness.Type = "http"
				service.Health.Liveness.Path = proto.Health.Liveness.Path
				service.Health.Liveness.Port = int(proto.Health.Liveness.Port)
			case generated.ProbeType_PROBE_TYPE_TCP:
				service.Health.Liveness.Type = "tcp"
				service.Health.Liveness.Port = int(proto.Health.Liveness.Port)
			case generated.ProbeType_PROBE_TYPE_COMMAND:
				service.Health.Liveness.Type = "command"
				service.Health.Liveness.Command = proto.Health.Liveness.Command
			}
		}

		if proto.Health.Readiness != nil {
			service.Health.Readiness = &types.Probe{
				InitialDelaySeconds: int(proto.Health.Readiness.InitialDelaySeconds),
				IntervalSeconds:     int(proto.Health.Readiness.PeriodSeconds),
				TimeoutSeconds:      int(proto.Health.Readiness.TimeoutSeconds),
			}

			switch proto.Health.Readiness.Type {
			case generated.ProbeType_PROBE_TYPE_HTTP:
				service.Health.Readiness.Type = "http"
				service.Health.Readiness.Path = proto.Health.Readiness.Path
				service.Health.Readiness.Port = int(proto.Health.Readiness.Port)
			case generated.ProbeType_PROBE_TYPE_TCP:
				service.Health.Readiness.Type = "tcp"
				service.Health.Readiness.Port = int(proto.Health.Readiness.Port)
			case generated.ProbeType_PROBE_TYPE_COMMAND:
				service.Health.Readiness.Type = "command"
				service.Health.Readiness.Command = proto.Health.Readiness.Command
			}
		}
	}

	return service, nil
}

// IsNotFound returns true if the error is a "not found" error.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Check if the error message contains "not found"
	return strings.Contains(err.Error(), "not found")
}
