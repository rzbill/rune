package service

import (
	"context"
	"testing"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/crypto"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
)

func TestSecretServiceCRUD(t *testing.T) {
	ctx := context.Background()
	kek, err := crypto.RandomBytes(32)
	if err != nil {
		t.Fatalf("failed to generate KEK: %v", err)
	}
	st := store.NewTestStoreWithOptions(store.StoreOptions{
		KEKBytes:                kek,
		SecretEncryptionEnabled: true,
		SecretLimits: store.Limits{
			MaxObjectBytes:   1 << 20,
			MaxKeyNameLength: 256,
		},
	})

	svc := NewSecretService(st, log.GetDefaultLogger())

	// Create
	createResp, err := svc.CreateSecret(ctx, &generated.CreateSecretRequest{Secret: &generated.Secret{
		Name:      "db-credentials",
		Namespace: "prod",
		Type:      "static",
		Data:      map[string]string{"username": "admin", "password": "s3cr3t"},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if createResp.Secret == nil || createResp.Secret.Name != "db-credentials" {
		t.Fatalf("bad create resp")
	}

	// Get
	getResp, err := svc.GetSecret(ctx, &generated.GetSecretRequest{Name: "db-credentials", Namespace: "prod"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if getResp.Secret.Name != "db-credentials" {
		t.Fatalf("bad get name")
	}

	// List
	listResp, err := svc.ListSecrets(ctx, &generated.ListSecretsRequest{Namespace: "prod"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listResp.Secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(listResp.Secrets))
	}

	// Update
	time.Sleep(10 * time.Millisecond)
	updResp, err := svc.UpdateSecret(ctx, &generated.UpdateSecretRequest{Secret: &generated.Secret{
		Name:      "db-credentials",
		Namespace: "prod",
		Type:      "static",
		Data:      map[string]string{"username": "admin", "password": "n3w"},
	}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updResp.Secret == nil {
		t.Fatalf("nil update resp")
	}

	// Delete
	delResp, err := svc.DeleteSecret(ctx, &generated.DeleteSecretRequest{Name: "db-credentials", Namespace: "prod"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if delResp.Code != 0 {
		_ = delResp
	}
}
