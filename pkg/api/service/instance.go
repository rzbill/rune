package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ResourceTypeInstance is the resource type for instances.
	ResourceTypeInstance = "instances"
)

// InstanceService implements the gRPC InstanceService.
type InstanceService struct {
	generated.UnimplementedInstanceServiceServer

	store         store.Store
	dockerRunner  runner.Runner
	processRunner runner.Runner
	logger        log.Logger
}

// NewInstanceService creates a new InstanceService with the given store, runners, and logger.
func NewInstanceService(store store.Store, dockerRunner, processRunner runner.Runner, logger log.Logger) *InstanceService {
	return &InstanceService{
		store:         store,
		dockerRunner:  dockerRunner,
		processRunner: runner.Runner(processRunner),
		logger:        logger.WithComponent("instance-service"),
	}
}

// GetInstance retrieves an instance by ID.
func (s *InstanceService) GetInstance(ctx context.Context, req *generated.GetInstanceRequest) (*generated.InstanceResponse, error) {
	s.logger.Debug("GetInstance called", log.Str("id", req.Id))

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "instance ID is required")
	}

	// Get the instance from the store - we'll need to search all namespaces
	// This is a simplification - in a production system you might have an index for instance IDs
	var instance *types.Instance
	namespaces, err := s.store.List(ctx, "namespaces", "")
	if err != nil {
		s.logger.Error("Failed to list namespaces", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to list namespaces: %v", err)
	}

	// If no namespaces, use the default
	if len(namespaces) == 0 {
		// Look for the instance in the default namespace
		instances, err := s.store.List(ctx, ResourceTypeInstance, DefaultNamespace)
		if err != nil {
			s.logger.Error("Failed to list instances", log.Err(err))
			return nil, status.Errorf(codes.Internal, "failed to list instances: %v", err)
		}

		for _, inst := range instances {
			i, ok := inst.(*types.Instance)
			if !ok {
				continue
			}

			if i.ID == req.Id {
				instance = i
				break
			}
		}
	} else {
		// Look through all namespaces
		for _, ns := range namespaces {
			namespace, ok := ns.(string)
			if !ok {
				continue
			}

			instances, err := s.store.List(ctx, ResourceTypeInstance, namespace)
			if err != nil {
				s.logger.Error("Failed to list instances", log.Str("namespace", namespace), log.Err(err))
				continue
			}

			for _, inst := range instances {
				i, ok := inst.(*types.Instance)
				if !ok {
					continue
				}

				if i.ID == req.Id {
					instance = i
					break
				}
			}

			if instance != nil {
				break
			}
		}
	}

	if instance == nil {
		return nil, status.Errorf(codes.NotFound, "instance not found: %s", req.Id)
	}

	// Get instance status from the appropriate runner
	var status types.InstanceStatus
	var statusErr error

	if instance.ContainerID != "" {
		if s.dockerRunner != nil {
			status, statusErr = s.dockerRunner.Status(ctx, instance.ID)
		} else {
			statusErr = fmt.Errorf("docker runner not available")
		}
	} else {
		if s.processRunner != nil {
			status, statusErr = s.processRunner.Status(ctx, instance.ID)
		} else {
			statusErr = fmt.Errorf("process runner not available")
		}
	}

	if statusErr != nil {
		s.logger.Warn("Failed to get instance status", log.Str("id", req.Id), log.Err(statusErr))
		// We'll continue with the stored status, just log the error
	} else {
		// Update the instance status in our response
		instance.Status = status
	}

	// Convert to protobuf message
	protoInstance, err := s.instanceModelToProto(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to convert instance to proto: %v", err)
	}

	return &generated.InstanceResponse{
		Instance: protoInstance,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: "Instance retrieved successfully",
		},
	}, nil
}

// ListInstances lists instances with optional filtering.
func (s *InstanceService) ListInstances(ctx context.Context, req *generated.ListInstancesRequest) (*generated.ListInstancesResponse, error) {
	s.logger.Debug("ListInstances called")

	namespace := req.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// Get instances from the store
	storeInstances, err := s.store.List(ctx, ResourceTypeInstance, namespace)
	if err != nil {
		s.logger.Error("Failed to list instances", log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to list instances: %v", err)
	}

	// Convert to domain model instances
	instances := make([]*types.Instance, 0, len(storeInstances))
	for _, inst := range storeInstances {
		instance, ok := inst.(*types.Instance)
		if !ok {
			continue
		}

		// Apply service name filter if provided
		if req.ServiceName != "" && instance.ServiceID != req.ServiceName {
			continue
		}

		// Apply node ID filter if provided
		if req.NodeId != "" && instance.NodeID != req.NodeId {
			continue
		}

		// Apply status filter if provided
		if req.Status != generated.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED {
			protoStatus := s.instanceStatusToProto(instance.Status)
			if protoStatus != req.Status {
				continue
			}
		}

		instances = append(instances, instance)
	}

	// Convert to protobuf messages
	protoInstances := make([]*generated.Instance, 0, len(instances))
	for _, instance := range instances {
		// Get instance status from the appropriate runner
		var status types.InstanceStatus
		var statusErr error

		if instance.ContainerID != "" {
			if s.dockerRunner != nil {
				status, statusErr = s.dockerRunner.Status(ctx, instance.ID)
			}
		} else {
			if s.processRunner != nil {
				status, statusErr = s.processRunner.Status(ctx, instance.ID)
			}
		}

		if statusErr == nil {
			// Update the instance status
			instance.Status = status
		}

		protoInstance, err := s.instanceModelToProto(instance)
		if err != nil {
			s.logger.Error("Failed to convert instance to proto", log.Err(err))
			continue
		}

		protoInstances = append(protoInstances, protoInstance)
	}

	return &generated.ListInstancesResponse{
		Instances: protoInstances,
		Status: &generated.Status{
			Code:    int32(codes.OK),
			Message: fmt.Sprintf("Found %d instances", len(protoInstances)),
		},
		Paging: &generated.PagingParams{
			Limit:  int32(len(protoInstances)),
			Offset: 0,
		},
	}, nil
}

// StartInstance starts an instance.
func (s *InstanceService) StartInstance(ctx context.Context, req *generated.InstanceActionRequest) (*generated.InstanceResponse, error) {
	s.logger.Debug("StartInstance called", log.Str("id", req.Id))

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "instance ID is required")
	}

	// Get the instance
	instanceResp, err := s.GetInstance(ctx, &generated.GetInstanceRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	// Determine the appropriate runner
	var runnerToUse runner.Runner
	if instanceResp.Instance.ContainerId != "" {
		if s.dockerRunner == nil {
			return nil, status.Error(codes.Unavailable, "docker runner not available")
		}
		runnerToUse = s.dockerRunner
	} else {
		if s.processRunner == nil {
			return nil, status.Error(codes.Unavailable, "process runner not available")
		}
		runnerToUse = s.processRunner
	}

	// Start the instance
	if err := runnerToUse.Start(ctx, req.Id); err != nil {
		s.logger.Error("Failed to start instance", log.Str("id", req.Id), log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to start instance: %v", err)
	}

	// Get the updated instance
	return s.GetInstance(ctx, &generated.GetInstanceRequest{Id: req.Id})
}

// StopInstance stops an instance.
func (s *InstanceService) StopInstance(ctx context.Context, req *generated.InstanceActionRequest) (*generated.InstanceResponse, error) {
	s.logger.Debug("StopInstance called", log.Str("id", req.Id))

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "instance ID is required")
	}

	// Get the instance
	instanceResp, err := s.GetInstance(ctx, &generated.GetInstanceRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	// Determine the appropriate runner
	var runnerToUse runner.Runner
	if instanceResp.Instance.ContainerId != "" {
		if s.dockerRunner == nil {
			return nil, status.Error(codes.Unavailable, "docker runner not available")
		}
		runnerToUse = s.dockerRunner
	} else {
		if s.processRunner == nil {
			return nil, status.Error(codes.Unavailable, "process runner not available")
		}
		runnerToUse = s.processRunner
	}

	// Set timeout
	timeout := time.Duration(30) * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	// Stop the instance
	if err := runnerToUse.Stop(ctx, req.Id, timeout); err != nil {
		s.logger.Error("Failed to stop instance", log.Str("id", req.Id), log.Err(err))
		return nil, status.Errorf(codes.Internal, "failed to stop instance: %v", err)
	}

	// Get the updated instance
	return s.GetInstance(ctx, &generated.GetInstanceRequest{Id: req.Id})
}

// RestartInstance restarts an instance.
func (s *InstanceService) RestartInstance(ctx context.Context, req *generated.InstanceActionRequest) (*generated.InstanceResponse, error) {
	s.logger.Debug("RestartInstance called", log.Str("id", req.Id))

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "instance ID is required")
	}

	// First stop the instance
	_, err := s.StopInstance(ctx, req)
	if err != nil {
		return nil, err
	}

	// Then start it again
	return s.StartInstance(ctx, req)
}

// instanceModelToProto converts a domain model instance to a protobuf message.
func (s *InstanceService) instanceModelToProto(instance *types.Instance) (*generated.Instance, error) {
	if instance == nil {
		return nil, fmt.Errorf("instance is nil")
	}

	protoInstance := &generated.Instance{
		Id:            instance.ID,
		Name:          instance.Name,
		ServiceId:     instance.ServiceID,
		NodeId:        instance.NodeID,
		Ip:            instance.IP,
		StatusMessage: instance.StatusMessage,
		ContainerId:   instance.ContainerID,
		Pid:           int32(instance.PID),
		CreatedAt:     instance.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     instance.UpdatedAt.Format(time.RFC3339),
	}

	// Convert status
	protoInstance.Status = s.instanceStatusToProto(instance.Status)

	// Convert resources
	if instance.Resources != nil {
		protoInstance.Resources = &generated.Resources{
			Cpu: &generated.ResourceLimit{
				Request: instance.Resources.CPU.Request,
				Limit:   instance.Resources.CPU.Limit,
			},
			Memory: &generated.ResourceLimit{
				Request: instance.Resources.Memory.Request,
				Limit:   instance.Resources.Memory.Limit,
			},
		}
	}

	return protoInstance, nil
}

// instanceStatusToProto converts a domain model instance status to a protobuf enum value.
func (s *InstanceService) instanceStatusToProto(status types.InstanceStatus) generated.InstanceStatus {
	switch status {
	case types.InstanceStatusPending:
		return generated.InstanceStatus_INSTANCE_STATUS_PENDING
	case types.InstanceStatusCreated:
		return generated.InstanceStatus_INSTANCE_STATUS_CREATED
	case types.InstanceStatusStarting:
		return generated.InstanceStatus_INSTANCE_STATUS_STARTING
	case types.InstanceStatusRunning:
		return generated.InstanceStatus_INSTANCE_STATUS_RUNNING
	case types.InstanceStatusStopped:
		return generated.InstanceStatus_INSTANCE_STATUS_STOPPED
	case types.InstanceStatusFailed:
		return generated.InstanceStatus_INSTANCE_STATUS_FAILED
	case types.InstanceStatusExited:
		return generated.InstanceStatus_INSTANCE_STATUS_EXITED
	default:
		return generated.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED
	}
}
