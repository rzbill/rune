package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
	"github.com/rzbill/rune/pkg/utils"
)

// AdminService implements generated.AdminServiceServer
type AdminService struct {
	generated.UnimplementedAdminServiceServer
	st     store.Store
	logger log.Logger
}

func NewAdminService(st store.Store, logger log.Logger) *AdminService {
	if logger == nil {
		logger = log.GetDefaultLogger().WithComponent("admin-service")
	} else {
		logger = logger.WithComponent("admin-service")
	}
	return &AdminService{st: st, logger: logger}
}

// AdminBootstrap creates the built-in root subject and returns a management token (opaque secret once)
func (s *AdminService) AdminBootstrap(ctx context.Context, _ *generated.AdminBootstrapRequest) (*generated.AdminBootstrapResponse, error) {
	// Ensure root user exists; create if missing
	ur := repos.NewUserRepo(s.st)
	root, err := ur.GetByName(ctx, "root")
	if store.IsNotFoundError(err) {
		createdRoot, err := ur.Create(ctx, &types.User{Name: "root"})
		if err != nil {
			return nil, err
		}
		root = createdRoot
	}

	// Ensure root policy attached to root user
	if err := s.ensureUserHasPolicy(ctx, ur, root, "root"); err != nil {
		return nil, err
	}
	// Issue root token with unique name
	tr := repos.NewTokenRepo(s.st)
	name := fmt.Sprintf("bootstrap-admin-%d", time.Now().UnixNano())
	tok, secret, err := tr.Issue(ctx, name, root.ID, "user", "bootstrap-admin", 0)
	if err != nil {
		return nil, err
	}
	return &generated.AdminBootstrapResponse{TokenId: tok.ID, TokenSecret: secret, SubjectId: root.ID}, nil
}

// ensureUserHasPolicy attaches policyName to user if missing
func (s *AdminService) ensureUserHasPolicy(ctx context.Context, ur *repos.UserRepo, u *types.User, policyName string) error {
	for _, p := range u.Policies {
		if p == policyName {
			return nil
		}
	}
	u.Policies = append(u.Policies, policyName)
	return ur.Update(ctx, u)
}

// PolicyCreate creates or updates a policy
func (s *AdminService) PolicyCreate(ctx context.Context, req *generated.PolicyCreateRequest) (*generated.PolicyCreateResponse, error) {
	pr := repos.NewPolicyRepo(s.st)
	p := &types.Policy{Name: req.Policy.Name, Description: req.Policy.Description, Builtin: false}
	for _, r := range req.Policy.Rules {
		p.Rules = append(p.Rules, types.PolicyRule{Resource: r.Resource, Verbs: r.Verbs, Namespace: r.Namespace})
	}
	if err := pr.Create(ctx, p); err != nil {
		// try update if exists
		if err := pr.Update(ctx, p); err != nil {
			return nil, err
		}
	}
	return &generated.PolicyCreateResponse{Policy: req.Policy}, nil
}

// PolicyUpdate updates a policy
func (s *AdminService) PolicyUpdate(ctx context.Context, req *generated.PolicyUpdateRequest) (*generated.PolicyUpdateResponse, error) {
	pr := repos.NewPolicyRepo(s.st)
	p := &types.Policy{Name: req.Policy.Name, Description: req.Policy.Description, Builtin: req.Policy.Builtin}
	for _, r := range req.Policy.Rules {
		p.Rules = append(p.Rules, types.PolicyRule{Resource: r.Resource, Verbs: r.Verbs, Namespace: r.Namespace})
	}
	if err := pr.Update(ctx, p); err != nil {
		return nil, err
	}
	return &generated.PolicyUpdateResponse{Policy: req.Policy}, nil
}

// PolicyDelete deletes a policy
func (s *AdminService) PolicyDelete(ctx context.Context, req *generated.PolicyDeleteRequest) (*generated.PolicyDeleteResponse, error) {
	pr := repos.NewPolicyRepo(s.st)
	name := req.Name
	if name == "" && req.Id != "" {
		name = req.Id
	}
	if err := pr.Delete(ctx, name); err != nil {
		return nil, err
	}
	return &generated.PolicyDeleteResponse{Deleted: true}, nil
}

// PolicyGet fetches a policy
func (s *AdminService) PolicyGet(ctx context.Context, req *generated.PolicyGetRequest) (*generated.PolicyGetResponse, error) {
	pr := repos.NewPolicyRepo(s.st)
	name := req.Name
	if name == "" && req.Id != "" {
		name = req.Id
	}
	p, err := pr.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	gp := &generated.Policy{Id: p.ID, Name: p.Name, Description: p.Description, Builtin: p.Builtin}
	for _, r := range p.Rules {
		gp.Rules = append(gp.Rules, &generated.PolicyRule{Resource: r.Resource, Verbs: r.Verbs, Namespace: r.Namespace})
	}
	return &generated.PolicyGetResponse{Policy: gp}, nil
}

// PolicyList lists policies
func (s *AdminService) PolicyList(ctx context.Context, _ *generated.PolicyListRequest) (*generated.PolicyListResponse, error) {
	pr := repos.NewPolicyRepo(s.st)
	ps, err := pr.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []*generated.Policy
	for _, p := range ps {
		gp := &generated.Policy{Id: p.ID, Name: p.Name, Description: p.Description, Builtin: p.Builtin}
		for _, r := range p.Rules {
			gp.Rules = append(gp.Rules, &generated.PolicyRule{Resource: r.Resource, Verbs: r.Verbs, Namespace: r.Namespace})
		}
		out = append(out, gp)
	}
	return &generated.PolicyListResponse{Policies: out}, nil
}

// TokenList lists tokens (admin-only). Does not return secrets.
func (s *AdminService) TokenList(ctx context.Context, _ *generated.TokenListRequest) (*generated.TokenListResponse, error) {
	tr := repos.NewTokenRepo(s.st)
	list, err := tr.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*generated.TokenInfo, 0, len(list))
	for _, t := range list {
		ti := &generated.TokenInfo{
			Id:          t.ID,
			Name:        t.Name,
			SubjectId:   t.SubjectID,
			SubjectType: t.SubjectType,
			Description: t.Description,
			IssuedAt:    t.IssuedAt.Unix(),
			Revoked:     t.Revoked,
		}
		if t.ExpiresAt != nil {
			ti.ExpiresAt = t.ExpiresAt.Unix()
		}
		out = append(out, ti)
	}
	return &generated.TokenListResponse{Tokens: out}, nil
}

// PolicyAttachToSubject attaches a policy to a user subject (MVP: users only)
func (s *AdminService) PolicyAttachToSubject(ctx context.Context, req *generated.PolicyAttachToSubjectRequest) (*generated.PolicyAttachToSubjectResponse, error) {
	ur := repos.NewUserRepo(s.st)
	u, err := ur.GetByNameOrID(ctx, utils.PickFirstNonEmpty(req.SubjectId, req.SubjectName))
	if err != nil {
		return nil, err
	}
	pol := req.PolicyName
	if pol == "" && req.PolicyId != "" {
		pol = req.PolicyId
	}
	if err := s.ensureUserHasPolicy(ctx, ur, u, pol); err != nil {
		return nil, err
	}
	return &generated.PolicyAttachToSubjectResponse{Attached: true}, nil
}

// PolicyDetachFromSubject detaches a policy from a user subject (MVP: users only)
func (s *AdminService) PolicyDetachFromSubject(ctx context.Context, req *generated.PolicyDetachFromSubjectRequest) (*generated.PolicyDetachFromSubjectResponse, error) {
	ur := repos.NewUserRepo(s.st)
	u, err := ur.GetByNameOrID(ctx, utils.PickFirstNonEmpty(req.SubjectId, req.SubjectName))
	if err != nil {
		return nil, err
	}
	pol := req.PolicyName
	if pol == "" && req.PolicyId != "" {
		pol = req.PolicyId
	}
	var filtered []string
	for _, p := range u.Policies {
		if p != pol {
			filtered = append(filtered, p)
		}
	}
	u.Policies = filtered
	if err := ur.Update(ctx, u); err != nil {
		return nil, err
	}
	return &generated.PolicyDetachFromSubjectResponse{Detached: true}, nil
}

// UserCreate upserts a user and attaches policies (MVP)
func (s *AdminService) UserCreate(ctx context.Context, req *generated.UserCreateRequest) (*generated.UserCreateResponse, error) {
	ur := repos.NewUserRepo(s.st)
	// Try get by name; if not exists, create
	u, err := ur.GetByName(ctx, req.Name)
	if err != nil {
		u = &types.User{Name: req.Name, Email: req.Email}
		createdUser, err := ur.Create(ctx, u)
		if err != nil {
			return nil, err
		}
		u = createdUser
	} else {
		// update email if provided
		if req.Email != "" {
			u.Email = req.Email
		}
	}
	// attach policies (dedupe)
	seen := map[string]struct{}{}
	for _, p := range u.Policies {
		seen[p] = struct{}{}
	}
	for _, p := range req.Policies {
		seen[p] = struct{}{}
	}
	u.Policies = u.Policies[:0]
	for p := range seen {
		u.Policies = append(u.Policies, p)
	}
	if err := ur.Update(ctx, u); err != nil {
		return nil, err
	}
	return &generated.UserCreateResponse{User: &generated.Subject{Id: u.ID, Kind: "user", Name: u.Name, Email: u.Email, Policies: u.Policies}}, nil
}

// UserList lists users (subjects)
func (s *AdminService) UserList(ctx context.Context, _ *generated.UserListRequest) (*generated.UserListResponse, error) {
	var users []types.User
	if err := s.st.List(ctx, types.ResourceTypeUser, "system", &users); err != nil {
		return nil, err
	}
	var out []*generated.Subject
	for _, u := range users {
		uu := u
		out = append(out, &generated.Subject{Id: uu.ID, Kind: "user", Name: uu.Name, Email: uu.Email, Policies: uu.Policies})
	}
	return &generated.UserListResponse{Users: out}, nil
}
