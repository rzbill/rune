package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

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

func TestRenderer_DepthLimit(t *testing.T) {
	root := t.TempDir()
	// c1 -> c2 -> c3 chain
	writeFile(t, root, "templates/c1.tmpl", "{{ template:templates/c2.tmpl }}\n")
	writeFile(t, root, "templates/c2.tmpl", "{{ template:templates/c3.tmpl }}\n")
	writeFile(t, root, "templates/c3.tmpl", "ok: 1\n")
	r := newTestRenderer(root, map[string]interface{}{}, map[string]interface{}{})
	r.MaxDepth = 1
	_, err := r.RenderString("{{ template:templates/c1.tmpl }}\n", 0)
	if err == nil || !strings.Contains(err.Error(), "max depth") {
		t.Fatalf("expected max depth error, got: %v", err)
	}
}

func TestRenderer_InlineAndWholeNode(t *testing.T) {
	root := t.TempDir()
	values := map[string]interface{}{
		"app": map[string]interface{}{
			"name":   "webapp",
			"config": map[string]interface{}{"k": "v"},
		},
	}
	ctx := map[string]interface{}{}
	in := "foo: {{ values:app.name }}\nbar: {{ values:app.config }}\n"
	r := newTestRenderer(root, values, ctx)
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	expected := "foo: webapp\nbar:\n  k: v\n"
	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Fatalf("unexpected output:\n%s\n-- expected --\n%s", out, expected)
	}
}

func TestRenderer_FileInclude_Rendered(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "templates/app.env", "APP_NAME={{ values:app.name }}\nLOG_LEVEL={{ values:app.log }}\n")
	values := map[string]interface{}{
		"app": map[string]interface{}{
			"name": "webapp",
			"log":  "info",
		},
	}
	ctx := map[string]interface{}{}
	in := "env: {{ file: templates/app.env }}\n"
	r := newTestRenderer(root, values, ctx)
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "env: |-") || !strings.Contains(out, "APP_NAME=webapp") || !strings.Contains(out, "LOG_LEVEL=info") {
		t.Fatalf("file include not rendered correctly:\n%s", out)
	}
}

func TestRenderer_TemplateInclude_Rendered(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "templates/partial.tmpl", "foo: {{ values:app.name }}\n")
	values := map[string]interface{}{"app": map[string]interface{}{"name": "web"}}
	ctx := map[string]interface{}{}
	in := "{{ template:templates/partial.tmpl }}\n"
	r := newTestRenderer(root, values, ctx)
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(out) != "foo: web" {
		t.Fatalf("unexpected template render: %q", out)
	}
}

func TestRenderer_CycleDetection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "templates/a.tmpl", "{{ template:templates/b.tmpl }}\n")
	writeFile(t, root, "templates/b.tmpl", "{{ template:templates/a.tmpl }}\n")
	values := map[string]interface{}{}
	ctx := map[string]interface{}{}
	in := "{{ template:templates/a.tmpl }}\n"
	r := newTestRenderer(root, values, ctx)
	_, err := r.RenderString(in, 0)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle detection error, got: %v", err)
	}
}

func TestRenderer_PathTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "outside.txt")
	_ = os.WriteFile(outside, []byte("secret"), 0o644)
	in := "env: {{ file: ../outside.txt }}\n"
	r := newTestRenderer(root, map[string]interface{}{}, map[string]interface{}{})
	_, err := r.RenderString(in, 0)
	if err == nil || !strings.Contains(err.Error(), "escapes runeset root") {
		t.Fatalf("expected path traversal error, got: %v", err)
	}
}

func TestRenderer_MissingKey(t *testing.T) {
	root := t.TempDir()
	in := "foo: {{ values:missing.path }}\n"
	r := newTestRenderer(root, map[string]interface{}{}, map[string]interface{}{})
	_, err := r.RenderString(in, 0)
	if err == nil || !strings.Contains(err.Error(), "missing values:missing.path") {
		t.Fatalf("expected missing key error, got: %v", err)
	}
}

func TestRenderer_ContextUsage(t *testing.T) {
	root := t.TempDir()
	ctx := map[string]interface{}{"runeset": map[string]interface{}{"name": "example-app"}}
	in := "note: {{ context:runeset.name }}\n"
	r := newTestRenderer(root, map[string]interface{}{}, ctx)
	out, err := r.RenderString(in, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(out) != "note: example-app" {
		t.Fatalf("unexpected context render: %q", out)
	}
}
