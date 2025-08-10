package cmd

import (
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
