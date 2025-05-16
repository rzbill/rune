package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

// ScalingOps handles the lifecycle of scaling operations
type ScalingOps struct {
	store  store.Store
	logger log.Logger
}

// NewScalingOps creates a new scaling manager
func NewScalingOps(store store.Store, logger log.Logger) *ScalingOps {
	return &ScalingOps{
		store:  store,
		logger: logger.WithComponent("scaling-ops"),
	}
}

// CreateScalingOperation creates a new scaling operation
func (m *ScalingOps) CreateScalingOperation(ctx context.Context, service *types.Service, params types.ScalingOperationParams) (*types.ScalingOperation, error) {
	// Create a unique key for this scaling operation
	opKey := fmt.Sprintf("scale-%s-%d", service.Name, time.Now().UnixNano())

	// Create scaling operation record
	op := &types.ScalingOperation{
		ID:           opKey,
		Namespace:    service.Namespace,
		ServiceName:  service.Name,
		CurrentScale: params.CurrentScale,
		TargetScale:  params.TargetScale,
		StepSize:     params.StepSize,
		Interval:     params.IntervalSeconds,
		StartTime:    time.Now(),
		Status:       types.ScalingOperationStatusInProgress,
		Mode:         types.ScalingModeImmediate,
	}

	if params.IsGradual {
		op.Mode = types.ScalingModeGradual
	}

	// Store the scaling operation
	if err := m.store.Create(ctx, types.ResourceTypeScalingOperation, service.Namespace, opKey, op); err != nil {
		return nil, fmt.Errorf("failed to store scaling operation: %w", err)
	}

	return op, nil
}

// GetActiveOperation gets the active scaling operation for a service if one exists
func (m *ScalingOps) GetActiveOperation(ctx context.Context, namespace, serviceName string) (*types.ScalingOperation, error) {
	var operations []types.ScalingOperation
	err := m.store.List(ctx, types.ResourceTypeScalingOperation, namespace, &operations)
	if err != nil {
		return nil, err
	}

	// Find the most recent active operation
	var activeOp *types.ScalingOperation
	for _, op := range operations {
		if op.ServiceName == serviceName && op.Status == types.ScalingOperationStatusInProgress {
			if activeOp == nil || op.StartTime.After(activeOp.StartTime) {
				activeOp = &op
			}
		}
	}

	return activeOp, nil
}
