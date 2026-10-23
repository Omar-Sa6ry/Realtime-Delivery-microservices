package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactDeliveryEvent_Validate(t *testing.T) {
	now := time.Now().UTC()
	base := FactDeliveryEvent{
		EventID:      "evt-123",
		DeliveryID:   "delivery-456",
		UserID:       "user-123",
		DriverID:     "driver-789",
		EventType:    DeliveryCreated,
		EventVersion: 1,
		CityID:       "city-1",
		ZoneID:       "zone-1",
		OccurredAt:   now,
		IngestedAt:   now,
		CorrelationID: "corr-123",
	}

	tests := []struct {
		name        string
		event       *FactDeliveryEvent
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid event",
			event:       &base,
			expectError: false,
		},
		{
			name: "nil event",
			event:       nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty event ID",
			event:       func() *FactDeliveryEvent { e := base; e.EventID = ""; return &e }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "empty delivery ID",
			event:       func() *FactDeliveryEvent { e := base; e.DeliveryID = ""; return &e }(),
			expectError: true,
			errorMsg:    "deliveryId is required",
		},
		{
			name: "unknown event type",
			event:       func() *FactDeliveryEvent { e := base; e.EventType = "unknown.type"; return &e }(),
			expectError: true,
			errorMsg:    "unknown event type",
		},
		{
			name: "zero occurred at",
			event:       func() *FactDeliveryEvent { e := base; e.OccurredAt = time.Time{}; return &e }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.event.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFactDeliveryCompleted_ComputeDurations(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		completed   *FactDeliveryCompleted
		expectError bool
		checkDurations func(*testing.T, *FactDeliveryCompleted)
	}{
		{
			name: "complete timeline with all timestamps",
			completed: &FactDeliveryCompleted{
				DeliveryID:     "delivery-123",
				UserID:         "user-123",
				DriverID:       "driver-456",
				CreatedAt:      now.Add(-30 * time.Minute),
				AssignedAt:     func() *time.Time { t := now.Add(-25 * time.Minute); return &t }(),
				AcceptedAt:     func() *time.Time { t := now.Add(-24 * time.Minute); return &t }(),
				PickupStartedAt: func() *time.Time { t := now.Add(-20 * time.Minute); return &t }(),
				PickedUpAt:     func() *time.Time { t := now.Add(-18 * time.Minute); return &t }(),
				InTransitAt:    func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(),
				DeliveredAt:    func() *time.Time { t := now.Add(-5 * time.Minute); return &t }(),
				CompletedAt:    func() *time.Time { t := now; return &t }(),
				CityID:         "city-1",
				IngestedAt:     now,
			},
			expectError: false,
			checkDurations: func(t *testing.T, c *FactDeliveryCompleted) {
				assert.NotNil(t, c.TotalDurationS)
				assert.True(t, *c.TotalDurationS >= 1800) // ~30 min
				assert.NotNil(t, c.AssignmentDurationS)
				assert.True(t, *c.AssignmentDurationS >= 300) // ~5 min
				assert.NotNil(t, c.PickupDurationS)
				assert.True(t, *c.PickupDurationS >= 120) // ~2 min
				assert.NotNil(t, c.TransitDurationS)
				assert.True(t, *c.TransitDurationS >= 600) // ~10 min
			},
		},
		{
			name: "only created and completed",
			completed: &FactDeliveryCompleted{
				DeliveryID:  "delivery-123",
				UserID:      "user-123",
				DriverID:    "driver-456",
				CreatedAt:   now.Add(-30 * time.Minute),
				CompletedAt: func() *time.Time { t := now; return &t }(),
				CityID:      "city-1",
				IngestedAt:  now,
			},
			expectError: false,
			checkDurations: func(t *testing.T, c *FactDeliveryCompleted) {
				assert.NotNil(t, c.TotalDurationS)
				assert.Nil(t, c.AssignmentDurationS)
				assert.Nil(t, c.PickupDurationS)
				assert.Nil(t, c.TransitDurationS)
			},
		},
		{
			name: "negative total duration (completed before created)",
			completed: &FactDeliveryCompleted{
				DeliveryID:  "delivery-123",
				UserID:      "user-123",
				DriverID:    "driver-456",
				CreatedAt:   now,
				CompletedAt: func() *time.Time { t := now.Add(-10 * time.Minute); return &t }(),
				CityID:      "city-1",
				IngestedAt:  now,
			},
			expectError: true,
		},
		{
			name: "negative assignment duration",
			completed: &FactDeliveryCompleted{
				DeliveryID:  "delivery-123",
				UserID:      "user-123",
				DriverID:    "driver-456",
				CreatedAt:   now,
				AssignedAt:  func() *time.Time { t := now.Add(-10 * time.Minute); return &t }(),
				CompletedAt: func() *time.Time { t := now.Add(30 * time.Minute); return &t }(),
				CityID:      "city-1",
				IngestedAt:  now,
			},
			expectError: true,
		},
		{
			name: "negative pickup duration",
			completed: &FactDeliveryCompleted{
				DeliveryID:       "delivery-123",
				UserID:           "user-123",
				DriverID:         "driver-456",
				CreatedAt:        now.Add(-30 * time.Minute),
				PickupStartedAt:  func() *time.Time { t := now.Add(-10 * time.Minute); return &t }(),
				PickedUpAt:       func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(),
				CompletedAt:      func() *time.Time { t := now; return &t }(),
				CityID:           "city-1",
				IngestedAt:       now,
			},
			expectError: true,
		},
		{
			name: "nil pointers handled gracefully",
			completed: &FactDeliveryCompleted{
				DeliveryID:     "delivery-123",
				UserID:         "user-123",
				DriverID:       "driver-456",
				CreatedAt:      now.Add(-30 * time.Minute),
				AssignedAt:     nil,
				AcceptedAt:     func() *time.Time { t := now.Add(-24 * time.Minute); return &t }(),
				PickupStartedAt: nil,
				PickedUpAt:     nil,
				InTransitAt:    nil,
				DeliveredAt:    nil,
				CompletedAt:    func() *time.Time { t := now; return &t }(),
				CityID:         "city-1",
				IngestedAt:     now,
			},
			expectError: false,
			checkDurations: func(t *testing.T, c *FactDeliveryCompleted) {
				assert.NotNil(t, c.TotalDurationS)
				assert.Nil(t, c.AssignmentDurationS)
				assert.Nil(t, c.PickupDurationS)
				assert.Nil(t, c.TransitDurationS)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.completed.ComputeDurations()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "negative duration")
			} else {
				assert.NoError(t, err)
				if tc.checkDurations != nil {
					tc.checkDurations(t, tc.completed)
				}
			}
		})
	}
}

func TestFactDeliveryCompleted_Validate(t *testing.T) {
	now := time.Now().UTC()
	base := FactDeliveryCompleted{
		DeliveryID: "delivery-123",
		UserID:     "user-123",
		DriverID:   "driver-456",
		CreatedAt:  now.Add(-30 * time.Minute),
		CityID:     "city-1",
		IngestedAt: now,
	}

	tests := []struct {
		name        string
		completed   *FactDeliveryCompleted
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid completed",
			completed:   &base,
			expectError: false,
		},
		{
			name: "nil completed",
			completed:   nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty delivery ID",
			completed:   func() *FactDeliveryCompleted { c := base; c.DeliveryID = ""; return &c }(),
			expectError: true,
			errorMsg:    "deliveryId is required",
		},
		{
			name: "zero created at",
			completed:   func() *FactDeliveryCompleted { c := base; c.CreatedAt = time.Time{}; return &c }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
		{
			name: "assigned before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.AssignedAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "accepted before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.AcceptedAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "pickup started before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.PickupStartedAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "picked up before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.PickedUpAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "in transit before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.InTransitAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "delivered before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.DeliveredAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
		{
			name: "completed before created",
			completed:   func() *FactDeliveryCompleted { c := base; c.CompletedAt = func() *time.Time { t := now.Add(-40 * time.Minute); return &t }(); return &c }(),
			expectError: true,
			errorMsg:    "timestamps out of order",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.completed.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeliveryEventTypes_Constants(t *testing.T) {
	expected := []string{
		"delivery.created",
		"delivery.driver.assigned",
		"delivery.driver.accepted",
		"delivery.pickup.started",
		"delivery.picked_up",
		"delivery.in_transit",
		"delivery.completed",
		"delivery.cancelled",
		"delivery.failed",
		"delivery.deleted",
	}

	for _, e := range expected {
		assert.True(t, KnownDeliveryEventTypes[e], "event type %s should be known", e)
	}

	assert.False(t, KnownDeliveryEventTypes["unknown.event"])
}