package domain

import "testing"

func TestMediaJob_IsRetryable(t *testing.T) {
	cases := []struct {
		name        string
		attempts    int
		maxAttempts int
		want        bool
	}{
		{"first_attempt", 0, 3, true},
		{"below_max", 2, 3, true},
		{"at_max", 3, 3, false},
		{"above_max", 5, 3, false},
		{"zero_max_attempts", 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := &MediaJob{Attempts: tc.attempts, MaxAttempts: tc.maxAttempts}
			if got := j.IsRetryable(); got != tc.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tc.want)
			}
		})
	}
}
