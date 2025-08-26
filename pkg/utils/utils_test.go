package utils

import "testing"

func TestPickFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{
			name:     "first non-empty value",
			values:   []string{"first", "second", "third"},
			expected: "first",
		},
		{
			name:     "empty first value",
			values:   []string{"", "second", "third"},
			expected: "second",
		},
		{
			name:     "all empty values",
			values:   []string{"", "", ""},
			expected: "",
		},
		{
			name:     "single non-empty value",
			values:   []string{"only"},
			expected: "only",
		},
		{
			name:     "no values",
			values:   []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PickFirstNonEmpty(tt.values...)
			if result != tt.expected {
				t.Errorf("PickFirstNonEmpty() = %v, want %v", result, tt.expected)
			}
		})
	}
}
func TestValidateDNS1123Name(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid name",
			input:   "valid-name-123",
			wantErr: false,
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too long name",
			input:   "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnop",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			input:   "-invalid",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			input:   "invalid-",
			wantErr: true,
		},
		{
			name:    "uppercase letters",
			input:   "Invalid",
			wantErr: true,
		},
		{
			name:    "special characters",
			input:   "invalid@name",
			wantErr: true,
		},
		{
			name:    "consecutive hyphens",
			input:   "invalid--name",
			wantErr: true,
		},
		{
			name:    "minimal valid name",
			input:   "n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNS1123Name(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDNS1123Name() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
