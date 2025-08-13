package repos

import (
	"context"
	"fmt"

	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
)

type UserRepo struct{ st store.Store }

func NewUserRepo(st store.Store) *UserRepo { return &UserRepo{st: st} }

func (r *UserRepo) Create(ctx context.Context, u *types.User) error {
	if u.Namespace == "" {
		u.Namespace = "system"
	}
	if u.Name == "" {
		return fmt.Errorf("user name required")
	}
	return r.st.Create(ctx, types.ResourceTypeUser, u.Namespace, u.Name, u)
}

func (r *UserRepo) Get(ctx context.Context, ns, name string) (*types.User, error) {
	var u types.User
	if err := r.st.Get(ctx, types.ResourceTypeUser, ns, name, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Update(ctx context.Context, u *types.User) error {
	return r.st.Update(ctx, types.ResourceTypeUser, u.Namespace, u.Name, u)
}

func (r *UserRepo) Delete(ctx context.Context, ns, name string) error {
	return r.st.Delete(ctx, types.ResourceTypeUser, ns, name)
}
