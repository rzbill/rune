package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ServiceSpec
		wantErr bool
	}{
		{
			name: "valid service",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			spec: &ServiceSpec{
				Image: "nginx:latest",
				Scale: 1,
			},
			wantErr: true,
		},
		{
			name: "missing image",
			spec: &ServiceSpec{
				Name:  "test-service",
				Scale: 1,
			},
			wantErr: true,
		},
		{
			name: "negative scale",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: -1,
			},
			wantErr: true,
		},
		{
			name: "zero scale (valid, defaults to 1)",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 0,
			},
			wantErr: false,
		},
		{
			name: "valid with health check",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Health: &HealthCheck{
					Liveness: &Probe{
						Type: "http",
						Path: "/health",
						Port: 8080,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid health check type",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Health: &HealthCheck{
					Liveness: &Probe{
						Type: "invalid",
						Path: "/health",
						Port: 8080,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing http path",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Health: &HealthCheck{
					Liveness: &Probe{
						Type: "http",
						Port: 8080,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Health: &HealthCheck{
					Liveness: &Probe{
						Type: "http",
						Path: "/health",
						Port: 0,
					},
				},
			},
			wantErr: true,
		},
		// New test cases for ports
		{
			name: "valid port configuration",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Ports: []ServicePort{
					{
						Name: "http",
						Port: 80,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "port without name",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Ports: []ServicePort{
					{
						Port: 80,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port number",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Ports: []ServicePort{
					{
						Name: "http",
						Port: 0, // Invalid port
					},
				},
			},
			wantErr: true,
		},
		// Test for autoscale
		{
			name: "valid autoscale configuration",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Autoscale: &ServiceAutoscale{
					Enabled: true,
					Min:     1,
					Max:     5,
					Metric:  "cpu",
					Target:  "80%",
				},
			},
			wantErr: false,
		},
		{
			name: "autoscale with negative min",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Autoscale: &ServiceAutoscale{
					Enabled: true,
					Min:     -1, // Invalid min
					Max:     5,
					Metric:  "cpu",
					Target:  "80%",
				},
			},
			wantErr: true,
		},
		{
			name: "autoscale with max < min",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Autoscale: &ServiceAutoscale{
					Enabled: true,
					Min:     5,
					Max:     3, // Invalid max < min
					Metric:  "cpu",
					Target:  "80%",
				},
			},
			wantErr: true,
		},
		{
			name: "autoscale without metric",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Autoscale: &ServiceAutoscale{
					Enabled: true,
					Min:     1,
					Max:     5,
					Target:  "80%",
				},
			},
			wantErr: true,
		},
		{
			name: "autoscale without target",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Autoscale: &ServiceAutoscale{
					Enabled: true,
					Min:     1,
					Max:     5,
					Metric:  "cpu",
				},
			},
			wantErr: true,
		},
		// Test for expose
		{
			name: "valid expose configuration",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Expose: &ServiceExpose{
					Port: "http",
					Host: "example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "expose without port",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Expose: &ServiceExpose{
					Host: "example.com",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServiceSpec.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceSpec_ToService(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ServiceSpec
		wantErr bool
	}{
		{
			name: "valid service",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
			},
			wantErr: false,
		},
		{
			name: "zero scale stays 0, that's a way to scale to 0",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid spec",
			spec: &ServiceSpec{
				Image: "nginx:latest", // Missing name
			},
			wantErr: true,
		},
		// Tests for new fields
		{
			name: "with namespace",
			spec: &ServiceSpec{
				Name:      "test-service",
				Namespace: "test-ns",
				Image:     "nginx:latest",
				Scale:     1,
			},
			wantErr: false,
		},
		{
			name: "with ports",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Ports: []ServicePort{
					{
						Name: "http",
						Port: 80,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with autoscale",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Autoscale: &ServiceAutoscale{
					Enabled: true,
					Min:     1,
					Max:     5,
					Metric:  "cpu",
					Target:  "80%",
				},
			},
			wantErr: false,
		},
		{
			name: "with network policy",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				NetworkPolicy: &ServiceNetworkPolicy{
					// Basic network policy configuration
					Ingress: []IngressRule{
						{
							From: []NetworkPolicyPeer{
								{
									Service: "frontend",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with affinity",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Affinity: &ServiceAffinity{
					Required:  []string{"region=us-east-1"},
					Preferred: []string{"zone=us-east-1a"},
				},
			},
			wantErr: false,
		},
		{
			name: "with discovery settings",
			spec: &ServiceSpec{
				Name:  "test-service",
				Image: "nginx:latest",
				Scale: 1,
				Discovery: &ServiceDiscovery{
					Mode: "load-balanced",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := tt.spec.ToService()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServiceSpec.ToService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Verify service was created correctly
			if service.Name != tt.spec.Name {
				t.Errorf("Service name = %v, want %v", service.Name, tt.spec.Name)
			}

			if service.Image != tt.spec.Image {
				t.Errorf("Service image = %v, want %v", service.Image, tt.spec.Image)
			}

			if tt.spec.Scale == 0 && service.Scale != 1 {
				t.Errorf("Service scale = %v, want 0 (default for zero)", service.Scale)
			} else if tt.spec.Scale > 0 && service.Scale != tt.spec.Scale {
				t.Errorf("Service scale = %v, want %v", service.Scale, tt.spec.Scale)
			}

			// Check namespace
			if tt.spec.Namespace != "" && service.Namespace != tt.spec.Namespace {
				t.Errorf("Service namespace = %v, want %v", service.Namespace, tt.spec.Namespace)
			} else if tt.spec.Namespace == "" && service.Namespace != "default" {
				t.Errorf("Service namespace = %v, want default", service.Namespace)
			}

			// Check ports
			if len(tt.spec.Ports) > 0 && len(service.Ports) != len(tt.spec.Ports) {
				t.Errorf("Service ports count = %v, want %v", len(service.Ports), len(tt.spec.Ports))
			}

			// Check autoscale
			if tt.spec.Autoscale != nil && service.Autoscale == nil {
				t.Errorf("Service autoscale = nil, want non-nil")
			} else if tt.spec.Autoscale != nil && service.Autoscale != nil {
				if service.Autoscale.Enabled != tt.spec.Autoscale.Enabled {
					t.Errorf("Service autoscale.Enabled = %v, want %v", service.Autoscale.Enabled, tt.spec.Autoscale.Enabled)
				}
				if service.Autoscale.Min != tt.spec.Autoscale.Min {
					t.Errorf("Service autoscale.Min = %v, want %v", service.Autoscale.Min, tt.spec.Autoscale.Min)
				}
				if service.Autoscale.Max != tt.spec.Autoscale.Max {
					t.Errorf("Service autoscale.Max = %v, want %v", service.Autoscale.Max, tt.spec.Autoscale.Max)
				}
			}

			// Check network policy
			if tt.spec.NetworkPolicy != nil && service.NetworkPolicy == nil {
				t.Errorf("Service networkPolicy = nil, want non-nil")
			}

			// Check affinity
			if tt.spec.Affinity != nil && service.Affinity == nil {
				t.Errorf("Service affinity = nil, want non-nil")
			} else if tt.spec.Affinity != nil && service.Affinity != nil {
				if len(service.Affinity.Required) != len(tt.spec.Affinity.Required) {
					t.Errorf("Service affinity.Required count = %v, want %v", len(service.Affinity.Required), len(tt.spec.Affinity.Required))
				}
			}

			// Check discovery
			if tt.spec.Discovery != nil && service.Discovery == nil {
				t.Errorf("Service discovery = nil, want non-nil")
			} else if tt.spec.Discovery != nil && service.Discovery != nil {
				if service.Discovery.Mode != tt.spec.Discovery.Mode {
					t.Errorf("Service discovery.Mode = %v, want %v", service.Discovery.Mode, tt.spec.Discovery.Mode)
				}
			}

			if service.ID == "" {
				t.Error("Service ID was not generated")
			}

			if service.Status != ServiceStatusPending {
				t.Errorf("Service status = %v, want %v", service.Status, ServiceStatusPending)
			}
		})
	}
}

func TestParseServiceData(t *testing.T) {
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
			name: "no services",
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
			serviceFile, err := ParseServiceData([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseServiceData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Check that we have services
			services := serviceFile.GetServices()
			if len(services) == 0 {
				t.Error("No services found in parsed file")
			}
		})
	}
}

func TestServiceFile_GetServices(t *testing.T) {
	tests := []struct {
		name     string
		file     *ServiceFile
		wantSize int
	}{
		{
			name: "single service",
			file: &ServiceFile{
				Service: &ServiceSpec{
					Name:  "test-service",
					Image: "nginx:latest",
				},
			},
			wantSize: 1,
		},
		{
			name: "multiple services",
			file: &ServiceFile{
				Services: []ServiceSpec{
					{
						Name:  "service-1",
						Image: "nginx:latest",
					},
					{
						Name:  "service-2",
						Image: "redis:latest",
					},
				},
			},
			wantSize: 2,
		},
		{
			name: "both single and multiple services",
			file: &ServiceFile{
				Service: &ServiceSpec{
					Name:  "test-service",
					Image: "nginx:latest",
				},
				Services: []ServiceSpec{
					{
						Name:  "service-1",
						Image: "nginx:latest",
					},
					{
						Name:  "service-2",
						Image: "redis:latest",
					},
				},
			},
			wantSize: 3,
		},
		{
			name:     "no services",
			file:     &ServiceFile{},
			wantSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := tt.file.GetServices()
			if len(services) != tt.wantSize {
				t.Errorf("ServiceFile.GetServices() size = %v, want %v", len(services), tt.wantSize)
			}
		})
	}
}

func TestParseServiceFile(t *testing.T) {
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
	serviceFile, err := ParseServiceFile(tmpFile)
	if err != nil {
		t.Fatalf("ParseServiceFile() error = %v", err)
	}

	// Verify the parsed content
	if serviceFile.Service == nil {
		t.Fatal("Expected single service but got nil")
	}

	service := serviceFile.Service
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
	_, err = ParseServiceFile(filepath.Join(tmpDir, "non-existent.yaml"))
	if err == nil {
		t.Error("Expected error when parsing non-existent file, got nil")
	}
}
