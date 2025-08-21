package server

import (
	"context"

	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
)

// SeedBuiltinPolicies ensures the built-in policies exist (idempotent).
func SeedBuiltinPolicies(ctx context.Context, st store.Store) error {
	pr := repos.NewPolicyRepo(st)
	builtins := []types.Policy{
		{Namespace: "system", Name: "root", Description: "Full access", Builtin: true, Rules: []types.PolicyRule{{Resource: "*", Verbs: []string{"*"}, Namespace: "*"}}},
		{Namespace: "system", Name: "admin", Description: "Full access", Builtin: true, Rules: []types.PolicyRule{{Resource: "*", Verbs: []string{"*"}, Namespace: "*"}}},
		{Namespace: "system", Name: "readwrite", Description: "Read/write typical ops", Builtin: true, Rules: []types.PolicyRule{{Resource: "*", Verbs: []string{"get", "list", "watch", "create", "update", "delete", "scale", "exec"}, Namespace: "*"}}},
		{Namespace: "system", Name: "readonly", Description: "Read-only", Builtin: true, Rules: []types.PolicyRule{{Resource: "*", Verbs: []string{"get", "list", "watch"}, Namespace: "*"}}},
	}
	for i := range builtins {
		name := builtins[i].Name
		if _, err := pr.Get(ctx, "system", name); err == nil {
			continue
		}
		_ = pr.Create(ctx, &builtins[i])
	}
	return nil
}
