package client

import (
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConfigMapClient provides methods for interacting with configs on the Rune API server.
type ConfigMapClient struct {
	client *Client
	logger log.Logger
	svc    generated.ConfigMapServiceClient
}

// NewConfigMapClient creates a new configmap client.
func NewConfigMapClient(client *Client) *ConfigMapClient {
	return &ConfigMapClient{
		client: client,
		logger: client.logger.WithComponent("configmap-client"),
		svc:    generated.NewConfigMapServiceClient(client.conn),
	}
}

// GetLogger returns the logger for this client
func (c *ConfigMapClient) GetLogger() log.Logger { return c.logger }

// CreateConfig creates a new configmap on the API server.
func (c *ConfigMapClient) CreateConfigMap(configmap *types.ConfigMap) error {
	c.logger.Debug("Creating configmap", log.Str("name", configmap.Name), log.Str("namespace", configmap.Namespace))

	req := &generated.CreateConfigMapRequest{
		ConfigMap: c.configToProto(configmap),
	}

	ctx, cancel := c.client.Context()
	defer cancel()

	resp, err := c.svc.CreateConfigMap(ctx, req)
	if err != nil {
		c.logger.Error("Failed to create configmap", log.Err(err), log.Str("name", configmap.Name))
		return convertGRPCError("create configmap", err)
	}
	if resp.Status != nil && resp.Status.Code != int32(codes.OK) {
		err := fmt.Errorf("API error: %s", resp.Status.Message)
		c.logger.Error("Failed to create configmap", log.Err(err), log.Str("name", configmap.Name))
		return err
	}
	return nil
}

// GetConfigMap retrieves a configmap by name.
func (c *ConfigMapClient) GetConfigMap(namespace, name string) (*types.ConfigMap, error) {
	c.logger.Debug("Getting configmap", log.Str("name", name), log.Str("namespace", namespace))

	req := &generated.GetConfigMapRequest{Name: name, Namespace: namespace}

	ctx, cancel := c.client.Context()
	defer cancel()

	resp, err := c.svc.GetConfigMap(ctx, req)
	if err != nil {
		statusErr, ok := status.FromError(err)
		if ok && statusErr.Code() == codes.NotFound {
			return nil, fmt.Errorf("configmap not found: %s/%s", namespace, name)
		}
		c.logger.Error("Failed to get configmap", log.Err(err), log.Str("name", name))
		return nil, convertGRPCError("get configmap", err)
	}
	if resp.Status != nil && resp.Status.Code != int32(codes.OK) {
		err := fmt.Errorf("API error: %s", resp.Status.Message)
		c.logger.Error("Failed to get configmap", log.Err(err), log.Str("name", name))
		return nil, err
	}

	cfg, err := c.protoToConfigMap(resp.ConfigMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert configmap: %w", err)
	}
	return cfg, nil
}

// UpdateConfigMap updates an existing configmap.
func (c *ConfigMapClient) UpdateConfigMap(configmap *types.ConfigMap) error {
	c.logger.Debug("Updating configmap", log.Str("name", configmap.Name), log.Str("namespace", configmap.Namespace))

	req := &generated.UpdateConfigMapRequest{ConfigMap: c.configToProto(configmap)}

	ctx, cancel := c.client.Context()
	defer cancel()

	resp, err := c.svc.UpdateConfigMap(ctx, req)
	if err != nil {
		c.logger.Error("Failed to update configmap", log.Err(err), log.Str("name", configmap.Name))
		return convertGRPCError("update configmap", err)
	}
	if resp.Status != nil && resp.Status.Code != int32(codes.OK) {
		err := fmt.Errorf("API error: %s", resp.Status.Message)
		c.logger.Error("Failed to update configmap", log.Err(err), log.Str("name", configmap.Name))
		return err
	}
	return nil
}

// DeleteConfigMap deletes a configmap.
func (c *ConfigMapClient) DeleteConfigMap(namespace, name string) error {
	c.logger.Debug("Deleting configmap", log.Str("name", name), log.Str("namespace", namespace))

	req := &generated.DeleteConfigMapRequest{Name: name, Namespace: namespace}

	ctx, cancel := c.client.Context()
	defer cancel()

	resp, err := c.svc.DeleteConfigMap(ctx, req)
	if err != nil {
		c.logger.Error("Failed to delete configmap", log.Err(err), log.Str("name", name))
		return convertGRPCError("delete configmap", err)
	}
	if resp.Code != int32(codes.OK) {
		return fmt.Errorf("API error: %s", resp.Message)
	}
	return nil
}

// ListConfigMaps lists configmaps in a namespace.
func (c *ConfigMapClient) ListConfigMaps(namespace string, labelSelector string, fieldSelector string) ([]*types.ConfigMap, error) {
	c.logger.Debug("Listing configmaps", log.Str("namespace", namespace))

	req := &generated.ListConfigMapsRequest{Namespace: namespace}

	ctx, cancel := c.client.Context()
	defer cancel()

	resp, err := c.svc.ListConfigMaps(ctx, req)
	if err != nil {
		c.logger.Error("Failed to list configs", log.Err(err), log.Str("namespace", namespace))
		return nil, convertGRPCError("list configs", err)
	}
	if resp.Status != nil && resp.Status.Code != int32(codes.OK) {
		err := fmt.Errorf("API error: %s", resp.Status.Message)
		c.logger.Error("Failed to list configs", log.Err(err), log.Str("namespace", namespace))
		return nil, err
	}

	configs := make([]*types.ConfigMap, 0, len(resp.ConfigMaps))
	for _, pc := range resp.ConfigMaps {
		cfg, err := c.protoToConfigMap(pc)
		if err != nil {
			c.logger.Warn("Failed to convert configmap", log.Err(err))
			continue
		}
		configs = append(configs, cfg)
	}
	// Apply client-side filtering
	filtered, err := c.filterConfigsBySelectors(configs, labelSelector, fieldSelector)
	if err != nil {
		return nil, err
	}
	return filtered, nil
}

// Converters
func (c *ConfigMapClient) configToProto(cfg *types.ConfigMap) *generated.ConfigMap {
	if cfg == nil {
		return nil
	}
	return &generated.ConfigMap{
		Name:      cfg.Name,
		Namespace: cfg.Namespace,
		Data:      cfg.Data,
	}
}

func (c *ConfigMapClient) protoToConfigMap(proto *generated.ConfigMap) (*types.ConfigMap, error) {
	if proto == nil {
		return nil, nil
	}
	createdAt, err := time.Parse(time.RFC3339, proto.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse createdAt: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, proto.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updatedAt: %w", err)
	}

	return &types.ConfigMap{
		Name:      proto.Name,
		Namespace: proto.Namespace,
		Data:      proto.Data,
		Version:   int(proto.Version),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// filterConfigsBySelectors applies client-side filtering for configs.
// Supported field selectors: name. Label selectors are not supported for configs in this build.
func (c *ConfigMapClient) filterConfigsBySelectors(configs []*types.ConfigMap, labelSelector, fieldSelector string) ([]*types.ConfigMap, error) {
	if labelSelector != "" {
		return nil, fmt.Errorf("label selector is not supported for configs")
	}
	fields, err := parseSelector(fieldSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid field selector: %w", err)
	}
	var nameFilter string
	if v, ok := fields["name"]; ok {
		nameFilter = v
		delete(fields, "name")
	}
	if len(fields) > 0 {
		return nil, fmt.Errorf("unsupported field selector keys for configs: %v", fields)
	}
	if nameFilter == "" {
		return configs, nil
	}
	result := make([]*types.ConfigMap, 0, len(configs))
	for _, cfg := range configs {
		if cfg != nil && cfg.Name == nameFilter {
			result = append(result, cfg)
		}
	}
	return result, nil
}
