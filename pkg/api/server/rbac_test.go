package server

import (
	"context"
	"testing"

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
	call := func(hasSubject bool, method string) error {
		ctx := context.Background()
		if hasSubject {
			ctx = context.WithValue(ctx, authCtxKey, &AuthInfo{SubjectID: "sub"})
		}
		info := &grpc.UnaryServerInfo{FullMethod: method}
		h := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }
		_, err := s.rbacUnaryInterceptor()(ctx, nil, info, h)
		return err
	}

	// without subject should be denied
	err = call(false, "/rune.api.ServiceService/GetService")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied without subject, got %v", err)
	}
	// with subject allowed (placeholder until policy engine wired)
	if err := call(true, "/rune.api.ServiceService/CreateService"); err != nil {
		t.Fatalf("expected allow with subject: %v", err)
	}
}

// Test RBAC on stream interceptor for logs (read) vs exec (write)
func TestRBACStreamInterceptor(t *testing.T) {
	s, err := New(WithAuth(nil))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	call := func(hasSubject bool, method string) error {
		ctx := context.Background()
		if hasSubject {
			ctx = context.WithValue(ctx, authCtxKey, &AuthInfo{SubjectID: "sub"})
		}
		ss := &fakeServerStream{ctx: ctx}
		info := &grpc.StreamServerInfo{FullMethod: method}
		h := func(srv interface{}, stream grpc.ServerStream) error { return nil }
		return s.rbacStreamInterceptor()(nil, ss, info, h)
	}

	// without subject should be denied
	err = call(false, "/rune.api.LogService/StreamLogs")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied without subject, got %v", err)
	}
	// with subject allowed (placeholder)
	if err := call(true, "/rune.api.ExecService/StreamExec"); err != nil {
		t.Fatalf("expected allow with subject: %v", err)
	}
}

type fakeServerStream struct{ ctx context.Context }

func (f *fakeServerStream) SetHeader(md metadata.MD) error  { return nil }
func (f *fakeServerStream) SendHeader(md metadata.MD) error { return nil }
func (f *fakeServerStream) SetTrailer(md metadata.MD)       {}
func (f *fakeServerStream) Context() context.Context        { return f.ctx }
func (f *fakeServerStream) SendMsg(m interface{}) error     { return nil }
func (f *fakeServerStream) RecvMsg(m interface{}) error     { return nil }
