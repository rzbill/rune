package server

import (
	"context"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/rzbill/rune/pkg/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// AuthorizationHeader is the header key for API key authentication.
	AuthorizationHeader = "authorization"

	// APIKeyPrefix is the prefix for API key values in the Authorization header.
	APIKeyPrefix = "Bearer "
)

// authFunc is the authentication function for gRPC requests.
func (s *APIServer) authFunc(ctx context.Context) (context.Context, error) {
	// If auth is disabled, no need to check auth
	if !s.options.EnableAuth {
		return ctx, nil
	}

	// Extract API key from metadata
	token, err := auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		s.logger.Warn("Missing API key in request")
		return nil, status.Errorf(codes.Unauthenticated, "missing API key: %v", err)
	}

	// Validate API key
	if !s.validateAPIKey(token) {
		s.logger.Warn("Invalid API key", log.Str("key", token))
		return nil, status.Errorf(codes.Unauthenticated, "invalid API key")
	}

	// API key is valid, add it to the context
	return ctx, nil
}

// validateAPIKey checks if the given API key is valid.
func (s *APIServer) validateAPIKey(key string) bool {
	// For demonstration purposes, we'll just do a simple comparison
	for _, validKey := range s.options.APIKeys {
		if key == validKey {
			return true
		}
	}
	return false
}

// extractAPIKey extracts the API key from the Authorization header.
func extractAPIKey(header string) (string, error) {
	if !strings.HasPrefix(header, APIKeyPrefix) {
		return "", status.Errorf(codes.Unauthenticated, "bad authorization string")
	}
	return strings.TrimPrefix(header, APIKeyPrefix), nil
}

// authInterceptor returns a unary interceptor for authentication.
func (s *APIServer) authInterceptor() grpc.UnaryServerInterceptor {
	return auth.UnaryServerInterceptor(s.authFunc)
}

// authStreamInterceptor returns a stream interceptor for authentication.
func (s *APIServer) authStreamInterceptor() grpc.StreamServerInterceptor {
	return auth.StreamServerInterceptor(s.authFunc)
}

// TODO: This function is currently unused - intended for future HTTP authentication implementation
// extractAPIKeyFromHTTPRequest extracts the API key from an HTTP request.
func extractAPIKeyFromHTTPRequest(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "no metadata in context")
	}

	authHeader := md.Get(AuthorizationHeader)
	if len(authHeader) == 0 {
		return "", status.Errorf(codes.Unauthenticated, "no authorization header")
	}

	return extractAPIKey(authHeader[0])
}
