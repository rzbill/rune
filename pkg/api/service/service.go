package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
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

	//Set Metadata with initial generation, creation and update times
	now := time.Now()
	service.Metadata = &types.ServiceMetadata{
		Generation: 1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Set initial status
	service.Status = types.ServiceStatusPending

	// Store the service
	if err := s.store.Create(ctx, types.ResourceTypeService, service.Namespace, service.Name, service); err != nil {
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
		return nil, status.Error(codes.InvalidArgument, "2service name is required")
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Get the service from the store
	var service types.Service
	s.logger.Debug("Getting service", log.Str("namespace", namespace), log.Str("name", req.Name))
	if err := s.store.Get(ctx, types.ResourceTypeService, namespace, req.Name, &service); err != nil {
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

	var services []types.Service
	var err error

	// Check if we need to list across all namespaces
	if namespace == "*" {
		// For all namespaces, we need to list each namespace separately
		// Get all namespaces first (this would typically come from a namespace store)
		// TODO: For now, we'll just query all resources directly without namespace filtering
		s.logger.Debug("Listing services across all namespaces")
		err = s.store.List(ctx, types.ResourceTypeService, "", &services)
	} else {
		// Get services from the store for a specific namespace
		err = s.store.List(ctx, types.ResourceTypeService, namespace, &services)
	}

	if err != nil {
		s.logger.Error("Failed to list services", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to list services: %v", err)
	}

	s.logger.Debug("Found services", log.Int("count", len(services)))

	// Convert to protobuf messages
	protoServices := make([]*generated.Service, 0, len(services))
	for _, service := range services {
		// Apply selector filtering (both labels and fields)
		if !matchSelectors(&service, req.LabelSelector, req.FieldSelector) {
			continue
		}

		protoService, err := s.serviceModelToProto(&service)
		if err != nil {
			s.logger.Error("Failed to convert service to proto", log.Err(err))
			continue
		}

		protoServices = append(protoServices, protoService)
	}

	s.logger.Debug("Found proto services", log.Int("count", len(protoServices)))

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

// matchSelectors checks if service matches all the labels and fields in the selectors
func matchSelectors(service *types.Service, labels map[string]string, fields map[string]string) bool {
	// Check label selectors
	if len(labels) > 0 {
		// Check if the service has all the requested labels
		for key, value := range labels {
			// If service has no labels or the specific label is not found
			if service.Labels == nil {
				return false
			}

			serviceValue, exists := service.Labels[key]
			if !exists || serviceValue != value {
				return false
			}
		}
	}

	// Check field selectors
	for k, v := range fields {
		switch k {
		case "name":
			if service.Name != v {
				return false
			}
		case "namespace":
			if service.Namespace != v {
				return false
			}
		case "status":
			if string(service.Status) != v {
				return false
			}
		case "runtime":
			if string(service.Runtime) != v {
				return false
			}
		default:
			// Unknown field, consider it a non-match
			return false
		}
	}

	// Match if we passed all checks
	return true
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
	err := s.store.Get(ctx, types.ResourceTypeService, namespace, req.Service.Name, &existingService)
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
	updatedService.Metadata = existingService.Metadata
	updatedService.Metadata.UpdatedAt = time.Now()

	// Determine if we need to increment the generation
	needsGenUpdate := req.Force // Force flag forces generation update

	if !needsGenUpdate {
		// Calculate service hash to determine if anything meaningful changed
		oldHash := calculateServiceHash(&existingService)
		newHash := calculateServiceHash(updatedService)

		if oldHash != newHash {
			// Service has changed, increment generation
			needsGenUpdate = true
			s.logger.Debug("Service hash changed, incrementing generation",
				log.Str("service", updatedService.Name),
				log.Str("namespace", updatedService.Namespace),
				log.Str("old_hash", oldHash[:8]),
				log.Str("new_hash", newHash[:8]),
				log.Int64("from_generation", existingService.Metadata.Generation),
				log.Int64("to_generation", existingService.Metadata.Generation+1))
		} else {
			// No changes detected, keep existing generation
			s.logger.Debug("Service unchanged, keeping generation",
				log.Str("service", updatedService.Name),
				log.Str("namespace", updatedService.Namespace),
				log.Str("hash", oldHash[:8]),
				log.Int64("generation", existingService.Metadata.Generation))
		}
	} else {
		s.logger.Info("Force flag set, incrementing generation",
			log.Str("service", updatedService.Name),
			log.Str("namespace", updatedService.Namespace),
			log.Int64("from_generation", existingService.Metadata.Generation),
			log.Int64("to_generation", existingService.Metadata.Generation+1))
	}

	if needsGenUpdate {
		// Increment generation to trigger reconciliation
		updatedService.Metadata.Generation = existingService.Metadata.Generation + 1
		updatedService.Status = types.ServiceStatusDeploying
	} else {
		// Keep existing generation and status
		updatedService.Metadata.Generation = existingService.Metadata.Generation
		updatedService.Status = existingService.Status
	}

	// Store the updated service
	if err := s.store.Update(ctx, types.ResourceTypeService, namespace, req.Service.Name, updatedService); err != nil {
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

// calculateServiceHash generates a hash of service properties that should trigger reconciliation when changed
func calculateServiceHash(service *types.Service) string {
	h := sha256.New()

	// Include only fields that should trigger a reconciliation when changed
	fmt.Fprintf(h, "image:%s\n", service.Image)
	fmt.Fprintf(h, "command:%s\n", service.Command)
	fmt.Fprintf(h, "scale:%d\n", service.Scale)
	fmt.Fprintf(h, "runtime:%s\n", string(service.Runtime))

	// Args
	fmt.Fprintf(h, "args:[")
	for i, arg := range service.Args {
		if i > 0 {
			fmt.Fprintf(h, ",")
		}
		fmt.Fprintf(h, "%s", arg)
	}
	fmt.Fprintf(h, "]\n")

	// Environment variables
	var envKeys []string
	for k := range service.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	fmt.Fprintf(h, "env:{")
	for i, k := range envKeys {
		if i > 0 {
			fmt.Fprintf(h, ",")
		}
		fmt.Fprintf(h, "%s:%s", k, service.Env[k])
	}
	fmt.Fprintf(h, "}\n")

	// Ports
	fmt.Fprintf(h, "ports:[")
	for i, port := range service.Ports {
		if i > 0 {
			fmt.Fprintf(h, ",")
		}
		fmt.Fprintf(h, "%s:%d:%d:%s", port.Name, port.Port, port.TargetPort, port.Protocol)
	}
	fmt.Fprintf(h, "]\n")

	// Resources
	if service.Resources.CPU.Request != "" || service.Resources.CPU.Limit != "" {
		fmt.Fprintf(h, "cpu:%s:%s\n", service.Resources.CPU.Request, service.Resources.CPU.Limit)
	}

	if service.Resources.Memory.Request != "" || service.Resources.Memory.Limit != "" {
		fmt.Fprintf(h, "memory:%s:%s\n", service.Resources.Memory.Request, service.Resources.Memory.Limit)
	}

	// Add more fields as needed that should trigger reconciliation when changed

	return fmt.Sprintf("%x", h.Sum(nil))
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
	err := s.store.Get(ctx, types.ResourceTypeService, namespace, req.Name, &existingService)
	if err != nil {
		if IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Name)
		}
		s.logger.Error("Failed to get service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
	}

	// TODO: Check if service has instances and force parameter is provided

	// Delete the service
	if err := s.store.Delete(ctx, types.ResourceTypeService, namespace, req.Name); err != nil {
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
	if err := s.store.Get(ctx, types.ResourceTypeService, namespace, req.Name, &service); err != nil {
		if IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "service not found: %s", req.Name)
		}
		s.logger.Error("Failed to get service", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to get service: %v", err)
	}

	// Update the scale
	service.Scale = int(req.Scale)
	service.Metadata.UpdatedAt = time.Now()

	// Store the updated service
	if err := s.store.Update(ctx, types.ResourceTypeService, namespace, req.Name, &service); err != nil {
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
		Metadata: &generated.ServiceMetadata{
			Generation: int32(service.Metadata.Generation),
			CreatedAt:  service.Metadata.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  service.Metadata.UpdatedAt.Format(time.RFC3339),
		},
		Runtime: string(service.Runtime),
	}

	// Convert labels
	if len(service.Labels) > 0 {
		protoService.Labels = make(map[string]string)
		for k, v := range service.Labels {
			protoService.Labels[k] = v
		}
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
	case types.ServiceStatusDeploying:
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
		Runtime:   types.RuntimeType(proto.Runtime),
	}

	// Convert labels
	if len(proto.Labels) > 0 {
		service.Labels = make(map[string]string)
		for k, v := range proto.Labels {
			service.Labels[k] = v
		}
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
		service.Status = types.ServiceStatusDeploying
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

// WatchServices watches services for changes.
func (s *ServiceService) WatchServices(req *generated.WatchServicesRequest, stream generated.ServiceService_WatchServicesServer) error {
	s.logger.Debug("WatchServices called",
		log.Str("namespace", req.Namespace),
		log.Int("labelSelector", len(req.LabelSelector)),
		log.Int("fieldSelector", len(req.FieldSelector)))

	ctx := stream.Context()

	// Set default namespace if not specified
	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Start watching for service changes from the store
	watchCh, err := s.store.Watch(ctx, types.ResourceTypeService, namespace)
	if err != nil {
		s.logger.Error("Failed to watch services", log.Err(err))
		return status.Errorf(codes.Internal, "failed to watch services: %v", err)
	}

	// Initialize with current services (simulating ADDED events for all existing services)
	var services []types.Service
	err = s.store.List(ctx, types.ResourceTypeService, namespace, &services)
	if err != nil {
		s.logger.Error("Failed to list services", log.Err(err))
		return status.Errorf(codes.Internal, "failed to list initial services: %v", err)
	}

	// Send all existing services as ADDED events
	for _, service := range services {

		// Apply selector filtering
		if !matchSelectors(&service, req.LabelSelector, req.FieldSelector) {
			continue
		}

		// Convert to proto and send to client
		protoService, err := s.serviceModelToProto(&service)
		if err != nil {
			s.logger.Error("Failed to convert service to proto", log.Err(err))
			continue
		}

		// Send to client
		err = stream.Send(&generated.WatchServicesResponse{
			Service:   protoService,
			EventType: generated.EventType_EVENT_TYPE_ADDED,
			Status:    &generated.Status{Code: int32(codes.OK)},
		})
		if err != nil {
			s.logger.Error("Failed to send initial service", log.Err(err))
			return status.Errorf(codes.Internal, "failed to send initial service: %v", err)
		}
	}

	// Watch loop - continue until client disconnects or context is cancelled
	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("Watch context cancelled")
			return nil

		case event, ok := <-watchCh:
			if !ok {
				s.logger.Debug("Watch channel closed")
				return nil
			}

			// Only handle events for services
			if event.ResourceType != types.ResourceTypeService {
				continue
			}

			// Convert to typed service
			var service types.Service
			if typedService, ok := event.Resource.(*types.Service); ok {
				service = *typedService
			} else {
				// Use JSON marshaling/unmarshaling for conversion
				rawData, err := json.Marshal(event.Resource)
				if err != nil {
					s.logger.Warn("Failed to marshal service data", log.Err(err))
					continue
				}

				if err := json.Unmarshal(rawData, &service); err != nil {
					s.logger.Warn("Failed to unmarshal service data", log.Err(err))
					continue
				}
			}

			// Apply selector filtering
			if !matchSelectors(&service, req.LabelSelector, req.FieldSelector) {
				continue
			}

			// Convert to proto
			protoService, err := s.serviceModelToProto(&service)
			if err != nil {
				s.logger.Error("Failed to convert service to proto", log.Err(err))
				continue
			}

			// Map store event type to proto event type
			var eventType generated.EventType
			switch event.Type {
			case store.WatchEventCreated:
				eventType = generated.EventType_EVENT_TYPE_ADDED
			case store.WatchEventUpdated:
				eventType = generated.EventType_EVENT_TYPE_MODIFIED
			case store.WatchEventDeleted:
				eventType = generated.EventType_EVENT_TYPE_DELETED
			default:
				eventType = generated.EventType_EVENT_TYPE_UNSPECIFIED
			}

			// Send to client
			err = stream.Send(&generated.WatchServicesResponse{
				Service:   protoService,
				EventType: eventType,
				Status:    &generated.Status{Code: int32(codes.OK)},
			})
			if err != nil {
				s.logger.Error("Failed to send watch event", log.Err(err))
				return status.Errorf(codes.Internal, "failed to send watch event: %v", err)
			}
		}
	}
}
