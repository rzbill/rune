package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureOutput captures stdout during fn execution
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestRuneset_RenderOnly_ExampleApp(t *testing.T) {
	// Arrange
	cwd, err := os.Getwd()
	require.NoError(t, err)
	root := filepath.Join(cwd, "../../../examples/runesets/example-app")
	vals := filepath.Join(root, "values/values.yaml")

	opts := &castOptions{
		valuesFiles: []string{vals},
		renderOnly:  true,
		dryRun:      false,
		namespace:   "demo",
	}

	// Act
	out := captureOutput(func() {
		_ = runRunesetCast(root, opts)
	})

	// Assert (spot-check key expansions and includes)
	require.Contains(t, out, "webapp-api")
	require.Contains(t, out, "configMap:")
	require.Contains(t, out, "README_BANNER")
	require.Contains(t, out, "APP_ENV")
}

func TestRuneset_DryRun_ExampleApp(t *testing.T) {
	// Arrange
	cwd, err := os.Getwd()
	require.NoError(t, err)
	root := filepath.Join(cwd, "../../../examples/runesets/example-app")
	vals := filepath.Join(root, "values/values.yaml")

	opts := &castOptions{
		valuesFiles: []string{vals},
		renderOnly:  false,
		dryRun:      true,
		namespace:   "demo",
	}

	// Act
	out := captureOutput(func() {
		err := runRunesetCast(root, opts)
		fmt.Println(err)
		require.NoError(t, err)
	})

	// Assert
	require.Contains(t, out, "Validation successful!")
}

func TestRuneset_ArchiveRoundTrip_Render(t *testing.T) {
	// create a minimal runeset in tmp and pack it
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "runeset.yaml"), []byte("name: demo\nversion: v1\ndescription: d\ndefaults:\n  namespace: demo\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "casts"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "values"), 0755))
	svc := []byte("service:\n  name: \"{{ values:app.name }}\"\n  namespace: demo\n  image: \"nginx:{{ values:app.tag }}\"\n  scale: {{ values:app.replicas }}\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "casts", "svc.yaml"), svc, 0644))
	vals := []byte("app:\n  name: x\n  tag: latest\n  replicas: 1\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "values", "values.yaml"), vals, 0644))

	// pack using tarGzDir
	archive := filepath.Join(t.TempDir(), "demo.runeset.tgz")
	require.NoError(t, tarGzDir(root, archive))

	// extract and render
	tmpDir, err := extractRunesetArchive(archive)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	opts := &castOptions{
		valuesFiles: []string{filepath.Join(tmpDir, "values", "values.yaml")},
		renderOnly:  true,
	}

	out := captureOutput(func() {
		_ = runRunesetCast(tmpDir, opts)
	})
	require.Contains(t, out, "name: \"x\"")
	require.Contains(t, out, "image: \"nginx:latest\"")
}

func TestExtractRunesetArchive_BlocksTraversal(t *testing.T) {
	// Create malicious archive with .. entry
	archive := filepath.Join(t.TempDir(), "bad.runeset.tgz")
	f, err := os.Create(archive)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	// write file with ../evil
	hdr := &tar.Header{Name: "../evil", Mode: 0644, Size: int64(len("bad"))}
	require.NoError(t, tw.WriteHeader(hdr))
	_, _ = tw.Write([]byte("bad"))
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	_, err = extractRunesetArchive(archive)
	require.Error(t, err)
}
