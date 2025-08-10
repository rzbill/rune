package types

import (
	"os"
	"path/filepath"
	"testing"
)

// Test parsing a cast YAML that includes a singular service plus plural
// secrets and configMaps. Ensures we get a populated CastFile back.
func TestParseCastFile_MixedForms(t *testing.T) {
	t.Parallel()

	yamlContent := `
 # single service
 service:
   name: api
   image: nginx:alpine
   ports:
     - name: http
       port: 80

 # multiple services
 services:
   - name: frontend
     image: nginx:alpine
     ports:
       - name: http-frontend
         port: 80
   - name: worker
     image: busybox
     scale: 2
     ports:
       - name: http-worker
         port: 80
   - name: cron
     image: busybox
     scale: 1

 # multiple secrets
 secrets:
   - name: db-password
     type: static
     data:
       password: s3cr3t
   - name: redis-password
     type: static
     data:
       password: s3cr3t-redis
   - name: mongo-password
     type: static
     data:
       password: s3cr3t-mongo

 # multiple configmaps
 configMaps:
   - name: app-config
     data:
       LOG_LEVEL: debug
   - name: web-config
     data:
       LOG_LEVEL: debug
   - name: api-config
     data:
       LOG_LEVEL: debug
 `

	dir := t.TempDir()
	file := filepath.Join(dir, "cast.yaml")
	if err := os.WriteFile(file, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	cf, err := ParseCastFile(file)
	if err != nil {
		t.Fatalf("ParseCastFile returned error: %v", err)
	}
	if cf == nil {
		t.Fatalf("ParseCastFile returned nil CastFile")
	}

	// Services should include one from singular 'service' and three from 'services' list
	if len(cf.Services) != 4 {
		t.Fatalf("expected 4 services, got %d", len(cf.Services))
	}
	// quick sanity on names
	names := map[string]bool{}
	for _, s := range cf.Services {
		names[s.Name] = true
	}
	for _, want := range []string{"api", "frontend", "worker", "cron"} {
		if !names[want] {
			t.Fatalf("expected service %q present, got names: %+v", want, names)
		}
	}

	// Secrets parsed from 'secrets' list
	if len(cf.Secrets) != 3 {
		t.Fatalf("expected 3 secret specs, got %d", len(cf.Secrets))
	}
	secretNames := map[string]bool{}
	for _, s := range cf.Secrets {
		secretNames[s.Name] = true
	}
	for _, want := range []string{"db-password", "redis-password", "mongo-password"} {
		if !secretNames[want] {
			t.Fatalf("expected secret %q present, got names: %+v", want, secretNames)
		}
	}

	// ConfigMaps parsed from 'configMaps' list
	if len(cf.ConfigMaps) != 3 {
		t.Fatalf("expected 3 configmap specs, got %d", len(cf.ConfigMaps))
	}
	cmNames := map[string]bool{}
	for _, c := range cf.ConfigMaps {
		cmNames[c.Name] = true
	}
	for _, want := range []string{"app-config", "web-config", "api-config"} {
		if !cmNames[want] {
			t.Fatalf("expected configmap %q present, got names: %+v", want, cmNames)
		}
	}

	// Validate conversion helpers
	secrets, err := cf.GetSecrets()
	if err != nil {
		t.Fatalf("GetSecrets returned error: %v", err)
	}
	if len(secrets) != 3 {
		t.Fatalf("expected 3 concrete secrets, got %d", len(secrets))
	}

	configs, err := cf.GetConfigMaps()
	if err != nil {
		t.Fatalf("GetConfigMaps returned error: %v", err)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3 concrete configmaps, got %d", len(configs))
	}
}

func TestParseCastFile(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "single service",
			yaml: `
service:
  name: test-service
  image: nginx:latest
  scale: 2
`,
			wantErr: false,
		},
		{
			name: "multiple services",
			yaml: `
services:
  - name: service-1
    image: nginx:latest
    scale: 1
  - name: service-2
    image: redis:latest
    scale: 3
`,
			wantErr: false,
		},
		{
			name: "no resources to parse",
			yaml: `
foo: bar
`,
			wantErr: true,
		},
		{
			name: "invalid yaml",
			yaml: `
service:
  name: test-service
  image: nginx:latest
  scale: 2
  - invalid
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write yaml to a temp file because ParseCastFile expects a filename
			dir := t.TempDir()
			fp := filepath.Join(dir, "case.yaml")
			if err := os.WriteFile(fp, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("failed to write temp yaml: %v", err)
			}
			cf, err := ParseCastFile(fp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCastFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Check that we have services
			services, err := cf.GetServices()
			if err != nil {
				t.Errorf("GetServices() error = %v", err)
				return
			}
			if len(services) == 0 {
				t.Error("No services found in parsed file")
			}
		})
	}
}

func TestParseCastFile_FullService(t *testing.T) {
	// Skip if short testing is enabled
	if testing.Short() {
		t.Skip("skipping file I/O test in short mode")
	}

	// Create a temporary file for testing
	content := `
service:
  name: test-service
  namespace: test-ns
  image: nginx:latest
  scale: 2
  env:
    FOO: bar
    BAR: baz
  ports:
    - name: http
      port: 80
    - name: https
      port: 443
  health:
    liveness:
      type: http
      path: /health
      port: 8080
  expose:
    port: http
    host: example.com
  autoscale:
    enabled: true
    min: 2
    max: 10
    metric: cpu
    target: 70%
  affinity:
    required:
      - region=us-east-1
    preferred:
      - zone=us-east-1a
  discovery:
    mode: load-balanced
`

	tmpDir, err := os.MkdirTemp("", "rune-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Test parsing the file
	cf, err := ParseCastFile(tmpFile)
	if err != nil {
		t.Fatalf("ParseCastFile() error = %v", err)
	}

	// Verify the parsed content
	if cf.Services == nil {
		t.Fatal("Expected single service but got nil")
	}

	service := cf.Services[0]
	if service.Name != "test-service" {
		t.Errorf("Service name = %v, want %v", service.Name, "test-service")
	}

	if service.Namespace != "test-ns" {
		t.Errorf("Service namespace = %v, want %v", service.Namespace, "test-ns")
	}

	if service.Image != "nginx:latest" {
		t.Errorf("Service image = %v, want %v", service.Image, "nginx:latest")
	}

	if service.Scale != 2 {
		t.Errorf("Service scale = %v, want %v", service.Scale, 2)
	}

	if len(service.Env) != 2 {
		t.Errorf("Service env count = %v, want %v", len(service.Env), 2)
	}

	if service.Env["FOO"] != "bar" {
		t.Errorf("Service env FOO = %v, want %v", service.Env["FOO"], "bar")
	}

	// Check ports
	if len(service.Ports) != 2 {
		t.Errorf("Service ports count = %v, want %v", len(service.Ports), 2)
	} else {
		if service.Ports[0].Name != "http" || service.Ports[0].Port != 80 {
			t.Errorf("Port 'http' not parsed correctly: %+v", service.Ports[0])
		}
		if service.Ports[1].Name != "https" || service.Ports[1].Port != 443 {
			t.Errorf("Port 'https' not parsed correctly: %+v", service.Ports[1])
		}
	}

	if service.Health == nil || service.Health.Liveness == nil {
		t.Error("Expected health check but got nil")
	} else {
		probe := service.Health.Liveness
		if probe.Type != "http" || probe.Path != "/health" || probe.Port != 8080 {
			t.Errorf("Health check not parsed correctly: %+v", probe)
		}
	}

	// Check expose
	if service.Expose == nil {
		t.Error("Expected expose but got nil")
	} else {
		if service.Expose.Port != "http" || service.Expose.Host != "example.com" {
			t.Errorf("Expose not parsed correctly: %+v", service.Expose)
		}
	}

	// Check autoscale
	if service.Autoscale == nil {
		t.Error("Expected autoscale but got nil")
	} else {
		if !service.Autoscale.Enabled || service.Autoscale.Min != 2 || service.Autoscale.Max != 10 ||
			service.Autoscale.Metric != "cpu" || service.Autoscale.Target != "70%" {
			t.Errorf("Autoscale not parsed correctly: %+v", service.Autoscale)
		}
	}

	// Check affinity
	if service.Affinity == nil {
		t.Error("Expected affinity but got nil")
	} else {
		if len(service.Affinity.Required) != 1 || service.Affinity.Required[0] != "region=us-east-1" {
			t.Errorf("Affinity required not parsed correctly: %+v", service.Affinity.Required)
		}
		if len(service.Affinity.Preferred) != 1 || service.Affinity.Preferred[0] != "zone=us-east-1a" {
			t.Errorf("Affinity preferred not parsed correctly: %+v", service.Affinity.Preferred)
		}
	}

	// Check discovery
	if service.Discovery == nil {
		t.Error("Expected discovery but got nil")
	} else {
		if service.Discovery.Mode != "load-balanced" {
			t.Errorf("Discovery mode not parsed correctly: %+v", service.Discovery.Mode)
		}
	}

	// Test parsing a non-existent file
	_, err = ParseCastFile(filepath.Join(tmpDir, "non-existent.yaml"))
	if err == nil {
		t.Error("Expected error when parsing non-existent file, got nil")
	}
}

func TestParseCastFile_FullSecret(t *testing.T) {
	t.Parallel()
	yamlContent := `
secrets:
  - name: api-key
    type: static
    data:
      key: abc123
  - name: dyn-token
    type: dynamic
    engine:
      name: vault
    rotation:
      interval: 30d
`
	dir := t.TempDir()
	fp := filepath.Join(dir, "secrets.yaml")
	if err := os.WriteFile(fp, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}
	cf, err := ParseCastFile(fp)
	if err != nil {
		t.Fatalf("ParseCastFile returned error: %v", err)
	}
	if len(cf.Secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(cf.Secrets))
	}
	// Validate conversion
	secs, err := cf.GetSecrets()
	if err != nil {
		t.Fatalf("GetSecrets error: %v", err)
	}
	if len(secs) != 2 {
		t.Fatalf("expected 2 concrete secrets, got %d", len(secs))
	}
}

func TestParseCastFile_FullConfigMap(t *testing.T) {
	t.Parallel()
	yamlContent := `
configMaps:
  - name: app-config
    data:
      LOG_LEVEL: info
  - name: api-config
    data:
      TIMEOUT: "5s"
`
	dir := t.TempDir()
	fp := filepath.Join(dir, "configs.yaml")
	if err := os.WriteFile(fp, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}
	cf, err := ParseCastFile(fp)
	if err != nil {
		t.Fatalf("ParseCastFile returned error: %v", err)
	}
	if len(cf.ConfigMaps) != 2 {
		t.Fatalf("expected 2 configmaps, got %d", len(cf.ConfigMaps))
	}
	// Validate conversion
	cfgs, err := cf.GetConfigMaps()
	if err != nil {
		t.Fatalf("GetConfigMaps error: %v", err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("expected 2 concrete configmaps, got %d", len(cfgs))
	}
}

func TestCastFile_ServiceUnknownFields(t *testing.T) {
	t.Parallel()
	yamlContent := `
services:
  - name: s1
    image: nginx
    badField: oops
`
	dir := t.TempDir()
	fp := filepath.Join(dir, "svc_bad.yaml")
	if err := os.WriteFile(fp, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}
	cf, err := ParseCastFile(fp)
	if err != nil {
		t.Fatalf("ParseCastFile returned error: %v", err)
	}
	errs := cf.Lint()
	if len(errs) == 0 {
		t.Fatalf("expected structural validation errors, got none")
	}
}
