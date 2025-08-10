package service

import (
	"context"
	"testing"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
)

func TestConfigServiceCRUD(t *testing.T) {
	ctx := context.Background()
	st := store.NewTestStoreWithOptions(store.StoreOptions{
		ConfigLimits: store.Limits{MaxObjectBytes: 1 << 20, MaxKeyNameLength: 256},
	})
	svc := NewConfigMapService(st, log.GetDefaultLogger())

	// Create
	_, err := svc.CreateConfigMap(ctx, &generated.CreateConfigMapRequest{ConfigMap: &generated.ConfigMap{
		Name:      "app-config",
		Namespace: "prod",
		Data:      map[string]string{"logLevel": "info"},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get
	getResp, err := svc.GetConfigMap(ctx, &generated.GetConfigMapRequest{Name: "app-config", Namespace: "prod"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if getResp.ConfigMap.Name != "app-config" {
		t.Fatalf("bad get name")
	}

	// List
	listResp, err := svc.ListConfigMaps(ctx, &generated.ListConfigMapsRequest{Namespace: "prod"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listResp.ConfigMaps) != 1 {
		t.Fatalf("expected 1 config, got %d", len(listResp.ConfigMaps))
	}

	// Update
	_, err = svc.UpdateConfigMap(ctx, &generated.UpdateConfigMapRequest{ConfigMap: &generated.ConfigMap{
		Name:      "app-config",
		Namespace: "prod",
		Data:      map[string]string{"logLevel": "debug"},
	}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	// Delete
	_, err = svc.DeleteConfigMap(ctx, &generated.DeleteConfigMapRequest{Name: "app-config", Namespace: "prod"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
}
