package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeliveryHandler_Handles(t *testing.T) {
	handler := &DeliveryHandler{}

	tests := []struct {
		name     string
		eventType string
		expected bool
	}{
		{"delivery.created", "delivery.created", true},
		{"delivery.driver.assigned", "delivery.driver.assigned", true},
		{"delivery.driver.accepted", "delivery.driver.accepted", true},
		{"delivery.pickup.started", "delivery.pickup.started", true},
		{"delivery.picked_up", "delivery.picked_up", true},
		{"delivery.in_transit", "delivery.in_transit", true},
		{"delivery.completed", "delivery.completed", true},
		{"delivery.cancelled", "delivery.cancelled", true},
		{"delivery.failed", "delivery.failed", true},
		{"delivery.deleted", "delivery.deleted", true},
		{"driver.available", "driver.available", false},
		{"payment.created", "payment.created", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, handler.Handles(tc.eventType))
		})
	}
}

func TestDeliveryHandler_Handle_DeliveryCreated(t *testing.T) {
	handler := &DeliveryHandler{}
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCreatedPayload("delivery-456")
	p := unmarshalPayload(payload)

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, p)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.Raw, 1)
	assert.Len(t, batch.Seen, 1)
	assert.Len(t, batch.DeliveryEvents, 1)
	assert.Len(t, batch.DeliveryCompleted, 1)

	event := batch.DeliveryEvents[0]
	assert.Equal(t, "evt-123", event.EventID)
	assert.Equal(t, "delivery-456", event.DeliveryID)
	assert.Equal(t, "user-123", event.UserID)
	assert.Equal(t, "delivery.created", event.EventType)
	assert.Equal(t, "city-1", event.CityID)
	assert.Equal(t, "zone-1", event.ZoneID)

	completed := batch.DeliveryCompleted[0]
	assert.Equal(t, "delivery-456", completed.DeliveryID)
	assert.NotNil(t, completed.CreatedAt)
}

func TestDeliveryHandler_Handle_DeliveryDriverAssigned(t *testing.T) {
	handler := &DeliveryHandler{}
	env := testutil.NewTestEventEnvelope("delivery.driver.assigned", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryDriverAssignedPayload("delivery-456", "driver-789")
	p := unmarshalPayload(payload)

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, p)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.DeliveryEvents, 1)
	// DeliveryCompleted is not created without prior DeliveryCreated event (CreatedAt would be zero)
	assert.Len(t, batch.DeliveryCompleted, 0)
	assert.Len(t, batch.DataQuality, 1) // Data quality issue for out of order timestamps

	event := batch.DeliveryEvents[0]
	assert.Equal(t, "delivery.driver.assigned", event.EventType)
	assert.Equal(t, "driver-789", event.DriverID)

	// Verify data quality issue
	assert.Equal(t, domain.IssueOutOfOrderTimestamps, batch.DataQuality[0].IssueType)
}

func TestDeliveryHandler_Handle_DeliveryCompleted(t *testing.T) {
	handler := &DeliveryHandler{}
	env := testutil.NewTestEventEnvelope("delivery.completed", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCompletedPayload("delivery-456")
	p := unmarshalPayload(payload)

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, p)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.DeliveryEvents, 1)
	// DeliveryCompleted is not created without prior DeliveryCreated event (CreatedAt would be zero)
	assert.Len(t, batch.DeliveryCompleted, 0)
	assert.Len(t, batch.DataQuality, 1) // Data quality issue for out of order timestamps

	event := batch.DeliveryEvents[0]
	assert.Equal(t, "delivery.completed", event.EventType)

	// Verify data quality issue
	assert.Equal(t, domain.IssueOutOfOrderTimestamps, batch.DataQuality[0].IssueType)
}

func TestDeliveryHandler_Handle_DeliveryDeleted(t *testing.T) {
	handler := &DeliveryHandler{}
	env := testutil.NewTestEventEnvelope("delivery.deleted", "evt-123", "delivery-456")
	payload := unmarshalPayload([]byte(`{"deliveryId": "delivery-456"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.DeliveryEvents, 0)
	assert.Len(t, batch.DeliveryCompleted, 0)
	assert.Len(t, batch.Raw, 1)
	assert.Len(t, batch.Seen, 1)
}

func TestDeliveryHandler_Handle_MissingDeliveryID(t *testing.T) {
	handler := &DeliveryHandler{}
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "")
	payload := unmarshalPayload([]byte(`{"userId": "user-123"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "deliveryId")
}

func TestDeliveryHandler_Handle_InvalidEvent(t *testing.T) {
	handler := &DeliveryHandler{}
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	env.OccurredAt = time.Time{} // Invalid
	payload := unmarshalPayload([]byte(`{"deliveryId": "delivery-456"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
}

func TestMergeTimeline(t *testing.T) {
	now := time.Now().UTC()
	event := &domain.FactDeliveryEvent{
		EventID:       "evt-123",
		DeliveryID:    "delivery-456",
		UserID:        "user-123",
		DriverID:      "driver-789",
		EventType:     domain.DeliveryCreated,
		EventVersion:  1,
		CityID:        "city-1",
		ZoneID:        "zone-1",
		OccurredAt:    now.Add(-30 * time.Minute),
		IngestedAt:    now,
		CorrelationID: "corr-123",
	}

	tests := []struct {
		name           string
		existing       *domain.FactDeliveryCompleted
		eventType      string
		payload        map[string]any
		checkCompleted func(*testing.T, *domain.FactDeliveryCompleted)
	}{
		{
			name:      "new timeline from delivery.created",
			existing:  nil,
			eventType: domain.DeliveryCreated,
			payload:   map[string]any{"deliveryId": "delivery-456", "userId": "user-123", "driverId": "driver-789", "cityId": "city-1", "zoneId": "zone-1"},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.Equal(t, "delivery-456", c.DeliveryID)
				assert.Equal(t, "user-123", c.UserID)
				assert.Equal(t, "driver-789", c.DriverID)
				assert.Equal(t, "city-1", c.CityID)
				assert.True(t, c.CreatedAt.Equal(event.OccurredAt))
			},
		},
		{
			name:      "merge delivery.driver.assigned",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryDriverAssigned,
			payload:   map[string]any{"deliveryId": "delivery-456", "driverId": "driver-789", "assignedAt": now.Add(-25 * time.Minute).Format(time.RFC3339)},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.Equal(t, "driver-789", c.DriverID)
				assert.NotNil(t, c.AssignedAt)
				// Compare with tolerance since RFC3339 parsing loses nanosecond precision
				expected := now.Add(-25 * time.Minute).Truncate(time.Second)
				assert.True(t, c.AssignedAt.Equal(expected), "AssignedAt=%v, expected=%v", c.AssignedAt, expected)
			},
		},
		{
			name:      "merge delivery.driver.accepted",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryDriverAccepted,
			payload:   map[string]any{"deliveryId": "delivery-456", "driverId": "driver-789", "acceptedAt": now.Add(-24 * time.Minute).Format(time.RFC3339)},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.Equal(t, "driver-789", c.DriverID)
				assert.NotNil(t, c.AcceptedAt)
				expected := now.Add(-24 * time.Minute).Truncate(time.Second)
				assert.True(t, c.AcceptedAt.Equal(expected), "AcceptedAt=%v, expected=%v", c.AcceptedAt, expected)
			},
		},
		{
			name:      "merge delivery.pickup.started",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryPickupStarted,
			payload:   map[string]any{"deliveryId": "delivery-456", "pickupStartedAt": now.Add(-20 * time.Minute).Format(time.RFC3339)},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.NotNil(t, c.PickupStartedAt)
				expected := now.Add(-20 * time.Minute).Truncate(time.Second)
				assert.True(t, c.PickupStartedAt.Equal(expected), "PickupStartedAt=%v, expected=%v", c.PickupStartedAt, expected)
			},
		},
		{
			name:      "merge delivery.picked_up",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryPickedUp,
			payload:   map[string]any{"deliveryId": "delivery-456", "pickedUpAt": now.Add(-18 * time.Minute).Format(time.RFC3339)},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.NotNil(t, c.PickedUpAt)
				expected := now.Add(-18 * time.Minute).Truncate(time.Second)
				assert.True(t, c.PickedUpAt.Equal(expected), "PickedUpAt=%v, expected=%v", c.PickedUpAt, expected)
			},
		},
		{
			name:      "merge delivery.in_transit",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryInTransit,
			payload:   map[string]any{"deliveryId": "delivery-456", "inTransitAt": now.Add(-15 * time.Minute).Format(time.RFC3339)},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.NotNil(t, c.InTransitAt)
				expected := now.Add(-15 * time.Minute).Truncate(time.Second)
				assert.True(t, c.InTransitAt.Equal(expected), "InTransitAt=%v, expected=%v", c.InTransitAt, expected)
			},
		},
		{
			name:      "merge delivery.completed",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryCompleted,
			payload:   map[string]any{"deliveryId": "delivery-456", "deliveredAt": now.Add(-5 * time.Minute).Format(time.RFC3339), "completedAt": now.Format(time.RFC3339)},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.NotNil(t, c.DeliveredAt)
				assert.NotNil(t, c.CompletedAt)
				expectedDelivered := now.Add(-5 * time.Minute).Truncate(time.Second)
				expectedCompleted := now.Truncate(time.Second)
				assert.True(t, c.DeliveredAt.Equal(expectedDelivered), "DeliveredAt=%v, expected=%v", c.DeliveredAt, expectedDelivered)
				assert.True(t, c.CompletedAt.Equal(expectedCompleted), "CompletedAt=%v, expected=%v", c.CompletedAt, expectedCompleted)
			},
		},
		{
			name:      "update user ID from event",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", UserID: "old-user", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryDriverAccepted,
			payload:   map[string]any{"deliveryId": "delivery-456", "userId": "new-user", "driverId": "driver-789"},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				// Event values take precedence over existing
				assert.Equal(t, "user-123", c.UserID)
			},
		},
		{
			name:      "update city ID from event",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "old-city", IngestedAt: now},
			eventType: domain.DeliveryDriverAccepted,
			payload:   map[string]any{"deliveryId": "delivery-456", "cityId": "new-city"},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				// Event values take precedence over existing
				assert.Equal(t, "city-1", c.CityID)
			},
		},
		{
			name:      "fallback to event OccurredAt when payload timestamp missing",
			existing:  &domain.FactDeliveryCompleted{DeliveryID: "delivery-456", CreatedAt: now.Add(-30 * time.Minute), CityID: "city-1", IngestedAt: now},
			eventType: domain.DeliveryDriverAccepted,
			payload:   map[string]any{"deliveryId": "delivery-456"},
			checkCompleted: func(t *testing.T, c *domain.FactDeliveryCompleted) {
				assert.NotNil(t, c.AcceptedAt)
				// Falls back to event.OccurredAt (which is now.Add(-30 * time.Minute))
				expected := now.Add(-30 * time.Minute)
				assert.True(t, c.AcceptedAt.Equal(expected), "AcceptedAt=%v, expected=%v", c.AcceptedAt, expected)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create event with the correct type for this test case
			testEvent := &domain.FactDeliveryEvent{
				EventID:       "evt-123",
				DeliveryID:    "delivery-456",
				UserID:        "user-123",
				DriverID:      "driver-789",
				EventType:     tc.eventType,
				EventVersion:  1,
				CityID:        "city-1",
				ZoneID:        "zone-1",
				OccurredAt:    now.Add(-30 * time.Minute),
				IngestedAt:    now,
				CorrelationID: "corr-123",
			}
			merged := MergeTimeline(tc.existing, testEvent, tc.payload)
			require.NotNil(t, merged)
			if tc.checkCompleted != nil {
				tc.checkCompleted(t, merged)
			}
		})
	}
}

func TestMergeTimeline_NilEvent(t *testing.T) {
	// Should not panic
	merged := MergeTimeline(nil, nil, map[string]any{})
	assert.Nil(t, merged)
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "first", firstNonEmpty("first", "second"))
	assert.Equal(t, "second", firstNonEmpty("", "second"))
	assert.Equal(t, "third", firstNonEmpty("", "", "third"))
	assert.Equal(t, "", firstNonEmpty("", ""))
	assert.Equal(t, "", firstNonEmpty())
}