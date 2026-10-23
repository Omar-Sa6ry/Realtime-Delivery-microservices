package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationChannel_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		channel  NotificationChannel
		expected bool
	}{
		{"email", NotificationChannelEmail, true},
		{"push", NotificationChannelPush, true},
		{"sms", NotificationChannelSMS, true},
		{"in_app", NotificationChannelInApp, true},
		{"invalid", NotificationChannel("INVALID"), false},
		{"empty", NotificationChannel(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.channel.IsValid())
		})
	}
}

func TestNotificationStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   NotificationStatus
		expected bool
	}{
		{"created", NotificationStatusCreated, true},
		{"sent", NotificationStatusSent, true},
		{"delivered", NotificationStatusDelivered, true},
		{"failed", NotificationStatusFailed, true},
		{"retrying", NotificationStatusRetrying, true},
		{"invalid", NotificationStatus("INVALID"), false},
		{"empty", NotificationStatus(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.status.IsValid())
		})
	}
}

func TestFactNotificationEvent_Validate(t *testing.T) {
	now := time.Now().UTC()
	base := FactNotificationEvent{
		EventID:        "evt-123",
		NotificationID: "notif-456",
		RecipientType:  "user",
		Channel:        NotificationChannelPush,
		Status:         NotificationStatusSent,
		TemplateID:     "template-123",
		RetryCount:     0,
		Error:          "",
		OccurredAt:     now,
		IngestedAt:     now,
	}

	tests := []struct {
		name        string
		event       *FactNotificationEvent
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
			event:       func() *FactNotificationEvent { e := base; e.EventID = ""; return &e }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "empty notification ID",
			event:       func() *FactNotificationEvent { e := base; e.NotificationID = ""; return &e }(),
			expectError: true,
			errorMsg:    "notificationId is required",
		},
		{
			name: "invalid channel",
			event:       func() *FactNotificationEvent { e := base; e.Channel = "INVALID"; return &e }(),
			expectError: true,
			errorMsg:    "unknown event type",
		},
		{
			name: "invalid status",
			event:       func() *FactNotificationEvent { e := base; e.Status = "INVALID"; return &e }(),
			expectError: true,
			errorMsg:    "unknown event type",
		},
		{
			name: "zero occurred at",
			event:       func() *FactNotificationEvent { e := base; e.OccurredAt = time.Time{}; return &e }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
		{
			name: "negative retry count",
			event:       func() *FactNotificationEvent { e := base; e.RetryCount = -1; return &e }(),
			expectError: true,
			errorMsg:    "retryCount must not be negative",
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