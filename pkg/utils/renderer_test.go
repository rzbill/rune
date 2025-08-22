package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func newTestRenderer(root string, values map[string]interface{}, ctx map[string]interface{}) *Renderer {
	return &Renderer{
		Root:          root,
		Values:        values,
		Ctx:           ctx,
		FileCache:     map[string]string{},
		TemplateCache: map[string]string{},
		Stack:         nil,
		MaxDepth:      8,
		PhRegex:       regexp.MustCompile(`\{\{(.*?)\}\}`),
	}
}

func TestDefaultFilterScalar(t *testing.T) {
	r := newTestRenderer(t.TempDir(), map[string]interface{}{}, map[string]interface{}{})
	in := "replicas: {{ values:app.replicas | default: 1 }}\n"
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(out) != "replicas: 1" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestDefaultFilterMapWholeNode(t *testing.T) {
	r := newTestRenderer(t.TempDir(), map[string]interface{}{}, map[string]interface{}{})
	in := "labels: {{ values:app.labels | default: { tier: \"web\" } }}\n"
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	expected := "labels:\n  tier: web\n"
	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Fatalf("unexpected output:\n%s\n-- expected --\n%s", out, expected)
	}
}

func TestDefaultFilterExistingValueUnchanged(t *testing.T) {
	r := newTestRenderer(t.TempDir(), map[string]interface{}{
		"app": map[string]interface{}{"replicas": 3},
	}, map[string]interface{}{})
	in := "replicas: {{ values:app.replicas | default: 1 }}\n"
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(out) != "replicas: 3" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPassThroughSecretAndConfigmap(t *testing.T) {
	r := newTestRenderer(t.TempDir(), map[string]interface{}{}, map[string]interface{}{})
	in := "env:\n  LOG_LEVEL: {{ configmap: app/logLevel }}\n  DB_PASSWORD: {{ secret: db/password }}\n"
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "{{ configmap: app/logLevel }}") || !strings.Contains(out, "{{ secret: db/password }}") {
		t.Fatalf("placeholders should pass through: %q", out)
	}
}

func TestFileIncludeDefaultWithinFile(t *testing.T) {
	dir := t.TempDir()
	// file content includes a default filter
	p := filepath.Join(dir, "templates", "vars.env")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte("REPLICAS={{ values:app.replicas | default: 2 }}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := newTestRenderer(dir, map[string]interface{}{}, map[string]interface{}{})
	in := "env: {{ file: templates/vars.env }}\n"
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "REPLICAS=2") {
		t.Fatalf("expected default to apply inside included file: %q", out)
	}
}
