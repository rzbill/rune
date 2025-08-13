package server

import (
	"context"
	"testing"

	"github.com/rzbill/rune/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Test minimal RBAC policy on unary interceptor
func TestRBACUnaryInterceptor(t *testing.T) {
	s, err := New(WithAuth(nil))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	// helper
	call := func(roles []types.Role, method string) error {
		ctx := context.Background()
		ctx = context.WithValue(ctx, authCtxKey, &AuthInfo{SubjectID: "sub", Roles: roles})
		info := &grpc.UnaryServerInfo{FullMethod: method}
		h := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }
		_, err := s.rbacUnaryInterceptor()(ctx, nil, info, h)
		return err
	}

	// readonly can read
	if err := call([]types.Role{types.RoleReadOnly}, "/rune.api.ServiceService/GetService"); err != nil {
		t.Fatalf("readonly should read: %v", err)
	}
	// readonly cannot write
	err = call([]types.Role{types.RoleReadOnly}, "/rune.api.ServiceService/CreateService")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("readonly should be denied write, got %v", err)
	}
	// readwrite can write
	if err := call([]types.Role{types.RoleReadWrite}, "/rune.api.ServiceService/CreateService"); err != nil {
		t.Fatalf("readwrite should write: %v", err)
	}
	// admin can all
	if err := call([]types.Role{types.RoleAdmin}, "/rune.api.ServiceService/DeleteService"); err != nil {
		t.Fatalf("admin should write: %v", err)
	}
}

// Test RBAC on stream interceptor for logs (read) vs exec (write)
func TestRBACStreamInterceptor(t *testing.T) {
	s, err := New(WithAuth(nil))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	call := func(roles []types.Role, method string) error {
		ctx := context.Background()
		ctx = context.WithValue(ctx, authCtxKey, &AuthInfo{SubjectID: "sub", Roles: roles})
		ss := &fakeServerStream{ctx: ctx}
		info := &grpc.StreamServerInfo{FullMethod: method}
		h := func(srv interface{}, stream grpc.ServerStream) error { return nil }
		return s.rbacStreamInterceptor()(nil, ss, info, h)
	}

	// readonly can stream logs
	if err := call([]types.Role{types.RoleReadOnly}, "/rune.api.LogService/StreamLogs"); err != nil {
		t.Fatalf("readonly should stream logs: %v", err)
	}
	// readonly cannot exec (treated as write)
	err = call([]types.Role{types.RoleReadOnly}, "/rune.api.ExecService/StreamExec")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("readonly should be denied exec, got %v", err)
	}
}

type fakeServerStream struct{ ctx context.Context }

func (f *fakeServerStream) SetHeader(md metadata.MD) error  { return nil }
func (f *fakeServerStream) SendHeader(md metadata.MD) error { return nil }
func (f *fakeServerStream) SetTrailer(md metadata.MD)       {}
func (f *fakeServerStream) Context() context.Context        { return f.ctx }
func (f *fakeServerStream) SendMsg(m interface{}) error     { return nil }
func (f *fakeServerStream) RecvMsg(m interface{}) error     { return nil }
