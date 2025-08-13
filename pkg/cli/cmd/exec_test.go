package cmd

import (
	"testing"
	"time"
)

func TestParseExecOptions(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		workdir     string
		env         []string
		tty         bool
		noTTY       bool
		timeout     string
		apiServer   string
		apiKey      string
		expectError bool
	}{
		{
			name:        "valid options",
			namespace:   "default",
			workdir:     "/app",
			env:         []string{"DEBUG=true", "LOG_LEVEL=debug"},
			tty:         true,
			timeout:     "30s",
			expectError: false,
		},
		{
			name:        "invalid timeout",
			namespace:   "default",
			timeout:     "invalid",
			expectError: true,
		},
		{
			name:        "invalid env format",
			namespace:   "default",
			env:         []string{"INVALID_ENV"},
			expectError: true,
		},
		{
			name:        "valid env format",
			namespace:   "default",
			env:         []string{"KEY=value"},
			timeout:     "5m",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global variables
			execNamespace = tt.namespace
			execWorkdir = tt.workdir
			execEnv = tt.env
			execTTY = tt.tty
			execNoTTY = tt.noTTY
			execTimeout = tt.timeout
			execAPIServer = tt.apiServer

			options, err := parseExecOptions()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify parsed options
			if options.Namespace != tt.namespace {
				t.Errorf("expected namespace %s, got %s", tt.namespace, options.Namespace)
			}

			if options.Workdir != tt.workdir {
				t.Errorf("expected workdir %s, got %s", tt.workdir, options.Workdir)
			}

			if options.TTY != tt.tty {
				t.Errorf("expected TTY %v, got %v", tt.tty, options.TTY)
			}

			if tt.timeout != "invalid" {
				expectedTimeout, _ := time.ParseDuration(tt.timeout)
				if options.Timeout != expectedTimeout {
					t.Errorf("expected timeout %v, got %v", expectedTimeout, options.Timeout)
				}
			}

			// Verify environment variables
			if len(tt.env) > 0 && tt.env[0] != "INVALID_ENV" {
				for _, env := range tt.env {
					parts := []string{env}
					if len(parts) == 2 {
						if options.Env[parts[0]] != parts[1] {
							t.Errorf("expected env %s=%s, got %s", parts[0], parts[1], options.Env[parts[0]])
						}
					}
				}
			}
		})
	}
}

func TestShouldAllocateTTY(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		expected bool
	}{
		{
			name:     "bash shell",
			command:  []string{"bash"},
			expected: true,
		},
		{
			name:     "sh shell",
			command:  []string{"sh"},
			expected: true,
		},
		{
			name:     "zsh shell",
			command:  []string{"zsh"},
			expected: true,
		},
		{
			name:     "vim editor",
			command:  []string{"vim", "file.txt"},
			expected: true,
		},
		{
			name:     "nano editor",
			command:  []string{"nano", "file.txt"},
			expected: true,
		},
		{
			name:     "top command",
			command:  []string{"top"},
			expected: true,
		},
		{
			name:     "htop command",
			command:  []string{"htop"},
			expected: true,
		},
		{
			name:     "less command",
			command:  []string{"less", "file.txt"},
			expected: true,
		},
		{
			name:     "ls command",
			command:  []string{"ls", "-la"},
			expected: false,
		},
		{
			name:     "python script",
			command:  []string{"python", "script.py"},
			expected: false,
		},
		{
			name:     "empty command",
			command:  []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldAllocateTTY(tt.command)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for command %v", tt.expected, result, tt.command)
			}
		})
	}
}

func TestIsInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected bool
	}{
		{
			name:     "instance ID with pattern",
			target:   "api-instance-123",
			expected: true,
		},
		{
			name:     "instance ID with different service",
			target:   "web-instance-456",
			expected: true,
		},
		{
			name:     "service name",
			target:   "api",
			expected: false,
		},
		{
			name:     "service name with dash",
			target:   "web-api",
			expected: false,
		},
		{
			name:     "empty string",
			target:   "",
			expected: false,
		},
		{
			name:     "instance with different pattern",
			target:   "api-pod-123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInstanceID(tt.target)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for target %s", tt.expected, result, tt.target)
			}
		})
	}
}
