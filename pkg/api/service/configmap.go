package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConfigMapService struct {
	generated.UnimplementedConfigMapServiceServer
	repo   *repos.ConfigRepo
	logger log.Logger
}

func NewConfigMapService(coreStore store.Store, logger log.Logger) *ConfigMapService {
	return &ConfigMapService{repo: repos.NewConfigRepo(coreStore), logger: logger}
}

func (s *ConfigMapService) CreateConfigMap(ctx context.Context, req *generated.CreateConfigMapRequest) (*generated.ConfigMapResponse, error) {
	if req.ConfigMap == nil {
		return nil, status.Error(codes.InvalidArgument, "config map is required")
	}
	now := time.Now()
	c := &types.ConfigMap{Name: req.ConfigMap.Name, Namespace: ns(req.ConfigMap.Namespace), Data: req.ConfigMap.Data, Version: 1, CreatedAt: now, UpdatedAt: now}
	ref := types.FormatRef(types.ResourceTypeConfigMap, c.Namespace, c.Name)
	if err := s.repo.Create(ctx, ref, c); err != nil {
		// If already exists, fall back to update path
		if store.IsAlreadyExistsError(err) {
			s.logger.Info("Config already exists, updating instead",
				log.Str("name", req.ConfigMap.Name),
				log.Str("namespace", ns(req.ConfigMap.Namespace)))
			return s.UpdateConfigMap(ctx, &generated.UpdateConfigMapRequest{ConfigMap: req.ConfigMap})
		}
		return nil, status.Errorf(codes.Internal, "create: %v", err)
	}
	return &generated.ConfigMapResponse{ConfigMap: toProtoConfigMap(c), Status: &generated.Status{Code: int32(codes.OK)}}, nil
}

func (s *ConfigMapService) GetConfigMap(ctx context.Context, req *generated.GetConfigMapRequest) (*generated.ConfigMapResponse, error) {
	ref := types.FormatRef(types.ResourceTypeConfigMap, ns(req.Namespace), req.Name)
	c, err := s.repo.Get(ctx, ref)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "get: %v", err)
	}
	return &generated.ConfigMapResponse{ConfigMap: toProtoConfigMap(c), Status: &generated.Status{Code: int32(codes.OK)}}, nil
}

func (s *ConfigMapService) UpdateConfigMap(ctx context.Context, req *generated.UpdateConfigMapRequest) (*generated.ConfigMapResponse, error) {
	if req.ConfigMap == nil {
		return nil, status.Error(codes.InvalidArgument, "config map is required")
	}
	namespace := ns(req.ConfigMap.Namespace)
	ref := types.FormatRef(types.ResourceTypeConfigMap, namespace, req.ConfigMap.Name)

	// Fetch current; if missing, return NotFound
	current, err := s.repo.Get(ctx, ref)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "config not found: %s/%s", namespace, req.ConfigMap.Name)
	}

	desired := &types.ConfigMap{Name: req.ConfigMap.Name, Namespace: namespace, Data: req.ConfigMap.Data}

	// If unchanged, no-op
	if reflect.DeepEqual(current.Data, desired.Data) {
		s.logger.Info("Config unchanged; no update", log.Str("name", desired.Name), log.Str("namespace", desired.Namespace))
		return &generated.ConfigMapResponse{ConfigMap: toProtoConfigMap(current), Status: &generated.Status{Code: int32(codes.OK)}}, nil
	}

	// For observability, compute hashes
	oldHash := hashConfig(current)
	newHash := hashConfig(desired)
	s.logger.Info("Updating config",
		log.Str("name", desired.Name), log.Str("namespace", desired.Namespace),
		log.Str("old_hash", oldHash[:8]), log.Str("new_hash", newHash[:8]))

	if err := s.repo.Update(ctx, ref, desired, store.WithSource(store.EventSourceAPI)); err != nil {
		return nil, status.Errorf(codes.Internal, "update: %v", err)
	}
	got, _ := s.repo.Get(ctx, ref)
	return &generated.ConfigMapResponse{ConfigMap: toProtoConfigMap(got), Status: &generated.Status{Code: int32(codes.OK)}}, nil
}

func (s *ConfigMapService) DeleteConfigMap(ctx context.Context, req *generated.DeleteConfigMapRequest) (*generated.Status, error) {
	ref := types.FormatRef(types.ResourceTypeConfigMap, ns(req.Namespace), req.Name)
	if err := s.repo.Delete(ctx, ref); err != nil {
		return nil, status.Errorf(codes.Internal, "delete: %v", err)
	}
	return &generated.Status{Code: int32(codes.OK)}, nil
}

func (s *ConfigMapService) ListConfigMaps(ctx context.Context, req *generated.ListConfigMapsRequest) (*generated.ListConfigMapsResponse, error) {
	// BaseRepo exposes List; ConfigMapRepo does not add a wrapper, so call through base
	configs, err := s.repo.List(ctx, ns(req.Namespace))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list: %v", err)
	}
	out := make([]*generated.ConfigMap, 0, len(configs))
	for _, c := range configs {
		out = append(out, toProtoConfigMap(c))
	}
	return &generated.ListConfigMapsResponse{ConfigMaps: out, Status: &generated.Status{Code: int32(codes.OK)}}, nil
}

func toProtoConfigMap(c *types.ConfigMap) *generated.ConfigMap {
	return &generated.ConfigMap{Name: c.Name, Namespace: c.Namespace, Data: c.Data, Version: int32(c.Version), CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339)}
}

// hashConfig returns a deterministic hash for comparing config content
func hashConfig(c *types.ConfigMap) string {
	payload := struct {
		Data map[string]string `json:"data"`
	}{Data: c.Data}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}
