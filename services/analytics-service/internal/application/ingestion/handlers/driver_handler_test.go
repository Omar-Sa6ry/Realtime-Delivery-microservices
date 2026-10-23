package handlers

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriverHandler_Handles(t *testing.T) {
	handler := &DriverHandler{}

	tests := []struct {
		name     string
		eventType string
		expected bool
	}{
		{"driver.assignment.offered", "driver.assignment.offered", true},
		{"driver.assignment.accepted", "driver.assignment.accepted", true},
		{"driver.assignment.rejected", "driver.assignment.rejected", true},
		{"driver.assignment.expired", "driver.assignment.expired", true},
		{"driver.assignment.released", "driver.assignment.released", true},
		{"driver.available", "driver.available", true},
		{"driver.unavailable", "driver.unavailable", true},
		{"delivery.created", "delivery.created", false},
		{"payment.created", "payment.created", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, handler.Handles(tc.eventType))
		})
	}
}

func TestDriverHandler_Handle_DriverAssignmentOffered(t *testing.T) {
	handler := &DriverHandler{}
	env := testutil.NewTestEventEnvelope("driver.assignment.offered", "evt-123", "assign-456")
	payload, _ := testutil.NewTestDriverAssignmentOfferedPayload("assign-456", "delivery-789", "driver-123")
	p := unmarshalPayload(payload)

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, p)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.Raw, 1)
	assert.Len(t, batch.Seen, 1)
	assert.Len(t, batch.DriverAssignments, 1)

	assignment := batch.DriverAssignments[0]
	assert.Equal(t, "assign-456", assignment.AssignmentID)
	assert.Equal(t, "delivery-789", assignment.DeliveryID)
	assert.Equal(t, "driver-123", assignment.DriverID)
	assert.Equal(t, domain.AssignmentOffered, assignment.Result)
	assert.NotNil(t, assignment.OfferedAt)
	assert.Nil(t, assignment.ResponseTimeMs) // No response yet for offered
}

func TestDriverHandler_Handle_DriverAssignmentAccepted(t *testing.T) {
	handler := &DriverHandler{}
	env := testutil.NewTestEventEnvelope("driver.assignment.accepted", "evt-123", "assign-456")
	payload := unmarshalPayload([]byte(`{
		"assignmentId": "assign-456",
		"deliveryId": "delivery-789",
		"driverId": "driver-123",
		"offeredAt": "2024-01-15T10:00:00Z",
		"respondedAt": "2024-01-15T10:02:00Z"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.DriverAssignments, 1)

	assignment := batch.DriverAssignments[0]
	assert.Equal(t, domain.AssignmentAccepted, assignment.Result)
	assert.NotNil(t, assignment.AcceptedAt)
	assert.NotNil(t, assignment.ResponseTimeMs)
	assert.Equal(t, uint32(120000), *assignment.ResponseTimeMs) // 2 minutes = 120000 ms
}

func TestDriverHandler_Handle_DriverAvailable(t *testing.T) {
	handler := &DriverHandler{}
	env := testutil.NewTestEventEnvelope("driver.available", "evt-123", "driver-456")
	payload := unmarshalPayload([]byte(`{"driverId": "driver-456"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.Raw, 1)
	assert.Len(t, batch.Seen, 1)
	assert.Len(t, batch.DriverAssignments, 0) // No assignment for available/unavailable
}

func TestDriverHandler_Handle_MissingAssignmentID(t *testing.T) {
	handler := &DriverHandler{}
	env := testutil.NewTestEventEnvelope("driver.assignment.offered", "evt-123", "")
	payload := unmarshalPayload([]byte(`{"deliveryId": "delivery-789", "driverId": "driver-123"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "assignmentId")
}

func TestDriverHandler_Handle_InvalidAssignmentResult(t *testing.T) {
	handler := &DriverHandler{}
	env := testutil.NewTestEventEnvelope("driver.assignment.offered", "evt-123", "assign-456")
	payload := unmarshalPayload([]byte(`{"assignmentId": "assign-456", "driverId": "driver-123"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
}