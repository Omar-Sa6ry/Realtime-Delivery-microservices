package domain

// RetryPolicy defines the strategy interface for determining max retry attempts per job type.
type RetryPolicy interface {
	// JobType returns the job type this policy handles.
	JobType() JobType

	// MaxAttempts returns the maximum number of attempts for this job type.
	MaxAttempts() int
}

// RetryPolicyRegistry holds all registered retry policies.
type RetryPolicyRegistry struct {
	policies map[JobType]RetryPolicy
}

func NewRetryPolicyRegistry() *RetryPolicyRegistry {
	return &RetryPolicyRegistry{
		policies: make(map[JobType]RetryPolicy),
	}
}

func (r *RetryPolicyRegistry) Register(p RetryPolicy) {
	r.policies[p.JobType()] = p
}

func (r *RetryPolicyRegistry) Get(jobType JobType) (RetryPolicy, bool) {
	p, ok := r.policies[jobType]
	return p, ok
}

// TranscodeRetryPolicy handles retry policy for transcoding jobs (expensive, fewer retries).
type TranscodeRetryPolicy struct{}

func (p *TranscodeRetryPolicy) JobType() JobType {
	return JobTypeTranscode
}

func (p *TranscodeRetryPolicy) MaxAttempts() int {
	return 2 // video transcoding is expensive; limit retries
}

// ScanRetryPolicy handles retry policy for scan jobs (more retries allowed).
type ScanRetryPolicy struct{}

func (p *ScanRetryPolicy) JobType() JobType {
	return JobTypeScan
}

func (p *ScanRetryPolicy) MaxAttempts() int {
	return 5
}

// DefaultRetryPolicy handles retry policy for all other job types.
type DefaultRetryPolicy struct{}

func (p *DefaultRetryPolicy) JobType() JobType {
	return JobType("DEFAULT")
}

func (p *DefaultRetryPolicy) MaxAttempts() int {
	return 3
}

// BuildRetryPolicyRegistry creates and populates the retry policy registry.
func BuildRetryPolicyRegistry() *RetryPolicyRegistry {
	registry := NewRetryPolicyRegistry()
	registry.Register(&TranscodeRetryPolicy{})
	registry.Register(&ScanRetryPolicy{})
	registry.Register(&DefaultRetryPolicy{})
	return registry
}

// DefaultRetryPolicyRegistry is the default registry for retry policies.
var DefaultRetryPolicyRegistry = BuildRetryPolicyRegistry()

// DefaultMaxAttempts returns the default retry count per job type using the registry.
func DefaultMaxAttempts(jt JobType) int {
	if policy, ok := DefaultRetryPolicyRegistry.Get(jt); ok {
		return policy.MaxAttempts()
	}
	// Fallback to default policy
	if policy, ok := DefaultRetryPolicyRegistry.Get(JobType("DEFAULT")); ok {
		return policy.MaxAttempts()
	}
	return 3
}