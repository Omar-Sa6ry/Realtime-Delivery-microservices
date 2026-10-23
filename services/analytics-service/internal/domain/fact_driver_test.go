package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignmentResult_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		result   AssignmentResult
		expected bool
	}{
		{"offered", AssignmentOffered, true},
		{"accepted", AssignmentAccepted, true},
		{"rejected", AssignmentRejected, true},
		{"expired", AssignmentExpired, true},
		{"released", AssignmentReleased, true},
		{"invalid", AssignmentResult("INVALID"), false},
		{"empty", AssignmentResult(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.result.IsValid())
		})
	}
}

func TestAssignmentResult_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		result   AssignmentResult
		expected bool
	}{
		{"offered", AssignmentOffered, false},
		{"accepted", AssignmentAccepted, true},
		{"rejected", AssignmentRejected, true},
		{"expired", AssignmentExpired, true},
		{"released", AssignmentReleased, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.result.IsTerminal())
		})
	}
}

func TestFactDriverAssignment_ComputeResponseTime(t *testing.T) {
	now := time.Now().UTC()
	offeredAt := now.Add(-10 * time.Minute)

	tests := []struct {
		name           string
		assignment     *FactDriverAssignment
		expectedMs     *uint32
		expectNil      bool
	}{
		{
			name: "accepted - response time computed",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				AcceptedAt:   func() *time.Time { t := offeredAt.Add(2 * time.Minute); return &t }(),
				Result:       AssignmentAccepted,
			},
			expectedMs: func() *uint32 { u := uint32(120000); return &u }(), // 2 min = 120000 ms
			expectNil:  false,
		},
		{
			name: "rejected - response time computed",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				RejectedAt:   func() *time.Time { t := offeredAt.Add(3 * time.Minute); return &t }(),
				Result:       AssignmentRejected,
			},
			expectedMs: func() *uint32 { u := uint32(180000); return &u }(),
			expectNil:  false,
		},
		{
			name: "expired - response time computed",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				ExpiredAt:    func() *time.Time { t := offeredAt.Add(5 * time.Minute); return &t }(),
				Result:       AssignmentExpired,
			},
			expectedMs: func() *uint32 { u := uint32(300000); return &u }(),
			expectNil:  false,
		},
		{
			name: "released - response time computed",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				ReleasedAt:   func() *time.Time { t := offeredAt.Add(1 * time.Minute); return &t }(),
				Result:       AssignmentReleased,
			},
			expectedMs: func() *uint32 { u := uint32(60000); return &u }(),
			expectNil:  false,
		},
		{
			name: "multiple timestamps - picks earliest valid",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				AcceptedAt:   func() *time.Time { t := offeredAt.Add(5 * time.Minute); return &t }(),
				RejectedAt:   func() *time.Time { t := offeredAt.Add(2 * time.Minute); return &t }(),
				Result:       AssignmentRejected,
			},
			expectedMs: func() *uint32 { u := uint32(120000); return &u }(), // picks rejected at 2 min
			expectNil:  false,
		},
		{
			name: "no response timestamps - nil response time",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				Result:       AssignmentOffered,
			},
			expectedMs: nil,
			expectNil:  true,
		},
		{
			name: "zero offered at - returns nil",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    time.Time{},
				AcceptedAt:   func() *time.Time { t := now; return &t }(),
				Result:       AssignmentAccepted,
			},
			expectedMs: nil,
			expectNil:  true,
		},
		{
			name: "nil assignment - no panic",
			assignment: nil,
			expectedMs: nil,
			expectNil:  true,
		},
		{
			name: "response before offered - ignored",
			assignment: &FactDriverAssignment{
				AssignmentID: "assign-123",
				DriverID:     "driver-1",
				OfferedAt:    offeredAt,
				AcceptedAt:   func() *time.Time { t := offeredAt.Add(-1 * time.Minute); return &t }(),
				Result:       AssignmentAccepted,
			},
			expectedMs: nil,
			expectNil:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.assignment != nil {
				tc.assignment.ComputeResponseTime()
				if tc.expectNil {
					assert.Nil(t, tc.assignment.ResponseTimeMs)
				} else {
					require.NotNil(t, tc.assignment.ResponseTimeMs)
					assert.Equal(t, *tc.expectedMs, *tc.assignment.ResponseTimeMs)
				}
			} else {
				// nil assignment should not panic
				tc.assignment.ComputeResponseTime()
			}
		})
	}
}

func TestFactDriverAssignment_Validate(t *testing.T) {
	now := time.Now().UTC()
	base := FactDriverAssignment{
		AssignmentID: "assign-123",
		DeliveryID:   "delivery-456",
		DriverID:     "driver-789",
		OfferedAt:    now.Add(-10 * time.Minute),
		Result:       AssignmentAccepted,
		IngestedAt:   now,
	}

	tests := []struct {
		name        string
		assignment  *FactDriverAssignment
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid assignment",
			assignment:  &base,
			expectError: false,
		},
		{
			name: "nil assignment",
			assignment:  nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty assignment ID",
			assignment:  func() *FactDriverAssignment { a := base; a.AssignmentID = ""; return &a }(),
			expectError: true,
			errorMsg:    "assignmentId is required",
		},
		{
			name: "empty driver ID",
			assignment:  func() *FactDriverAssignment { a := base; a.DriverID = ""; return &a }(),
			expectError: true,
			errorMsg:    "driverId is required",
		},
		{
			name: "invalid result",
			assignment:  func() *FactDriverAssignment { a := base; a.Result = "INVALID"; return &a }(),
			expectError: true,
			errorMsg:    "unknown event type",
		},
		{
			name: "zero offered at",
			assignment:  func() *FactDriverAssignment { a := base; a.OfferedAt = time.Time{}; return &a }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
		{
			name: "accepted before offered",
			assignment:  func() *FactDriverAssignment { a := base; a.AcceptedAt = func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(); return &a }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "rejected before offered",
			assignment:  func() *FactDriverAssignment { a := base; a.RejectedAt = func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(); return &a }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "expired before offered",
			assignment:  func() *FactDriverAssignment { a := base; a.ExpiredAt = func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(); return &a }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "released before offered",
			assignment:  func() *FactDriverAssignment { a := base; a.ReleasedAt = func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(); return &a }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.assignment.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}