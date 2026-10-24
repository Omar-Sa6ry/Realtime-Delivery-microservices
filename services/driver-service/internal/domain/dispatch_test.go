package domain_test

import (
	"sort"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

func TestDispatchPolicy_NewDispatchPolicy(t *testing.T) {
	policy := domain.NewDispatchPolicy()

	if policy == nil {
		t.Fatal("NewDispatchPolicy() returned nil")
	}
	if policy == nil {
		return
	}
}

func TestDispatchPolicy_Setters(t *testing.T) {
	policy := domain.NewDispatchPolicy()

	policy.SetMaxAttempts(10)
	policy.SetSearchRadiusKm(10.5)
	policy.SetAssignmentTimeout(60 * time.Second)
	policy.SetCandidateCount(20)
}

func TestDispatchPolicy_ValidateDispatchPolicy(t *testing.T) {
	createPolicy := func(maxAttempts int, radiusKm float64, timeout time.Duration, count int) *domain.DispatchPolicy {
		p := domain.NewDispatchPolicy()
		p.SetMaxAttempts(maxAttempts)
		p.SetSearchRadiusKm(radiusKm)
		p.SetAssignmentTimeout(timeout)
		p.SetCandidateCount(count)
		return p
	}

	tests := []struct {
		name     string
		policy   *domain.DispatchPolicy
		expected error
	}{
		{
			name: "valid policy",
			policy: createPolicy(5, 5.0, 30*time.Second, 10),
			expected: nil,
		},
		{
			name:     "nil policy",
			policy:   nil,
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "zero max attempts",
			policy: createPolicy(0, 5.0, 30*time.Second, 10),
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "negative max attempts",
			policy: createPolicy(-1, 5.0, 30*time.Second, 10),
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "zero search radius",
			policy: createPolicy(5, 0, 30*time.Second, 10),
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "negative search radius",
			policy: createPolicy(5, -1.0, 30*time.Second, 10),
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "zero assignment timeout",
			policy: createPolicy(5, 5.0, 0, 10),
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "negative assignment timeout",
			policy: createPolicy(5, 5.0, -time.Second, 10),
			expected: domain.ErrInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateDispatchPolicy(tt.policy)
			if tt.expected != nil {
				if err == nil {
					t.Errorf("ValidateDispatchPolicy() expected error %v, got nil", tt.expected)
				} else if err.Error() != tt.expected.Error() {
					t.Errorf("ValidateDispatchPolicy() error = %v, want %v", err, tt.expected)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDispatchPolicy() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDispatchPolicy_RankCandidates(t *testing.T) {
	policy := domain.NewDispatchPolicy()
	policy.SetCandidateCount(5)

	candidates := []domain.Candidate{
		{DriverID: "d1", DistanceMeters: 1000, RankingScore: 1000},
		{DriverID: "d2", DistanceMeters: 500, RankingScore: 500},
		{DriverID: "d3", DistanceMeters: 2000, RankingScore: 2000},
		{DriverID: "d4", DistanceMeters: 1500, RankingScore: 1500},
		{DriverID: "d5", DistanceMeters: 300, RankingScore: 300},
		{DriverID: "d6", DistanceMeters: 800, RankingScore: 800},
	}

	ranked := policy.RankCandidates(candidates)

	if len(ranked) != 5 {
		t.Errorf("RankCandidates returned %d candidates, want 5", len(ranked))
	}

	// Should be sorted by ranking score (ascending)
	for i := 1; i < len(ranked); i++ {
		if ranked[i].RankingScore < ranked[i-1].RankingScore {
			t.Errorf("Candidates not sorted: %v then %v", ranked[i-1], ranked[i])
		}
	}

	// First should be d5 (300)
	if ranked[0].DriverID != "d5" {
		t.Errorf("First candidate = %v, want d5", ranked[0].DriverID)
	}
}

func TestDispatchPolicy_RankCandidates_ZeroRankingScore(t *testing.T) {
	policy := domain.NewDispatchPolicy()
	policy.SetCandidateCount(10)

	candidates := []domain.Candidate{
		{DriverID: "d1", DistanceMeters: 1000, RankingScore: 0},
		{DriverID: "d2", DistanceMeters: 500, RankingScore: 0},
	}

	ranked := policy.RankCandidates(candidates)

	// Should use DistanceMeters as fallback
	if len(ranked) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(ranked))
	}
	if ranked[0].DriverID != "d2" {
		t.Errorf("First should be d2 (500m), got %v", ranked[0].DriverID)
	}
	if ranked[1].DriverID != "d1" {
		t.Errorf("Second should be d1 (1000m), got %v", ranked[1].DriverID)
	}
}

func TestDispatchPolicy_RankCandidates_Empty(t *testing.T) {
	policy := domain.NewDispatchPolicy()
	ranked := policy.RankCandidates([]domain.Candidate{})
	if len(ranked) != 0 {
		t.Errorf("Expected 0 candidates, got %d", len(ranked))
	}
}

func TestDispatchPolicy_RankCandidates_LessThanCandidateCount(t *testing.T) {
	policy := domain.NewDispatchPolicy()
	policy.SetCandidateCount(10)

	candidates := []domain.Candidate{
		{DriverID: "d1", DistanceMeters: 1000},
	}

	ranked := policy.RankCandidates(candidates)

	if len(ranked) != 1 {
		t.Errorf("Expected 1 candidate, got %d", len(ranked))
	}
}

func TestDispatchPolicy_AttemptReservation(t *testing.T) {
	policy := domain.NewDispatchPolicy()
	policy.SetMaxAttempts(3)

	tests := []struct {
		name          string
		driverID      string
		deliveryID    string
		currentAttempts int
		expectedBool  bool
		expectedError error
	}{
		{
			name:             "success first attempt",
			driverID:         "driver-1",
			deliveryID:       "delivery-1",
			currentAttempts:  0,
			expectedBool:     true,
			expectedError:    nil,
		},
		{
			name:             "success within max attempts",
			driverID:         "driver-1",
			deliveryID:       "delivery-1",
			currentAttempts:  2,
			expectedBool:     true,
			expectedError:    nil,
		},
		{
			name:             "fails at max attempts",
			driverID:         "driver-1",
			deliveryID:       "delivery-1",
			currentAttempts:  3,
			expectedBool:     false,
			expectedError:    domain.ErrMaxAttemptsReached,
		},
		{
			name:             "fails with empty driverID",
			driverID:         "",
			deliveryID:       "delivery-1",
			currentAttempts:  0,
			expectedBool:     false,
			expectedError:    domain.ErrInvalidArgument,
		},
		{
			name:             "fails with empty deliveryID",
			driverID:         "driver-1",
			deliveryID:       "",
			currentAttempts:  0,
			expectedBool:     false,
			expectedError:    domain.ErrInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := policy.AttemptReservation(tt.driverID, tt.deliveryID, tt.currentAttempts)
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("AttemptReservation() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("AttemptReservation() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("AttemptReservation() unexpected error: %v", err)
				}
			}
			if result != tt.expectedBool {
				t.Errorf("AttemptReservation() = %v, want %v", result, tt.expectedBool)
			}
		})
	}
}

func TestFindAvailableDriversPolicy_Validate(t *testing.T) {
	tests := []struct {
		name     string
		policy   *domain.FindAvailableDriversPolicy
		expected error
	}{
		{
			name: "valid policy",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  37.7749,
				PickupLongitude: -122.4194,
				RadiusKm:        5.0,
			},
			expected: nil,
		},
		{
			name: "zero radius",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  37.7749,
				PickupLongitude: -122.4194,
				RadiusKm:        0,
			},
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "negative radius",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  37.7749,
				PickupLongitude: -122.4194,
				RadiusKm:        -1.0,
			},
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "latitude too high",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  91,
				PickupLongitude: -122.4194,
				RadiusKm:        5.0,
			},
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "latitude too low",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  -91,
				PickupLongitude: -122.4194,
				RadiusKm:        5.0,
			},
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "longitude too high",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  37.7749,
				PickupLongitude: 181,
				RadiusKm:        5.0,
			},
			expected: domain.ErrInvalidArgument,
		},
		{
			name: "longitude too low",
			policy: &domain.FindAvailableDriversPolicy{
				PickupLatitude:  37.7749,
				PickupLongitude: -181,
				RadiusKm:        5.0,
			},
			expected: domain.ErrInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.expected != nil {
				if err == nil {
					t.Errorf("Validate() expected error %v, got nil", tt.expected)
				} else if err.Error() != tt.expected.Error() {
					t.Errorf("Validate() error = %v, want %v", err, tt.expected)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCandidate_Sorting(t *testing.T) {
	candidates := []domain.Candidate{
		{DriverID: "d1", RankingScore: 3.0},
		{DriverID: "d2", RankingScore: 1.0},
		{DriverID: "d3", RankingScore: 2.0},
	}

	sort.Sort(domain.ByRankingScore(candidates))

	if candidates[0].DriverID != "d2" {
		t.Errorf("First = %v, want d2", candidates[0].DriverID)
	}
	if candidates[1].DriverID != "d3" {
		t.Errorf("Second = %v, want d3", candidates[1].DriverID)
	}
	if candidates[2].DriverID != "d1" {
		t.Errorf("Third = %v, want d1", candidates[2].DriverID)
	}
}