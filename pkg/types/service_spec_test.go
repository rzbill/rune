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
		t.Fatalf("expected validate error, got nil")
	}
}

func TestServiceSpec_EnvFrom_Normalization(t *testing.T) {
	yamlData := `
name: api
image: repo/api
namespace: default
envFrom:
  - secret: app-secrets
    prefix: APP_
  - configMap: app-config
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
	if len(svc.EnvFrom) != 2 {
		t.Fatalf("expected 2 envFrom entries, got %d", len(svc.EnvFrom))
	}
	if svc.EnvFrom[0].SecretName != "app-secrets" || svc.EnvFrom[0].Namespace != "default" || svc.EnvFrom[0].Prefix != "APP_" {
		t.Errorf("unexpected first envFrom: %+v", svc.EnvFrom[0])
	}
	if svc.EnvFrom[1].ConfigmapName != "app-config" || svc.EnvFrom[1].Namespace != "default" {
		t.Errorf("unexpected second envFrom: %+v", svc.EnvFrom[1])
	}
}

func TestServiceSpec_EnvFrom_Validation(t *testing.T) {
	// both secret and configMap set -> error
	yamlData := `
name: api
image: repo/api
envFrom:
  - secret: a
    configMap: b
`
	var spec ServiceSpec
	if err := yaml.Unmarshal([]byte(yamlData), &spec); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if err := spec.Validate(); err == nil {
		t.Fatalf("expected validate error, got nil")
	}
}

func TestServiceSpec_EnvFrom_Shorthand_Unmarshal(t *testing.T) {
	yamlData := `
service:
  name: api
  image: repo/api
  envFrom: {{secret:env-secret}}
`
	cf, err := ParseCastFileFromBytes([]byte(yamlData))
	if err != nil {
		t.Fatalf("parse cast file error: %v", err)
	}
	services, err := cf.GetServices()
	if err != nil {
		t.Fatalf("get services error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	svc := services[0]
	if len(svc.EnvFrom) != 1 {
		t.Fatalf("expected 1 envFrom entries, got %d", len(svc.EnvFrom))
	}
	if svc.EnvFrom[0].SecretName != "env-secret" {
		t.Errorf("expected envFrom secret=env-secret, got %+v", svc.EnvFrom[0])
	}
}

func TestServiceSpec_EnvFrom_Mixed_Shorthand_Unmarshal(t *testing.T) {
	yamlData := `
service:
  name: api
  image: repo/api
  envFrom:
    - {{secret:env-secret}}
    - configmap: env-config
      prefix: APP_
  env:
    APP_MODE: production
`
	cf, err := ParseCastFileFromBytes([]byte(yamlData))
	if err != nil {
		t.Fatalf("parse cast file error: %v", err)
	}
	services, err := cf.GetServices()
	if err != nil {
		t.Fatalf("get services error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	svc := services[0]
	if len(svc.EnvFrom) != 2 {
		t.Fatalf("expected 2 envFrom entries, got %d", len(svc.EnvFrom))
	}
	if svc.EnvFrom[0].SecretName != "env-secret" {
		t.Errorf("expected envFrom secret=env-secret, got %+v", svc.EnvFrom[0])
	}
	if svc.EnvFrom[1].ConfigmapName != "env-config" {
		t.Errorf("expected envFrom configmap=env-config, got %+v", svc.EnvFrom[1])
	}
	if svc.EnvFrom[1].Prefix != "APP_" {
		t.Errorf("expected envFrom prefix=APP_, got %+v", svc.EnvFrom[1])
	}
	if svc.Env["APP_MODE"] != "production" {
		t.Errorf("expected env APP_MODE=production, got %+v", svc.Env)
	}
}
