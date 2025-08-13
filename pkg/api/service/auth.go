package service

import (
	"context"
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
	tokenRepo *repos.TokenRepo
	logger    log.Logger
}

func NewAuthService(st store.Store, logger log.Logger) *AuthService {
	if logger == nil {
		logger = log.GetDefaultLogger().WithComponent("auth-service")
	} else {
		logger = logger.WithComponent("auth-service")
	}
	return &AuthService{tokenRepo: repos.NewTokenRepo(st), logger: logger}
}

// AuthService implementation
func (s *AuthService) WhoAmI(ctx context.Context, _ *generated.WhoAmIRequest) (*generated.WhoAmIResponse, error) {
	resp := &generated.WhoAmIResponse{}
	// Re-extract bearer and look up token to avoid relying on server context types
	if token, err := grpc_auth.AuthFromMD(ctx, "bearer"); err == nil {
		if tok, err2 := s.tokenRepo.FindBySecret(ctx, token); err2 == nil {
			resp.SubjectId = tok.SubjectID
			for _, r := range tok.RoleBindings {
				resp.Roles = append(resp.Roles, string(r))
			}
		}
	}
	return resp, nil
}

func (s *AuthService) CreateToken(ctx context.Context, req *generated.CreateTokenRequest) (*generated.CreateTokenResponse, error) {
	roles := make([]types.Role, 0, len(req.Roles))
	for _, r := range req.Roles {
		roles = append(roles, types.Role(r))
	}
	tok, secret, err := s.tokenRepo.Issue(ctx, req.Namespace, req.Name, req.SubjectId, req.SubjectType, roles, req.Description, time.Duration(req.TtlSeconds)*time.Second)
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
