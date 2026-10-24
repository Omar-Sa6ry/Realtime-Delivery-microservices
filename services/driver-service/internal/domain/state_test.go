package domain_test

import (
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

func TestValidateDriverTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.DriverStatus
		to       domain.DriverStatus
		action   string
		expected error
	}{
		{
			name:     "valid offline to available",
			from:     domain.DriverStatusOffline,
			to:       domain.DriverStatusAvailable,
			action:   "GoOnline",
			expected: nil,
		},
		{
			name:     "valid available to offline",
			from:     domain.DriverStatusAvailable,
			to:       domain.DriverStatusOffline,
			action:   "GoOffline",
			expected: nil,
		},
		{
			name:     "valid available to busy",
			from:     domain.DriverStatusAvailable,
			to:       domain.DriverStatusBusy,
			action:   "Reserve",
			expected: nil,
		},
		{
			name:     "valid busy to available",
			from:     domain.DriverStatusBusy,
			to:       domain.DriverStatusAvailable,
			action:   "CompleteDelivery",
			expected: nil,
		},
		{
			name:     "invalid offline to busy",
			from:     domain.DriverStatusOffline,
			to:       domain.DriverStatusBusy,
			action:   "Reserve",
			expected: &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
		{
			name:     "invalid available to available",
			from:     domain.DriverStatusAvailable,
			to:       domain.DriverStatusAvailable,
			action:   "GoOnline",
			expected: &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateDriverTransition(tt.from, tt.to, tt.action)
			if tt.expected != nil {
				if err == nil {
					t.Errorf("ValidateDriverTransition() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDriverTransition() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateAssignmentTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.AssignmentStatus
		to       domain.AssignmentStatus
		action   string
		expected error
	}{
		{
			name:     "valid none to offered",
			from:     domain.AssignmentStatusNone,
			to:       domain.AssignmentStatusOffered,
			action:   "create",
			expected: nil,
		},
		{
			name:     "valid offered to accepted",
			from:     domain.AssignmentStatusOffered,
			to:       domain.AssignmentStatusAccepted,
			action:   "accept",
			expected: nil,
		},
		{
			name:     "valid offered to rejected",
			from:     domain.AssignmentStatusOffered,
			to:       domain.AssignmentStatusRejected,
			action:   "reject",
			expected: nil,
		},
		{
			name:     "valid offered to expired",
			from:     domain.AssignmentStatusOffered,
			to:       domain.AssignmentStatusExpired,
			action:   "expire",
			expected: nil,
		},
		{
			name:     "valid accepted to active",
			from:     domain.AssignmentStatusAccepted,
			to:       domain.AssignmentStatusActive,
			action:   "start",
			expected: nil,
		},
		{
			name:     "valid active to completed",
			from:     domain.AssignmentStatusActive,
			to:       domain.AssignmentStatusCompleted,
			action:   "complete",
			expected: nil,
		},
		{
			name:     "valid rejected to cancelled",
			from:     domain.AssignmentStatusRejected,
			to:       domain.AssignmentStatusCancelled,
			action:   "cancel",
			expected: nil,
		},
		{
			name:     "valid expired to cancelled",
			from:     domain.AssignmentStatusExpired,
			to:       domain.AssignmentStatusCancelled,
			action:   "cancel",
			expected: nil,
		},
		{
			name:     "invalid none to accepted",
			from:     domain.AssignmentStatusNone,
			to:       domain.AssignmentStatusAccepted,
			action:   "accept",
			expected: &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
		{
			name:     "invalid cancelled to offered",
			from:     domain.AssignmentStatusCancelled,
			to:       domain.AssignmentStatusOffered,
			action:   "create",
			expected: &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateAssignmentTransition(tt.from, tt.to, tt.action)
			if tt.expected != nil {
				if err == nil {
					t.Errorf("ValidateAssignmentTransition() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateAssignmentTransition() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name         string
		aggregateType string
		from          interface{}
		to            interface{}
		action        string
		expected      error
	}{
		{
			name:         "valid driver transition",
			aggregateType: "driver",
			from:          domain.DriverStatusOffline,
			to:            domain.DriverStatusAvailable,
			action:        "GoOnline",
			expected:      nil,
		},
		{
			name:         "valid assignment transition",
			aggregateType: "assignment",
			from:          domain.AssignmentStatusOffered,
			to:            domain.AssignmentStatusAccepted,
			action:        "accept",
			expected:      nil,
		},
		{
			name:         "invalid driver transition",
			aggregateType: "driver",
			from:          domain.DriverStatusOffline,
			to:            domain.DriverStatusBusy,
			action:        "Reserve",
			expected:      &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
		{
			name:         "invalid assignment transition",
			aggregateType: "assignment",
			from:          domain.AssignmentStatusNone,
			to:            domain.AssignmentStatusAccepted,
			action:        "accept",
			expected:      &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
		{
			name:         "unsupported aggregate type",
			aggregateType: "unknown",
			from:          domain.DriverStatusOffline,
			to:            domain.DriverStatusAvailable,
			action:        "GoOnline",
			expected:      &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
		{
			name:         "invalid from type for driver",
			aggregateType: "driver",
			from:          "invalid",
			to:            domain.DriverStatusAvailable,
			action:        "GoOnline",
			expected:      &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
		{
			name:         "invalid to type for driver",
			aggregateType: "driver",
			from:          domain.DriverStatusOffline,
			to:            "invalid",
			action:        "GoOnline",
			expected:      &domain.Error{Code: "validate_transition_error", Message: "transition validation failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateTransition(tt.aggregateType, tt.from, tt.to, tt.action)
			if tt.expected != nil {
				if err == nil {
					t.Errorf("ValidateTransition() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTransition() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsValidDriverStatusTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.DriverStatus
		to       domain.DriverStatus
		action   string
		expected bool
	}{
		{
			name:     "valid offline to available",
			from:     domain.DriverStatusOffline,
			to:       domain.DriverStatusAvailable,
			action:   "GoOnline",
			expected: true,
		},
		{
			name:     "valid available to offline",
			from:     domain.DriverStatusAvailable,
			to:       domain.DriverStatusOffline,
			action:   "GoOffline",
			expected: true,
		},
		{
			name:     "valid available to busy",
			from:     domain.DriverStatusAvailable,
			to:       domain.DriverStatusBusy,
			action:   "Reserve",
			expected: true,
		},
		{
			name:     "valid busy to available",
			from:     domain.DriverStatusBusy,
			to:       domain.DriverStatusAvailable,
			action:   "CompleteDelivery",
			expected: true,
		},
		{
			name:     "invalid offline to busy",
			from:     domain.DriverStatusOffline,
			to:       domain.DriverStatusBusy,
			action:   "Reserve",
			expected: false,
		},
		{
			name:     "invalid unknown from",
			from:     domain.DriverStatus("UNKNOWN"),
			to:       domain.DriverStatusAvailable,
			action:   "GoOnline",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.IsValidDriverStatusTransition(tt.from, tt.to, tt.action)
			if result != tt.expected {
				t.Errorf("IsValidDriverStatusTransition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidAssignmentStatusTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.AssignmentStatus
		to       domain.AssignmentStatus
		action   string
		expected bool
	}{
		{
			name:     "valid none to offered",
			from:     domain.AssignmentStatusNone,
			to:       domain.AssignmentStatusOffered,
			action:   "create",
			expected: true,
		},
		{
			name:     "valid offered to accepted",
			from:     domain.AssignmentStatusOffered,
			to:       domain.AssignmentStatusAccepted,
			action:   "accept",
			expected: true,
		},
		{
			name:     "valid offered to rejected",
			from:     domain.AssignmentStatusOffered,
			to:       domain.AssignmentStatusRejected,
			action:   "reject",
			expected: true,
		},
		{
			name:     "valid offered to expired",
			from:     domain.AssignmentStatusOffered,
			to:       domain.AssignmentStatusExpired,
			action:   "expire",
			expected: true,
		},
		{
			name:     "valid accepted to active",
			from:     domain.AssignmentStatusAccepted,
			to:       domain.AssignmentStatusActive,
			action:   "start",
			expected: true,
		},
		{
			name:     "valid active to completed",
			from:     domain.AssignmentStatusActive,
			to:       domain.AssignmentStatusCompleted,
			action:   "complete",
			expected: true,
		},
		{
			name:     "valid rejected to cancelled",
			from:     domain.AssignmentStatusRejected,
			to:       domain.AssignmentStatusCancelled,
			action:   "cancel",
			expected: true,
		},
		{
			name:     "valid expired to cancelled",
			from:     domain.AssignmentStatusExpired,
			to:       domain.AssignmentStatusCancelled,
			action:   "cancel",
			expected: true,
		},
		{
			name:     "invalid none to accepted",
			from:     domain.AssignmentStatusNone,
			to:       domain.AssignmentStatusAccepted,
			action:   "accept",
			expected: false,
		},
		{
			name:     "invalid cancelled to offered",
			from:     domain.AssignmentStatusCancelled,
			to:       domain.AssignmentStatusOffered,
			action:   "create",
			expected: false,
		},
		{
			name:     "invalid unknown from",
			from:     domain.AssignmentStatus("UNKNOWN"),
			to:       domain.AssignmentStatusOffered,
			action:   "create",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.IsValidAssignmentStatusTransition(tt.from, tt.to, tt.action)
			if result != tt.expected {
				t.Errorf("IsValidAssignmentStatusTransition() = %v, want %v", result, tt.expected)
			}
		})
	}
}