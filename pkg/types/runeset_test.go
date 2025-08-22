package types

import (
	"os"
	"path/filepath"
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
