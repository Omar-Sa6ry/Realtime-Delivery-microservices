package domain

import "testing"

func TestRetryPolicyRegistry_RegisterAndGet(t *testing.T) {
	r := NewRetryPolicyRegistry()
	policy := &ScanRetryPolicy{}
	r.Register(policy)

	got, ok := r.Get(JobTypeScan)
	if !ok {
		t.Fatal("expected registered policy to be found")
	}
	if got != policy {
		t.Errorf("Get() = %v, want the registered policy", got)
	}

	if _, ok := r.Get(JobTypeTranscode); ok {
		t.Error("unregistered job type must not be found")
	}
}

func TestRetryPolicies(t *testing.T) {
	cases := []struct {
		name         string
		policy       RetryPolicy
		wantJobType  JobType
		wantAttempts int
	}{
		{"transcode", &TranscodeRetryPolicy{}, JobTypeTranscode, 2},
		{"scan", &ScanRetryPolicy{}, JobTypeScan, 5},
		{"default", &DefaultRetryPolicy{}, JobType("DEFAULT"), 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.JobType(); got != tc.wantJobType {
				t.Errorf("JobType() = %q, want %q", got, tc.wantJobType)
			}
			if got := tc.policy.MaxAttempts(); got != tc.wantAttempts {
				t.Errorf("MaxAttempts() = %d, want %d", got, tc.wantAttempts)
			}
		})
	}
}

func TestBuildRetryPolicyRegistry(t *testing.T) {
	r := BuildRetryPolicyRegistry()
	cases := []struct {
		jobType     JobType
		wantAttempt int
	}{
		{JobTypeTranscode, 2},
		{JobTypeScan, 5},
		{JobType("DEFAULT"), 3},
	}
	for _, tc := range cases {
		p, ok := r.Get(tc.jobType)
		if !ok {
			t.Fatalf("expected policy for %s", tc.jobType)
		}
		if p.MaxAttempts() != tc.wantAttempt {
			t.Errorf("MaxAttempts() for %s = %d, want %d", tc.jobType, p.MaxAttempts(), tc.wantAttempt)
		}
	}
}

func TestDefaultMaxAttempts(t *testing.T) {
	cases := []struct {
		name    string
		jobType JobType
		want    int
	}{
		{"transcode_registered", JobTypeTranscode, 2},
		{"scan_registered", JobTypeScan, 5},
		{"unregistered_falls_back_to_default", JobTypeCompress, 3},
		{"unknown_type_falls_back_to_default", JobType("NOPE"), 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DefaultMaxAttempts(tc.jobType); got != tc.want {
				t.Errorf("DefaultMaxAttempts(%s) = %d, want %d", tc.jobType, got, tc.want)
			}
		})
	}
}

func TestDefaultMaxAttempts_WhenDefaultPolicyMissing(t *testing.T) {
	orig := DefaultRetryPolicyRegistry
	DefaultRetryPolicyRegistry = NewRetryPolicyRegistry()
	defer func() { DefaultRetryPolicyRegistry = orig }()

	if got := DefaultMaxAttempts(JobTypeScan); got != 3 {
		t.Errorf("DefaultMaxAttempts() = %d, want hard-coded fallback 3", got)
	}
}
