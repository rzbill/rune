package cmd

import (
	"strings"
	"testing"
)

func TestHasYAMLExtension(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"file.yaml", true},
		{"file.yml", true},
		{"file.YAML", true},
		{"file.YML", true},
		{"file.txt", false},
		{"file", false},
		{"file.yaml.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := hasYAMLExtension(tt.filename)
			if got != tt.want {
				t.Errorf("hasYAMLExtension(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDetermineResourceType(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		wantType        string
		wantErrContains string
	}{
		{
			name:     "service",
			yaml:     `service: { name: test }`,
			wantType: "service",
		},
		{
			name:     "services",
			yaml:     `services: [ { name: test } ]`,
			wantType: "service",
		},
		{
			name:     "job",
			yaml:     `job: { name: test }`,
			wantType: "job",
		},
		{
			name:     "jobs",
			yaml:     `jobs: [ { name: test } ]`,
			wantType: "job",
		},
		{
			name:     "secret",
			yaml:     `secret: { name: test }`,
			wantType: "secret",
		},
		{
			name:     "function",
			yaml:     `function: { name: test }`,
			wantType: "function",
		},
		{
			name:     "config",
			yaml:     `config: { name: test }`,
			wantType: "config",
		},
		{
			name:     "networkPolicy",
			yaml:     `networkPolicy: { name: test }`,
			wantType: "networkPolicy",
		},
		{
			name:            "invalid yaml",
			yaml:            `this is not valid yaml:`,
			wantErrContains: "unrecognized resource type",
		},
		{
			name:            "unknown resource",
			yaml:            `unknown: { name: test }`,
			wantErrContains: "unrecognized resource type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceType, err := determineResourceType([]byte(tt.yaml))

			// Check if error matches expectation
			if tt.wantErrContains != "" {
				if err == nil {
					t.Errorf("Expected error containing %q but got none", tt.wantErrContains)
				} else if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Expected error containing %q, got %v", tt.wantErrContains, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check if resource type matches expectation
			if tt.wantType != "" && resourceType != tt.wantType {
				t.Errorf("Expected resource type %q, got %q", tt.wantType, resourceType)
			}
		})
	}
}
