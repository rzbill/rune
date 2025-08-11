package types

import (
	"os"
	"testing"
)

func TestSecretSpec_EnvExpansion(t *testing.T) {
	t.Parallel()

	os.Setenv("TEST_SECRET_VAL", "super-secret")
	os.Setenv("TEST_SECRET_B64", "YmFzZTY0LXN0cmluZw==")
	t.Cleanup(func() {
		os.Unsetenv("TEST_SECRET_VAL")
		os.Unsetenv("TEST_SECRET_B64")
	})

	yamlContent := []byte(`
secret:
  name: demo
  namespace: default
  type: static
  value: ${TEST_SECRET_VAL}
  valueBase64: ${TEST_SECRET_B64}
  data:
    a: ${TEST_SECRET_VAL}
    b: ${TEST_SECRET_B64}
`)

	cf, err := ParseCastFileFromBytes(yamlContent)
	if err != nil {
		t.Fatalf("parse castfile: %v", err)
	}

	secrets, err := cf.GetSecrets()
	if err != nil {
		t.Fatalf("GetSecrets: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	s := secrets[0]
	if s.Data["value"] != "super-secret" {
		t.Fatalf("value not expanded: %q", s.Data["value"])
	}
	if s.Data["valueBase64"] != "YmFzZTY0LXN0cmluZw==" {
		t.Fatalf("valueBase64 not expanded: %q", s.Data["valueBase64"])
	}
	if s.Data["a"] != "super-secret" || s.Data["b"] != "YmFzZTY0LXN0cmluZw==" {
		t.Fatalf("data env expansion failed: %+v", s.Data)
	}
}
