package domain

type RetryPolicy interface {
	JobType() JobType
	MaxAttempts() int
}

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

type TranscodeRetryPolicy struct{}

func (p *TranscodeRetryPolicy) JobType() JobType {
	return JobTypeTranscode
}

func (p *TranscodeRetryPolicy) MaxAttempts() int {
	return 2 // video transcoding is expensive; limit retries
}

type ScanRetryPolicy struct{}

func (p *ScanRetryPolicy) JobType() JobType {
	return JobTypeScan
}

func (p *ScanRetryPolicy) MaxAttempts() int {
	return 5
}

type DefaultRetryPolicy struct{}

func (p *DefaultRetryPolicy) JobType() JobType {
	return JobType("DEFAULT")
}

func (p *DefaultRetryPolicy) MaxAttempts() int {
	return 3
}

func BuildRetryPolicyRegistry() *RetryPolicyRegistry {
	registry := NewRetryPolicyRegistry()
	registry.Register(&TranscodeRetryPolicy{})
	registry.Register(&ScanRetryPolicy{})
	registry.Register(&DefaultRetryPolicy{})
	return registry
}

var DefaultRetryPolicyRegistry = BuildRetryPolicyRegistry()

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