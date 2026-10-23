package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventVersion_IsSupported(t *testing.T) {
	tests := []struct {
		name     string
		version  EventVersion
		expected bool
	}{
		{"version 0", 0, false},
		{"version 1", 1, true},
		{"version 2", 2, false},
		{"negative version", -1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.version.IsSupported())
		})
	}
}

func TestEventEnvelope_Validate(t *testing.T) {
	now := time.Now().UTC()
	base := EventEnvelope{
		EventID:        "evt-123",
		EventType:      "delivery.created",
		EventVersion:   1,
		OccurredAt:     now,
		Producer:       "delivery-service",
		AggregateType:  "delivery",
		AggregateID:    "delivery-456",
		CorrelationID:  "corr-789",
		CausationID:    "cause-012",
		SourceTopic:    "delivery.events",
		SourcePartition: 0,
		SourceOffset:   100,
		IngestedAt:     now,
	}

	tests := []struct {
		name        string
		envelope    *EventEnvelope
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid envelope",
			envelope:    &base,
			expectError: false,
		},
		{
			name: "nil envelope",
			envelope:    nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty event ID",
			envelope:    func() *EventEnvelope { e := base; e.EventID = ""; return &e }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "whitespace event ID",
			envelope:    func() *EventEnvelope { e := base; e.EventID = "   "; return &e }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "empty event type",
			envelope:    func() *EventEnvelope { e := base; e.EventType = ""; return &e }(),
			expectError: true,
			errorMsg:    "eventType is required",
		},
		{
			name: "empty aggregate ID",
			envelope:    func() *EventEnvelope { e := base; e.AggregateID = ""; return &e }(),
			expectError: true,
			errorMsg:    "aggregateId is required",
		},
		{
			name: "unsupported event version 0",
			envelope:    func() *EventEnvelope { e := base; e.EventVersion = 0; return &e }(),
			expectError: true,
			errorMsg:    "unsupported event version",
		},
		{
			name: "unsupported event version 2",
			envelope:    func() *EventEnvelope { e := base; e.EventVersion = 2; return &e }(),
			expectError: true,
			errorMsg:    "unsupported event version",
		},
		{
			name: "zero occurred at",
			envelope:    func() *EventEnvelope { e := base; e.OccurredAt = time.Time{}; return &e }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
		{
			name: "future occurred at beyond tolerance",
			envelope:    func() *EventEnvelope { e := base; e.OccurredAt = now.Add(10 * time.Minute); return &e }(),
			expectError: true,
			errorMsg:    "occurredAt is beyond the future tolerance",
		},
		{
			name: "future occurred at within tolerance",
			envelope:    func() *EventEnvelope { e := base; e.OccurredAt = now.Add(2 * time.Minute); return &e }(),
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.envelope.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsKnownAggregateType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"delivery", "delivery", true},
		{"driver", "driver", true},
		{"assignment", "assignment", true},
		{"payment", "payment", true},
		{"notification", "notification", true},
		{"user", "user", true},
		{"unknown", "unknown", false},
		{"case insensitive", "DELIVERY", true},
		{"with spaces", " delivery ", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, IsKnownAggregateType(tc.input))
		})
	}
}