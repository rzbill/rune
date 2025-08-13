package types

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestServiceSpec_Dependencies_Unmarshal_StringAndStructured(t *testing.T) {
	yamlData := `
name: api
image: repo/api:latest
dependencies:
  - "db"                 # same ns
  - "cache.shared.rune"  # cross-ns FQDN
  - service: auth         # structured
    namespace: security
`
	var spec ServiceSpec
	if err := yaml.Unmarshal([]byte(yamlData), &spec); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate error: %v", err)
	}

	svc, err := spec.ToService()
	if err != nil {
		t.Fatalf("ToService error: %v", err)
	}

	if len(svc.Dependencies) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(svc.Dependencies))
	}

	// Default namespace is "default" in ToService when not provided
	// dep0: "db" -> service=db, ns=default
	if svc.Dependencies[0].Service != "db" || svc.Dependencies[0].Namespace != "default" {
		t.Errorf("dep0 unexpected: %+v", svc.Dependencies[0])
	}
	// dep1: "cache.shared.rune" -> service=cache, ns=shared
	if svc.Dependencies[1].Service != "cache" || svc.Dependencies[1].Namespace != "shared" {
		t.Errorf("dep1 unexpected: %+v", svc.Dependencies[1])
	}
	// dep2: structured auth/security
	if svc.Dependencies[2].Service != "auth" || svc.Dependencies[2].Namespace != "security" {
		t.Errorf("dep2 unexpected: %+v", svc.Dependencies[2])
	}
}

func TestServiceSpec_Dependencies_Invalid(t *testing.T) {
	yamlData := `
name: api
image: repo/api
dependencies:
  - {}
`
	var spec ServiceSpec
	if err := yaml.Unmarshal([]byte(yamlData), &spec); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if err := spec.Validate(); err == nil {
		t.Fatalf("expected validate error for empty dependency, got nil")
	}
}
