package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// KEKSource defines how to load the master key (KEK)
type KEKSource string

const (
	KEKSourceFile      KEKSource = "file"
	KEKSourceEnv       KEKSource = "env"
	KEKSourceGenerated KEKSource = "generated"
)

// KEKOptions holds configuration for loading the KEK
type KEKOptions struct {
	Source            KEKSource
	FilePath          string
	EnvVar            string // e.g., RUNE_MASTER_KEY
	GenerateIfMissing bool
}

// LoadOrGenerateKEK loads a 32-byte KEK according to options. File/env values
// are expected to be base64-encoded 32 bytes. If Source is generated or
// GenerateIfMissing is true, a new random key will be produced and, if FilePath
// is set, persisted with permissions 0600.
func LoadOrGenerateKEK(opts KEKOptions) ([]byte, error) {
	switch opts.Source {
	case KEKSourceFile:
		if opts.FilePath == "" {
			return nil, errors.New("kek file path is required")
		}
		b64, err := os.ReadFile(opts.FilePath)
		if err != nil {
			if opts.GenerateIfMissing && errors.Is(err, os.ErrNotExist) {
				return generateAndPersistKEK(opts.FilePath)
			}
			return nil, fmt.Errorf("failed to read kek file: %w", err)
		}
		key, err := decodeB64Key(string(trimSpaceBytes(b64)))
		if err != nil {
			return nil, fmt.Errorf("invalid kek file: %w", err)
		}
		return key, nil
	case KEKSourceEnv:
		if opts.EnvVar == "" {
			return nil, errors.New("kek env var is required")
		}
		val := os.Getenv(opts.EnvVar)
		if val == "" {
			if opts.GenerateIfMissing && opts.FilePath != "" {
				return generateAndPersistKEK(opts.FilePath)
			}
			return nil, fmt.Errorf("env var %s is empty", opts.EnvVar)
		}
		return decodeB64Key(val)
	case KEKSourceGenerated:
		return randomKey(32)
	default:
		return nil, fmt.Errorf("unknown kek source: %s", opts.Source)
	}
}

func generateAndPersistKEK(path string) ([]byte, error) {
	key, err := randomKey(32)
	if err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString(key)
	// Ensure directory exists
	if err := os.MkdirAll(dirOf(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create dir for kek: %w", err)
	}
	// Write with 0600 permissions
	if err := os.WriteFile(path, []byte(b64), fs.FileMode(0600)); err != nil {
		return nil, fmt.Errorf("failed to write kek file: %w", err)
	}
	return key, nil
}

func randomKey(n int) ([]byte, error) {
	key := make([]byte, n)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func decodeB64Key(v string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: got %d, want 32", len(key))
	}
	return key, nil
}

func trimSpaceBytes(b []byte) []byte {
	// simple manual trim to avoid importing bytes
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}

func dirOf(path string) string {
	// minimal split, avoids importing filepath for simple case
	last := -1
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			last = i
		}
	}
	if last <= 0 {
		return "."
	}
	return path[:last]
}
