package crypto

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerateKEK_File_GenerateIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kek.b64")
	key, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile, FilePath: path, GenerateIfMissing: true})
	if err != nil {
		t.Fatalf("LoadOrGenerateKEK: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
	// Ensure file exists and decodes to same key length
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(dec) != 32 {
		t.Fatalf("decoded file key length = %d", len(dec))
	}
}

func TestLoadOrGenerateKEK_Env(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	os.Setenv("RUNE_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	defer os.Unsetenv("RUNE_MASTER_KEY")

	got, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceEnv, EnvVar: "RUNE_MASTER_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 32 {
		t.Fatalf("len=%d", len(got))
	}
}

func TestLoadOrGenerateKEK_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kek.b64")

	// Test that generated file has correct permissions
	_, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile, FilePath: path, GenerateIfMissing: true})
	if err != nil {
		t.Fatalf("LoadOrGenerateKEK: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Should be 0600 (owner read/write only)
	expectedMode := os.FileMode(0600)
	if info.Mode().Perm() != expectedMode {
		t.Fatalf("expected file mode %v, got %v", expectedMode, info.Mode().Perm())
	}

	// Verify we can read the key back
	key2, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile, FilePath: path})
	if err != nil {
		t.Fatalf("failed to read back generated key: %v", err)
	}

	if len(key2) != 32 {
		t.Fatalf("read back key length = %d", len(key2))
	}
}

func TestLoadOrGenerateKEK_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.b64")

	// Should fail when file doesn't exist and GenerateIfMissing is false
	_, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile, FilePath: path})
	if err == nil {
		t.Fatal("expected error when file doesn't exist")
	}
}

func TestLoadOrGenerateKEK_InvalidBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.b64")

	// Write invalid base64 content
	if err := os.WriteFile(path, []byte("not-base64!"), 0600); err != nil {
		t.Fatal(err)
	}

	// Should fail to decode
	_, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile, FilePath: path})
	if err == nil {
		t.Fatal("expected error with invalid base64")
	}
}

func TestLoadOrGenerateKEK_WrongKeyLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrong-length.b64")

	// Write base64 of 16-byte key (too short)
	shortKey := make([]byte, 16)
	for i := range shortKey {
		shortKey[i] = byte(i)
	}
	b64 := base64.StdEncoding.EncodeToString(shortKey)

	if err := os.WriteFile(path, []byte(b64), 0600); err != nil {
		t.Fatal(err)
	}

	// Should fail due to wrong length
	_, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile, FilePath: path})
	if err == nil {
		t.Fatal("expected error with wrong key length")
	}
}

func TestLoadOrGenerateKEK_Generated(t *testing.T) {
	// Test generated source
	key, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceGenerated})
	if err != nil {
		t.Fatalf("LoadOrGenerateKEK: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestLoadOrGenerateKEK_InvalidSource(t *testing.T) {
	// Test invalid source
	_, err := LoadOrGenerateKEK(KEKOptions{Source: "invalid"})
	if err == nil {
		t.Fatal("expected error with invalid source")
	}
}

func TestLoadOrGenerateKEK_MissingRequiredFields(t *testing.T) {
	// Test missing file path
	_, err := LoadOrGenerateKEK(KEKOptions{Source: KEKSourceFile})
	if err == nil {
		t.Fatal("expected error with missing file path")
	}

	// Test missing env var
	_, err = LoadOrGenerateKEK(KEKOptions{Source: KEKSourceEnv})
	if err == nil {
		t.Fatal("expected error with missing env var")
	}
}
