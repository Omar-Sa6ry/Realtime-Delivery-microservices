package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

func TestAssignmentStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.AssignmentStatus
		expected bool
	}{
		{"None is valid", domain.AssignmentStatusNone, true},
		{"Offered is valid", domain.AssignmentStatusOffered, true},
		{"Accepted is valid", domain.AssignmentStatusAccepted, true},
		{"Active is valid", domain.AssignmentStatusActive, true},
		{"Completed is valid", domain.AssignmentStatusCompleted, true},
		{"Rejected is valid", domain.AssignmentStatusRejected, true},
		{"Expired is valid", domain.AssignmentStatusExpired, true},
		{"Cancelled is valid", domain.AssignmentStatusCancelled, true},
		{"Unknown is invalid", domain.AssignmentStatus("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsAssignmentStatusValid(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.AssignmentStatus
		expected bool
	}{
		{"Offered is valid", domain.AssignmentStatusOffered, true},
		{"Completed is valid", domain.AssignmentStatusCompleted, true},
		{"Unknown is invalid", domain.AssignmentStatus("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.IsAssignmentStatusValid(tt.status)
			if result != tt.expected {
				t.Errorf("IsAssignmentStatusValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidAssignmentTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.AssignmentStatus
		to       domain.AssignmentStatus
		action   string
		expected bool
	}{
		{"None to Offered via create", domain.AssignmentStatusNone, domain.AssignmentStatusOffered, "create", true},
		{"Offered to Accepted via accept", domain.AssignmentStatusOffered, domain.AssignmentStatusAccepted, "accept", true},
		{"Offered to Rejected via reject", domain.AssignmentStatusOffered, domain.AssignmentStatusRejected, "reject", true},
		{"Offered to Expired via expire", domain.AssignmentStatusOffered, domain.AssignmentStatusExpired, "expire", true},
		{"Accepted to Active via start", domain.AssignmentStatusAccepted, domain.AssignmentStatusActive, "start", true},
		{"Active to Completed via complete", domain.AssignmentStatusActive, domain.AssignmentStatusCompleted, "complete", true},
		{"Rejected to Cancelled via cancel", domain.AssignmentStatusRejected, domain.AssignmentStatusCancelled, "cancel", true},
		{"Expired to Cancelled via cancel", domain.AssignmentStatusExpired, domain.AssignmentStatusCancelled, "cancel", true},
		{"None to Accepted invalid", domain.AssignmentStatusNone, domain.AssignmentStatusAccepted, "accept", false},
		{"Offered to Completed invalid", domain.AssignmentStatusOffered, domain.AssignmentStatusCompleted, "complete", false},
		{"Cancelled to any invalid", domain.AssignmentStatusCancelled, domain.AssignmentStatusOffered, "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.IsValidAssignmentTransition(tt.from, tt.to, tt.action)
			if result != tt.expected {
				t.Errorf("IsValidAssignmentTransition(%v, %v, %q) = %v, want %v", tt.from, tt.to, tt.action, result, tt.expected)
			}
		})
	}
}

func TestAssignment_CreateAssignment(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
		checkFields   bool
	}{
		{
			name: "success from none",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusNone,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusOffered,
			checkFields:    true,
		},
		{
			name: "fails when already offered",
			assignment: &domain.Assignment{
				ID:     "assign-2",
				Status: domain.AssignmentStatusOffered,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusOffered,
			checkFields:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.CreateAssignment()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("CreateAssignment() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("CreateAssignment() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("CreateAssignment() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
			if tt.checkFields {
				if tt.assignment.AttemptNumber != 1 {
					t.Errorf("AttemptNumber = %v, want 1", tt.assignment.AttemptNumber)
				}
				if tt.assignment.ExpiresAt.IsZero() {
					t.Errorf("ExpiresAt should be set")
				}
				if !tt.assignment.ExpiresAt.After(time.Now()) {
					t.Errorf("ExpiresAt should be in future")
				}
			}
		})
	}
}

func TestAssignment_Accept(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
		checkFields   bool
	}{
		{
			name: "success from offered",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusOffered,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusAccepted,
			checkFields:    true,
		},
		{
			name: "fails when not offered",
			assignment: &domain.Assignment{
				ID:     "assign-2",
				Status: domain.AssignmentStatusAccepted,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusAccepted,
			checkFields:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.Accept()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Accept() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Accept() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Accept() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
			if tt.checkFields {
				if tt.assignment.AcceptedAt == nil {
					t.Errorf("AcceptedAt should be set")
				}
			}
		})
	}
}

func TestAssignment_Reject(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
		checkFields   bool
	}{
		{
			name: "success from offered",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusOffered,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusRejected,
			checkFields:    true,
		},
		{
			name: "fails when not offered",
			assignment: &domain.Assignment{
				ID:     "assign-2",
				Status: domain.AssignmentStatusAccepted,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusAccepted,
			checkFields:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.Reject()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Reject() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Reject() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Reject() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
			if tt.checkFields {
				if tt.assignment.RejectedAt == nil {
					t.Errorf("RejectedAt should be set")
				}
			}
		})
	}
}

func TestAssignment_Expire(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
	}{
		{
			name: "success from offered",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusOffered,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusExpired,
		},
		{
			name: "fails when not offered",
			assignment: &domain.Assignment{
				ID:     "assign-2",
				Status: domain.AssignmentStatusAccepted,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.Expire()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expire() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Expire() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Expire() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
		})
	}
}

func TestAssignment_Start(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
	}{
		{
			name: "success from accepted",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusAccepted,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusActive,
		},
		{
			name: "fails when not accepted",
			assignment: &domain.Assignment{
				ID:     "assign-2",
				Status: domain.AssignmentStatusOffered,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusOffered,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.Start()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Start() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Start() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Start() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
		})
	}
}

func TestAssignment_Complete(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
		checkFields   bool
	}{
		{
			name: "success from active",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusActive,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusCompleted,
			checkFields:    true,
		},
		{
			name: "fails when not active",
			assignment: &domain.Assignment{
				ID:     "assign-2",
				Status: domain.AssignmentStatusAccepted,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusAccepted,
			checkFields:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.Complete()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Complete() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Complete() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Complete() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
			if tt.checkFields {
				if tt.assignment.CompletedAt == nil {
					t.Errorf("CompletedAt should be set")
				}
			}
		})
	}
}

func TestAssignment_Cancel(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *domain.Assignment
		expectedError error
		expectedStatus domain.AssignmentStatus
	}{
		{
			name: "success from offered",
			assignment: &domain.Assignment{
				ID:     "assign-1",
				Status: domain.AssignmentStatusOffered,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusCancelled,
		},
		{
			name: "success from accepted",
			assignment: &domain.Assignment{
				ID:           "assign-2",
				Status:       domain.AssignmentStatusAccepted,
				AcceptedAt:   ptrTime(time.Now()),
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusCancelled,
		},
		{
			name: "success from active",
			assignment: &domain.Assignment{
				ID:     "assign-3",
				Status: domain.AssignmentStatusActive,
			},
			expectedError:   nil,
			expectedStatus: domain.AssignmentStatusCancelled,
		},
		{
			name: "fails from completed",
			assignment: &domain.Assignment{
				ID:     "assign-4",
				Status: domain.AssignmentStatusCompleted,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusCompleted,
		},
		{
			name: "fails from rejected",
			assignment: &domain.Assignment{
				ID:     "assign-5",
				Status: domain.AssignmentStatusRejected,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusRejected,
		},
		{
			name: "fails from expired",
			assignment: &domain.Assignment{
				ID:     "assign-6",
				Status: domain.AssignmentStatusExpired,
			},
			expectedError:   errors.New("invalid assignment state transition"),
			expectedStatus: domain.AssignmentStatusExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assignment.Cancel()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Cancel() expected error, got nil")
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Cancel() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Cancel() unexpected error: %v", err)
				}
			}
			if tt.assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status = %v, want %v", tt.assignment.Status, tt.expectedStatus)
			}
		})
	}
}

func TestAssignment_Duration(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Minute)

	tests := []struct {
		name     string
		assignment *domain.Assignment
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{
			name: "offered duration since offered",
			assignment: &domain.Assignment{
				Status:    domain.AssignmentStatusOffered,
				OfferedAt: past,
			},
			expectedMin: time.Since(past) - time.Second,
			expectedMax: time.Since(past) + time.Second,
		},
		{
			name: "accepted duration since accepted",
			assignment: &domain.Assignment{
				Status:     domain.AssignmentStatusAccepted,
				AcceptedAt: &past,
			},
			expectedMin: time.Since(past) - time.Second,
			expectedMax: time.Since(past) + time.Second,
		},
		{
			name: "active duration since offered",
			assignment: &domain.Assignment{
				Status:    domain.AssignmentStatusActive,
				OfferedAt: past,
			},
			expectedMin: time.Since(past) - time.Second,
			expectedMax: time.Since(past) + time.Second,
		},
		{
			name: "completed returns time since updated when no completed time",
			assignment: &domain.Assignment{
				Status:      domain.AssignmentStatusCompleted,
				CompletedAt: nil,
				UpdatedAt:   past,
			},
			expectedMin: time.Since(past) - time.Second,
			expectedMax: time.Since(past) + time.Second,
		},
		{
			name: "rejected returns 0",
			assignment: &domain.Assignment{
				Status: domain.AssignmentStatusRejected,
			},
			expectedMin: 0,
			expectedMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := tt.assignment.Duration()
			if duration < tt.expectedMin || duration > tt.expectedMax {
				t.Errorf("Duration() = %v, want between %v and %v", duration, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestAssignment_ValidateOfferExpires(t *testing.T) {
	tests := []struct {
		name     string
		expiresAt time.Time
		expected bool
	}{
		{"not expired", time.Now().Add(time.Minute), true},
		{"expired", time.Now().Add(-time.Minute), false},
		{"exact now", time.Now(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &domain.Assignment{
				ExpiresAt: tt.expiresAt,
			}
			result := a.ValidateOfferExpires()
			if result != tt.expected {
				t.Errorf("ValidateOfferExpires() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAssignment_String(t *testing.T) {
	a := &domain.Assignment{
		Status: domain.AssignmentStatusOffered,
	}
	if a.Status.String() != "OFFERED" {
		t.Errorf("String() = %v, want OFFERED", a.Status.String())
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}