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

func TestNotificationHandler_Handles(t *testing.T) {
	handler := &NotificationHandler{}

	tests := []struct {
		name     string
		eventType string
		expected bool
	}{
		{"notification.created", "notification.created", true},
		{"notification.sent", "notification.sent", true},
		{"notification.delivered", "notification.delivered", true},
		{"notification.failed", "notification.failed", true},
		{"notification.retrying", "notification.retrying", true},
		{"delivery.created", "delivery.created", false},
		{"driver.available", "driver.available", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, handler.Handles(tc.eventType))
		})
	}
}

func TestNotificationHandler_Handle_NotificationCreated(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.created", "evt-123", "notif-456")
	payload, _ := testutil.NewTestNotificationCreatedPayload("notif-456")
	p := unmarshalPayload(payload)

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, p)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.Raw, 1)
	assert.Len(t, batch.Seen, 1)
	assert.Len(t, batch.NotificationEvents, 1)

	event := batch.NotificationEvents[0]
	assert.Equal(t, "evt-123", event.EventID)
	assert.Equal(t, "notif-456", event.NotificationID)
	assert.Equal(t, "user", event.RecipientType)
	assert.Equal(t, domain.NotificationChannelPush, event.Channel)
	assert.Equal(t, domain.NotificationStatusCreated, event.Status)
	assert.Equal(t, "template-123", event.TemplateID)
	assert.Equal(t, 0, event.RetryCount)
	assert.Nil(t, event.SentAt)
	assert.Nil(t, event.DeliveredAt)
	assert.Nil(t, event.FailedAt)
}

func TestNotificationHandler_Handle_NotificationSent(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.sent", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user",
		"channel": "push",
		"templateId": "template-123",
		"sentAt": "2024-01-15T10:30:00Z"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	event := batch.NotificationEvents[0]
	assert.Equal(t, domain.NotificationStatusSent, event.Status)
	assert.NotNil(t, event.SentAt)
	assert.True(t, event.SentAt.Equal(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)))
}

func TestNotificationHandler_Handle_NotificationDelivered(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.delivered", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user",
		"channel": "email",
		"deliveredAt": "2024-01-15T10:35:00Z"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	event := batch.NotificationEvents[0]
	assert.Equal(t, domain.NotificationChannelEmail, event.Channel)
	assert.Equal(t, domain.NotificationStatusDelivered, event.Status)
	assert.NotNil(t, event.DeliveredAt)
	assert.True(t, event.DeliveredAt.Equal(time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC)))
}

func TestNotificationHandler_Handle_NotificationFailed(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.failed", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user",
		"channel": "push",
		"error": "Device not registered",
		"failedAt": "2024-01-15T10:40:00Z",
		"retryCount": 3
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	event := batch.NotificationEvents[0]
	assert.Equal(t, domain.NotificationStatusFailed, event.Status)
	assert.Equal(t, "Device not registered", event.Error)
	assert.Equal(t, 3, event.RetryCount)
	assert.NotNil(t, event.FailedAt)
}

func TestNotificationHandler_Handle_DefaultChannel(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.created", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	event := batch.NotificationEvents[0]
	assert.Equal(t, domain.NotificationChannelPush, event.Channel)
}

func TestNotificationHandler_Handle_MissingNotificationID(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.created", "evt-123", "")
	payload := unmarshalPayload([]byte(`{"recipientType": "user"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "notificationId")
}

func TestNotificationHandler_Handle_InvalidChannel(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.created", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user",
		"channel": "invalid"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "unknown event type")
}

func TestNotificationHandler_Handle_InvalidStatus(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.unknown", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "unknown event type")
}

func TestNotificationHandler_Handle_NegativeRetryCount(t *testing.T) {
	handler := &NotificationHandler{}
	env := testutil.NewTestEventEnvelope("notification.created", "evt-123", "notif-456")
	payload := unmarshalPayload([]byte(`{
		"notificationId": "notif-456",
		"recipientType": "user",
		"retryCount": -1
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "retryCount must not be negative")
}

func TestGetRetryCount(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected int
	}{
		{"float64 positive", map[string]any{"retryCount": float64(3)}, 3},
		{"float64 zero", map[string]any{"retryCount": float64(0)}, 0},
		{"float64 negative", map[string]any{"retryCount": float64(-1)}, 0},
		{"string number", map[string]any{"retryCount": "5"}, 5},
		{"string invalid", map[string]any{"retryCount": "abc"}, 0},
		{"missing", map[string]any{}, 0},
		{"nil", map[string]any{"retryCount": nil}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := getRetryCount(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}