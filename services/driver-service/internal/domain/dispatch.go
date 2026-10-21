package domain

import (
	"sort"
	"time"
)

type DispatchPolicy struct {
	maxAttempts        int
	searchRadiusKm     float64
	assignmentTimeout  time.Duration
	candidateCount     int
}

func NewDispatchPolicy() *DispatchPolicy {
	return &DispatchPolicy{
		maxAttempts:       5,
		searchRadiusKm:    5,
		assignmentTimeout: 30 * time.Second,
		candidateCount:    10,
	}
}

func (p *DispatchPolicy) SetMaxAttempts(n int) {
	p.maxAttempts = n
}

func (p *DispatchPolicy) SetSearchRadiusKm(radiusKm float64) {
	p.searchRadiusKm = radiusKm
}

func (p *DispatchPolicy) SetAssignmentTimeout(timeout time.Duration) {
	p.assignmentTimeout = timeout
}

func (p *DispatchPolicy) SetCandidateCount(count int) {
	p.candidateCount = count
}

type FindAvailableDriversPolicy struct {
	PickupLatitude  float64
	PickupLongitude float64
	RadiusKm        float64
	VehicleType     VehicleType
	DeliveryID      string
	ExcludeDriverIDs []string
}

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

type Candidate struct {
	DriverID       string
	DistanceMeters float64
	VehicleType    VehicleType
	Status         DriverStatus
	RankingScore   float64
}

type ByRankingScore []Candidate

func (c ByRankingScore) Len() int           { return len(c) }
func (c ByRankingScore) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ByRankingScore) Less(i, j int) bool { return c[i].RankingScore < c[j].RankingScore }

func (p *DispatchPolicy) RankCandidates(candidates []Candidate) []Candidate {
	ranked := make([]Candidate, len(candidates))
	copy(ranked, candidates)

	for i := range ranked {
		if ranked[i].RankingScore == 0 {
			ranked[i].RankingScore = ranked[i].DistanceMeters
		}
	}

	sort.Sort(ByRankingScore(ranked))

	return ranked[:min(p.candidateCount, len(ranked))]
}

func (p *DispatchPolicy) AttemptReservation(driverID string, deliveryID string, currentAttempts int) (bool, error) {
	if driverID == "" || deliveryID == "" {
		return false, ErrInvalidArgument
	}

	if currentAttempts >= p.maxAttempts {
		return false, ErrMaxAttemptsReached
	}

	return true, nil
}

type DispatchAttempt struct {
	ID            string
	DeliveryID    string
	DriverID      string
	DistanceMeters float64
	AttemptNumber int
	Result        DispatchResult
	Reason        string
	CreatedAt     time.Time
}

type DispatchResult string

const (
	DispatchResultAccepted   DispatchResult = "ACCEPTED"
	DispatchResultRejected   DispatchResult = "REJECTED"
	DispatchResultTimeout    DispatchResult = "TIMEOUT"
	DispatchResultNoDriver   DispatchResult = "NO_DRIVER"
)

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
