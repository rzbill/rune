package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	// Set specific values for testing
	origVersion := Version
	origBuildTime := BuildTime
	origCommit := Commit

	defer func() {
		// Restore original values
		Version = origVersion
		BuildTime = origBuildTime
		Commit = origCommit
	}()

	Version = "1.0.0"
	BuildTime = "2023-01-01"
	Commit = "abcdef0123456789"

	info := Info()

	// Check that the returned info contains the expected values
	if !strings.Contains(info, "1.0.0") {
		t.Errorf("Expected info to contain version, got: %s", info)
	}

	if !strings.Contains(info, "abcdef01") {
		t.Errorf("Expected info to contain commit short SHA, got: %s", info)
	}

	if !strings.Contains(info, "2023-01-01") {
		t.Errorf("Expected info to contain build time, got: %s", info)
	}

	if !strings.Contains(info, runtime.GOOS) {
		t.Errorf("Expected info to contain OS, got: %s", info)
	}

	if !strings.Contains(info, runtime.GOARCH) {
		t.Errorf("Expected info to contain architecture, got: %s", info)
	}

	// Test with short commit
	Commit = "abc123"
	info = Info()
	if !strings.Contains(info, "abc123") {
		t.Errorf("Expected info to contain short commit as is, got: %s", info)
	}
}

func TestMap(t *testing.T) {
	// Set specific values for testing
	origVersion := Version
	origBuildTime := BuildTime
	origCommit := Commit

	defer func() {
		// Restore original values
		Version = origVersion
		BuildTime = origBuildTime
		Commit = origCommit
	}()

	Version = "1.0.0"
	BuildTime = "2023-01-01"
	Commit = "abcdef0123456789"

	m := Map()

	// Check that the returned map contains the expected keys and values
	if m["version"] != "1.0.0" {
		t.Errorf("Expected version to be 1.0.0, got: %s", m["version"])
	}

	if m["buildTime"] != "2023-01-01" {
		t.Errorf("Expected buildTime to be 2023-01-01, got: %s", m["buildTime"])
	}

	if m["commit"] != "abcdef0123456789" {
		t.Errorf("Expected commit to be abcdef0123456789, got: %s", m["commit"])
	}

	if m["os"] != runtime.GOOS {
		t.Errorf("Expected os to be %s, got: %s", runtime.GOOS, m["os"])
	}

	if m["arch"] != runtime.GOARCH {
		t.Errorf("Expected arch to be %s, got: %s", runtime.GOARCH, m["arch"])
	}

	if !strings.HasPrefix(m["goVersion"], "go1.") {
		t.Errorf("Expected goVersion to start with go1., got: %s", m["goVersion"])
	}
}
