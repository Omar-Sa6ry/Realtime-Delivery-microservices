package domain

import (
	"sort"
	"time"
)

// DispatchPolicy defines the rules and logic for dispatching deliveries to drivers.
type DispatchPolicy struct {
	maxAttempts        int
	searchRadiusKm     float64
	assignmentTimeout  time.Duration
	candidateCount     int
}

// NewDispatchPolicy creates a default dispatch policy.
func NewDispatchPolicy() *DispatchPolicy {
	return &DispatchPolicy{
		maxAttempts:       5,
		searchRadiusKm:    5,
		assignmentTimeout: 20 * time.Second,
		candidateCount:    10,
	}
}

// SetMaxAttempts sets the maximum dispatch attempts per delivery.
func (p *DispatchPolicy) SetMaxAttempts(n int) {
	p.maxAttempts = n
}

// SetSearchRadiusKm sets the search radius in kilometers.
func (p *DispatchPolicy) SetSearchRadiusKm(radiusKm float64) {
	p.searchRadiusKm = radiusKm
}

// SetAssignmentTimeout sets the assignment offer timeout duration.
func (p *DispatchPolicy) SetAssignmentTimeout(timeout time.Duration) {
	p.assignmentTimeout = timeout
}

// SetCandidateCount sets the number of candidates to consider.
func (p *DispatchPolicy) SetCandidateCount(count int) {
	p.candidateCount = count
}

// FindAvailableDriversPolicy defines the criteria for finding available drivers.
type FindAvailableDriversPolicy struct {
	PickupLatitude  float64
	PickupLongitude float64
	RadiusKm        float64
	VehicleType     string
	DeliveryID      string
	ExcludeDriverIDs []string
}

// Validate checks the policy criteria are valid.
func (f *FindAvailableDriversPolicy) Validate() error {
	if f.RadiusKm <= 0 {
		return ErrInvalidArgument
	}
	if f.PickupLatitude < -90 || f.PickupLatitude > 90 {
		return ErrInvalidArgument
	}
	if f.PickupLongitude < -180 || f.PickupLongitude > 180 {
		return ErrInvalidArgument
	}
	return nil
}

// Candidate represents a driver candidate for dispatch with ranking score.
type Candidate struct {
	DriverID       string
	DistanceMeters float64
	VehicleType    string
	Status         DriverStatus
	RankingScore   float64
}

// ByRankingScore sorts candidates by ranking score ascending (closest/best first).
type ByRankingScore []Candidate

func (c ByRankingScore) Len() int           { return len(c) }
func (c ByRankingScore) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ByRankingScore) Less(i, j int) bool { return c[i].RankingScore < c[j].RankingScore }

// RankCandidates ranks drivers by distance and compatibility.
func (p *DispatchPolicy) RankCandidates(candidates []Candidate) []Candidate {
	ranked := make([]Candidate, len(candidates))
	copy(ranked, candidates)

	sort.Sort(ByRankingScore(ranked))

	// Apply simple ranking: nearest compatible driver first
	for i := range ranked {
		// Future: add availability, vehicle compatibility, acceptance rate, etc.
		if ranked[i].RankingScore == 0 {
			ranked[i].RankingScore = ranked[i].DistanceMeters
		}
	}

	return ranked[:min(p.candidateCount, len(ranked))]
}

// AttemptReservation attempts to atomically reserve a driver for a delivery.
// It acquires a distributed lock, checks driver state, and conditionally updates MongoDB.
func (p *DispatchPolicy) AttemptReservation(driverID string, deliveryID string) (bool, error) {
	// Validate attempt count
	// In a real implementation, this would:
	// 1. Acquire Redis lock: lock:driver:{driverID}
	// 2. Read current driver state from MongoDB
	// 3. Conditionally update: status = BUSY WHERE status = AVAILABLE
	// 4. Create assignment record as OFFERED
	// 5. Release lock
	// 6. Return success/failure

	// For now, validate basic criteria
	if driverID == "" {
		return false, ErrInvalidArgument
	}

	if deliveryID == "" {
		return false, ErrInvalidArgument
	}

	// Simulate: check if driver is available (would be MongoDB check in real impl)
	// This is a placeholder for the atomic reservation logic
	return true, nil
}

// DispatchAttempt records why a candidate was selected or rejected.
type DispatchAttempt struct {
	AttemptNumber int
	DriverID      string
	DistanceMeters float64
	Result        DispatchResult
	Reason        string
	CreatedAt     time.Time
}

// DispatchResult represents the outcome of a dispatch attempt.
type DispatchResult string

const (
	DispatchResultAccepted   DispatchResult = "ACCEPTED"
	DispatchResultRejected   DispatchResult = "REJECTED"
	DispatchResultTimeout    DispatchResult = "TIMEOUT"
	DispatchResultNoDriver   DispatchResult = "NO_DRIVER"
)

// RecordAttempt records a dispatch attempt for operational debugging.
func (p *DispatchPolicy) RecordAttempt(attempt DispatchAttempt) {
	// In a real implementation, this would persist to MongoDB dispatch_attempts collection
	// _id, deliveryId, driverId, distanceMeters, attemptNumber, result, reason, createdAt
	_ = attempt
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ValidateDispatchPolicy validates the dispatch policy configuration.
func ValidateDispatchPolicy(p *DispatchPolicy) error {
	if p == nil {
		return ErrInvalidArgument
	}
	if p.maxAttempts <= 0 {
		return ErrInvalidArgument
	}
	if p.searchRadiusKm <= 0 {
		return ErrInvalidArgument
	}
	if p.assignmentTimeout <= 0 {
		return ErrInvalidArgument
	}
	return nil
}

// Ensure DispatchPolicy validates correctly.