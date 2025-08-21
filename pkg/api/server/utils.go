package server

import (
	"context"
	"net"
	"reflect"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// methodToAction maps a gRPC method to (resource, verb)
func methodToAction(method string) (string, string) {
	switch {
	case strings.HasPrefix(method, "/rune.api.ServiceService/Get"):
		return "services", "get"
	case strings.HasPrefix(method, "/rune.api.ServiceService/List"):
		return "services", "list"
	case strings.HasPrefix(method, "/rune.api.ServiceService/Create"):
		return "services", "create"
	case strings.HasPrefix(method, "/rune.api.ServiceService/Update"):
		return "services", "update"
	case strings.HasPrefix(method, "/rune.api.ServiceService/Delete"):
		return "services", "delete"
	case strings.HasPrefix(method, "/rune.api.ServiceService/Scale"):
		return "services", "scale"

	case strings.HasPrefix(method, "/rune.api.InstanceService/Get"):
		return "instances", "get"
	case strings.HasPrefix(method, "/rune.api.InstanceService/List"):
		return "instances", "list"
	case strings.HasPrefix(method, "/rune.api.InstanceService/Watch"):
		return "instances", "watch"

	case strings.HasPrefix(method, "/rune.api.LogService/StreamLogs"):
		return "logs", "get"
	case strings.HasPrefix(method, "/rune.api.ExecService/StreamExec"):
		return "exec", "exec"
	case strings.HasPrefix(method, "/rune.api.HealthService/GetHealth"):
		return "health", "get"

	case strings.HasPrefix(method, "/rune.api.ConfigMapService/Get"):
		return "configmaps", "get"
	case strings.HasPrefix(method, "/rune.api.ConfigMapService/List"):
		return "configmaps", "list"
	case strings.HasPrefix(method, "/rune.api.ConfigMapService/Create"):
		return "configmaps", "create"
	case strings.HasPrefix(method, "/rune.api.ConfigMapService/Update"):
		return "configmaps", "update"
	case strings.HasPrefix(method, "/rune.api.ConfigMapService/Delete"):
		return "configmaps", "delete"

	case strings.HasPrefix(method, "/rune.api.SecretService/Get"):
		return "secrets", "get"
	case strings.HasPrefix(method, "/rune.api.SecretService/List"):
		return "secrets", "list"
	case strings.HasPrefix(method, "/rune.api.SecretService/Create"):
		return "secrets", "create"
	case strings.HasPrefix(method, "/rune.api.SecretService/Update"):
		return "secrets", "update"
	case strings.HasPrefix(method, "/rune.api.SecretService/Delete"):
		return "secrets", "delete"

	case strings.HasPrefix(method, "/rune.api.AuthService/WhoAmI"):
		return "auth", "get"
	case strings.HasPrefix(method, "/rune.api.AdminService/"):
		return "admin", "*"
	default:
		return "*", "*"
	}
}

// methodToResource maps a gRPC method to a resource
func methodToResource(method string) string {
	switch {
	case strings.HasPrefix(method, "/rune.api.AdminService/"):
		return "admin"
	default:
		return ""
	}
}

// extractNamespace best-effort extracts a Namespace string field from request
func extractNamespace(req interface{}) string {
	rv := reflect.ValueOf(req)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		f := rv.FieldByName("Namespace")
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
	}
	return ""
}

func peerFromContext(ctx context.Context) (string, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return "", false
	}
	return p.Addr.String(), true
}

func isLocalhost(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}
	return ip.IsLoopback()
}

func statusPermissionDenied(msg string) error { return status.Errorf(codes.PermissionDenied, msg) }
