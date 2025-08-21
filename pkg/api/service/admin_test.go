package service

import (
	"context"
	"testing"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/store"
)

func TestAdminBootstrap_LocalOnlyDefault(t *testing.T) {
	st := store.NewTestStore()
	t.Cleanup(func() { st.Close() })
	svc := NewAdminService(st, nil)

	// Should succeed when no users exist (peer check best-effort; not enforced in test)
	resp, err := svc.AdminBootstrap(context.Background(), nil)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if resp.TokenSecret == "" {
		t.Fatalf("expected token secret")
	}
}

func TestPolicyCRUD_MVP(t *testing.T) {
	st := store.NewTestStore()
	t.Cleanup(func() { st.Close() })
	svc := NewAdminService(st, nil)

	// Create
	_, err := svc.PolicyCreate(context.Background(), &generated.PolicyCreateRequest{Policy: &generated.Policy{Name: "dev"}})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	// Get
	if _, err := svc.PolicyGet(context.Background(), &generated.PolicyGetRequest{Name: "dev"}); err != nil {
		t.Fatalf("get policy: %v", err)
	}
	// List
	if _, err := svc.PolicyList(context.Background(), &generated.PolicyListRequest{}); err != nil {
		t.Fatalf("list policy: %v", err)
	}
	// Delete
	if _, err := svc.PolicyDelete(context.Background(), &generated.PolicyDeleteRequest{Name: "dev"}); err != nil {
		t.Fatalf("delete policy: %v", err)
	}
}
