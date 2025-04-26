//go:build unit
// +build unit

// go:build unit

package runner

import (
	"testing"
)

// TestLogOptionsDefaults ensures the LogOptions have correct defaults
func TestLogOptionsDefaults(t *testing.T) {
	options := LogOptions{}

	// Check default values
	if options.Follow {
		t.Error("Expected Follow to be false by default")
	}

	if options.Tail != 0 {
		t.Errorf("Expected Tail to be 0 by default, got %d", options.Tail)
	}

	if options.Timestamps {
		t.Error("Expected Timestamps to be false by default")
	}
}

// TestExecOptions ensures ExecOptions works as expected
func TestExecOptions(t *testing.T) {
	// Create exec options with command
	options := ExecOptions{
		Command: []string{"ls", "-la"},
		Env:     map[string]string{"TEST": "value"},
		TTY:     true,
	}

	// Verify options
	if len(options.Command) != 2 {
		t.Errorf("Expected command length 2, got %d", len(options.Command))
	}

	if options.Command[0] != "ls" || options.Command[1] != "-la" {
		t.Errorf("Command doesn't match expected values")
	}

	if val, ok := options.Env["TEST"]; !ok || val != "value" {
		t.Errorf("Environment variable not set correctly")
	}

	if !options.TTY {
		t.Error("Expected TTY to be true")
	}
}
