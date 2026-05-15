package service

import "testing"

func TestHasPermission(t *testing.T) {
	tests := []struct {
		actual   string
		required string
		expected bool
	}{
		{"owner", "read", true},
		{"owner", "write", true},
		{"owner", "delete", true},
		{"delete", "read", true},
		{"delete", "write", true},
		{"write", "read", true},
		{"write", "delete", false},
		{"read", "write", false},
		{"read", "delete", false},
		{"read", "read", true},
	}
	for _, tt := range tests {
		result := hasPermission(tt.actual, tt.required)
		if result != tt.expected {
			t.Errorf("hasPermission(%s, %s) = %v, expected %v", tt.actual, tt.required, result, tt.expected)
		}
	}
}
