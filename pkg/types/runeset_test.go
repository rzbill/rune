package types

import (
	"os"
	"path/filepath"
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

func TestParseRunesetManifest_OK(t *testing.T) {
	root := t.TempDir()
	manifest := "" +
		"name: example\n" +
		"version: 0.1.0\n" +
		"description: test\n" +
		"namespace: demo\n" +
		"defaults:\n  app:\n    name: web\n"
	mp := writeFile(t, root, "runeset.yaml", manifest)
	mf, err := ParseRunesetManifest(mp)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if mf.Name != "example" || mf.Version != "0.1.0" || mf.Namespace != "demo" {
		t.Fatalf("unexpected manifest: %+v", mf)
	}
	if mf.Defaults == nil || mf.Defaults["app"] == nil {
		t.Fatalf("defaults not parsed: %+v", mf.Defaults)
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
	out, err := RenderRunesetYAML(in, root, values, ctx)
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
	out, err := RenderRunesetYAML(in, root, values, ctx)
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
	out, err := RenderRunesetYAML(in, root, values, ctx)
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
	_, err := RenderRunesetYAML(in, root, values, ctx)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle detection error, got: %v", err)
	}
}

func TestRenderer_DepthLimit(t *testing.T) {
	root := t.TempDir()
	// c1 -> c2 -> c3 chain
	writeFile(t, root, "templates/c1.tmpl", "{{ template:templates/c2.tmpl }}\n")
	writeFile(t, root, "templates/c2.tmpl", "{{ template:templates/c3.tmpl }}\n")
	writeFile(t, root, "templates/c3.tmpl", "ok: 1\n")
	r := NewRunesetRenderer(root, map[string]interface{}{}, map[string]interface{}{})
	r.MaxDepth = 1
	_, err := r.RenderString("{{ template:templates/c1.tmpl }}\n", 0)
	if err == nil || !strings.Contains(err.Error(), "max depth") {
		t.Fatalf("expected max depth error, got: %v", err)
	}
}

func TestRenderer_PathTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "outside.txt")
	_ = os.WriteFile(outside, []byte("secret"), 0o644)
	in := "env: {{ file: ../outside.txt }}\n"
	_, err := RenderRunesetYAML(in, root, map[string]interface{}{}, map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "escapes runeset root") {
		t.Fatalf("expected path traversal error, got: %v", err)
	}
}

func TestRenderer_MissingKey(t *testing.T) {
	root := t.TempDir()
	in := "foo: {{ values:missing.path }}\n"
	_, err := RenderRunesetYAML(in, root, map[string]interface{}{}, map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "missing values:missing.path") {
		t.Fatalf("expected missing key error, got: %v", err)
	}
}

func TestRenderer_ContextUsage(t *testing.T) {
	root := t.TempDir()
	ctx := map[string]interface{}{"runeset": map[string]interface{}{"name": "example-app"}}
	in := "note: {{ context:runeset.name }}\n"
	out, err := RenderRunesetYAML(in, root, map[string]interface{}{}, ctx)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(out) != "note: example-app" {
		t.Fatalf("unexpected context render: %q", out)
	}
}
