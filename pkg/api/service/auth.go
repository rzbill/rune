package service

import (
	"context"
	"fmt"
	"time"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
)

// AuthService implements generated.AuthServiceServer
type AuthService struct {
	generated.UnimplementedAuthServiceServer
	tokenRepo  *repos.TokenRepo
	userRepo   *repos.UserRepo
	policyRepo *repos.PolicyRepo
	logger     log.Logger
}

func NewAuthService(st store.Store, logger log.Logger) *AuthService {
	if logger == nil {
		logger = log.GetDefaultLogger().WithComponent("auth-service")
	} else {
		logger = logger.WithComponent("auth-service")
	}
	return &AuthService{
		tokenRepo:  repos.NewTokenRepo(st),
		userRepo:   repos.NewUserRepo(st),
		policyRepo: repos.NewPolicyRepo(st),
		logger:     logger,
	}
}

// AuthService implementation
func (s *AuthService) WhoAmI(ctx context.Context, _ *generated.WhoAmIRequest) (*generated.WhoAmIResponse, error) {
	resp := &generated.WhoAmIResponse{}
	// Re-extract bearer and look up token to avoid relying on server context types
	if token, err := grpc_auth.AuthFromMD(ctx, "bearer"); err == nil {
		if tok, err2 := s.tokenRepo.FindBySecret(ctx, token); err2 == nil {
			resp.SubjectId = tok.SubjectID
		}
	}
	return resp, nil
}

func (s *AuthService) CreateToken(ctx context.Context, req *generated.CreateTokenRequest) (*generated.CreateTokenResponse, error) {
	// Determine subject type (users only for MVP)
	subjectType := req.SubjectType
	if subjectType == "" {
		subjectType = "user"
	}
	if subjectType != "user" {
		return nil, fmt.Errorf("invalid subject-type: %s (only 'user' supported)", subjectType)
	}

	// Resolve subject name: prefer SubjectId (treated as user name), else use token Name
	subjectName := req.SubjectId
	if subjectName == "" {
		subjectName = req.Name
	}
	if subjectName == "" {
		return nil, fmt.Errorf("either subject_id or name must be provided to derive subject")
	}

	// Ensure user exists (create if missing); attach default policies on auto-create
	u, err := s.userRepo.Get(ctx, "system", subjectName)
	if err != nil {
		u = &types.User{Namespace: "system", Name: subjectName}
		if err := s.userRepo.Create(ctx, u); err != nil {
			return nil, err
		}
		// Attach provided policies; fallback to readonly if none provided
		attached := false
		if len(req.Policies) > 0 {
			for _, p := range req.Policies {
				if p == "" {
					continue
				}
				if err := s.ensureUserHasPolicy(ctx, u, p); err != nil {
					return nil, err
				}
				attached = true
			}
		}
		if !attached {
			if err := s.ensureUserHasPolicy(ctx, u, "readonly"); err != nil {
				return nil, err
			}
		}
	}

	subjectID := u.ID
	if subjectID == "" {
		subjectID = u.Name
	}

	// Issue token
	tok, secret, err := s.tokenRepo.Issue(ctx, "system", req.Name, subjectID, subjectType, req.Description, time.Duration(req.TtlSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	return &generated.CreateTokenResponse{Id: tok.ID, Name: tok.Name, Namespace: tok.Namespace, Secret: secret}, nil
}

func (s *AuthService) RevokeToken(ctx context.Context, req *generated.RevokeTokenRequest) (*generated.RevokeTokenResponse, error) {
	if err := s.tokenRepo.Revoke(ctx, req.Namespace, req.Name); err != nil {
		return nil, err
	}
	return &generated.RevokeTokenResponse{Revoked: true}, nil
}

// ensureUserHasPolicy attaches policyName to the user if it's not already present
func (s *AuthService) ensureUserHasPolicy(ctx context.Context, u *types.User, policyName string) error {
	for _, p := range u.Policies {
		if p == policyName {
			return nil
		}
	}
	u.Policies = append(u.Policies, policyName)
	return s.userRepo.Update(ctx, u)
}
