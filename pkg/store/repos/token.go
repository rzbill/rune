package repos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

type TokenRepo struct{ st store.Store }

func NewTokenRepo(st store.Store) *TokenRepo { return &TokenRepo{st: st} }

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Issue creates a new token with a freshly generated secret. Returns the plaintext secret once.
func (r *TokenRepo) Issue(ctx context.Context, ns, name, subjectID, subjectType string, desc string, ttl time.Duration) (*types.Token, string, error) {
	if ns == "" {
		ns = "system"
	}
	if name == "" {
		name = uuid.NewString()
	}
	secret := uuid.NewString() + "." + uuid.NewString()
	now := time.Now()
	var exp *time.Time
	if ttl > 0 {
		t := now.Add(ttl)
		exp = &t
	}
	tok := &types.Token{
		Namespace:   ns,
		Name:        name,
		ID:          uuid.NewString(),
		SubjectID:   subjectID,
		SubjectType: subjectType,
		Description: desc,
		IssuedAt:    now,
		ExpiresAt:   exp,
		Revoked:     false,
		SecretHash:  hashSecret(secret),
	}
	if err := r.st.Create(ctx, types.ResourceTypeToken, ns, name, tok); err != nil {
		return nil, "", err
	}
	return tok, secret, nil
}

func (r *TokenRepo) Get(ctx context.Context, ns, name string) (*types.Token, error) {
	var t types.Token
	if err := r.st.Get(ctx, types.ResourceTypeToken, ns, name, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TokenRepo) Revoke(ctx context.Context, ns, name string) error {
	t, err := r.Get(ctx, ns, name)
	if err != nil {
		return err
	}
	t.Revoked = true
	return r.st.Update(ctx, types.ResourceTypeToken, ns, name, t)
}

// FindBySecret tries to locate and validate a token by comparing the hash.
func (r *TokenRepo) FindBySecret(ctx context.Context, secret string) (*types.Token, error) {
	// Token namespace is system; list all tokens in system namespace (MVP) and match hash
	// TODO: index for scalability
	var tokens []types.Token
	if err := r.st.List(ctx, types.ResourceTypeToken, "system", &tokens); err != nil {
		return nil, err
	}
	secret = strings.TrimSpace(secret)
	h := hashSecret(secret)
	now := time.Now()
	for _, t := range tokens {
		if t.SecretHash == h && !t.Revoked && (t.ExpiresAt == nil || t.ExpiresAt.After(now)) {
			tt := t
			return &tt, nil
		}
	}
	return nil, fmt.Errorf("token not found or invalid")
}
