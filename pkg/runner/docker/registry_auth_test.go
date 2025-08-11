package docker

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func decodeAuth(b64 string) (map[string]string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func TestParseImageHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ghcr.io/acme/app:1.0", "ghcr.io"},
		{"123456789012.dkr.ecr.us-east-1.amazonaws.com/repo:tag", "123456789012.dkr.ecr.us-east-1.amazonaws.com"},
		{"nginx:alpine", "index.docker.io"},
		{"localhost:5000/repo", "localhost:5000"},
	}
	for _, c := range cases {
		got := parseImageHost(c.in)
		if got != c.want {
			t.Fatalf("parseImageHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchWildcardHost(t *testing.T) {
	if !matchWildcardHost("*.dkr.ecr.us-east-1.amazonaws.com", "123456789012.dkr.ecr.us-east-1.amazonaws.com") {
		t.Fatal("wildcard should match ECR host")
	}
	if matchWildcardHost("*.example.com", "repo.example.org") {
		t.Fatal("wildcard should not match different TLD")
	}
}

func TestResolveRegistryAuth_BasicExactAndWildcard(t *testing.T) {
	r := &DockerRunner{
		logger: nil,
		config: &DockerConfig{Registries: []RegistryConfig{
			{Registry: "ghcr.io", Auth: RegistryAuth{Type: "basic", Username: "u", Password: "p"}},
			{Registry: "*.internal.registry.local", Auth: RegistryAuth{Type: "basic", Username: "wu", Password: "wp"}},
		}},
	}

	// exact host
	auth := r.resolveRegistryAuth("ghcr.io/acme/app:1.0")
	if auth == "" {
		t.Fatal("expected non-empty auth for ghcr.io")
	}
	m, err := decodeAuth(auth)
	if err != nil {
		t.Fatal(err)
	}
	if m["username"] != "u" || m["password"] != "p" || !strings.Contains(m["serveraddress"], "ghcr.io") {
		t.Fatalf("unexpected auth payload: %+v", m)
	}

	// wildcard host
	auth2 := r.resolveRegistryAuth("a.internal.registry.local/team/app:2")
	if auth2 == "" {
		t.Fatal("expected non-empty auth for wildcard")
	}
	m2, err := decodeAuth(auth2)
	if err != nil {
		t.Fatal(err)
	}
	if m2["username"] != "wu" || m2["password"] != "wp" {
		t.Fatalf("unexpected wildcard auth payload: %+v", m2)
	}

	// docker hub (not configured) should be empty
	if got := r.resolveRegistryAuth("nginx:alpine"); got != "" {
		t.Fatalf("expected empty auth for docker hub, got %q", got)
	}
}
