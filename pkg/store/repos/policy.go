package repos

import (
	"context"

	"github.com/google/uuid"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

type PolicyRepo struct{ st store.Store }

func NewPolicyRepo(st store.Store) *PolicyRepo { return &PolicyRepo{st: st} }

func (r *PolicyRepo) Create(ctx context.Context, p *types.Policy) error {
	if p.Namespace == "" {
		p.Namespace = "system"
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	return r.st.Create(ctx, types.ResourceTypePolicy, p.Namespace, p.Name, p)
}

func (r *PolicyRepo) Update(ctx context.Context, p *types.Policy) error {
	if p.Namespace == "" {
		p.Namespace = "system"
	}
	return r.st.Update(ctx, types.ResourceTypePolicy, p.Namespace, p.Name, p)
}

func (r *PolicyRepo) Get(ctx context.Context, ns, name string) (*types.Policy, error) {
	var p types.Policy
	if err := r.st.Get(ctx, types.ResourceTypePolicy, ns, name, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PolicyRepo) Delete(ctx context.Context, ns, name string) error {
	return r.st.Delete(ctx, types.ResourceTypePolicy, ns, name)
}

func (r *PolicyRepo) List(ctx context.Context, ns string) ([]types.Policy, error) {
	var ps []types.Policy
	if err := r.st.List(ctx, types.ResourceTypePolicy, ns, &ps); err != nil {
		return nil, err
	}
	return ps, nil
}
