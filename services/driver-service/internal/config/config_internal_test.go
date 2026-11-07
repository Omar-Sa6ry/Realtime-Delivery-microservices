package config

import "testing"

// TestGetEnv exercises getEnv directly: non-empty values win, empty and
// missing values fall back to the provided default.
func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue string
		expected     string
	}{
		{"set value", "DRIVER_TEST_GETENV_SET", "custom", "fallback", "custom"},
		{"empty value falls back", "DRIVER_TEST_GETENV_EMPTY", "", "fallback", "fallback"},
		{"unset key falls back", "DRIVER_TEST_GETENV_UNSET_XYZ", "", "fallback", "fallback"},
		{"empty default", "DRIVER_TEST_GETENV_EMPTY_DEFAULT", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if got := getEnv(tt.key, tt.defaultValue); got != tt.expected {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.expected)
			}
		})
	}
}
