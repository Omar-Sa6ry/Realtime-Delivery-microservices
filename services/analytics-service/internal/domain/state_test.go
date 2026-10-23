package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIdempotencyRecord(t *testing.T) {
	record := NewIdempotencyRecord("evt-123", "delivery", "delivery-456", "delivery.events", 100)

	require.NotNil(t, record)
	assert.Equal(t, "evt-123", record.EventID)
	assert.Equal(t, "delivery", record.AggregateType)
	assert.Equal(t, "delivery-456", record.AggregateID)
	assert.Equal(t, "delivery.events", record.SourceTopic)
	assert.Equal(t, int64(100), record.SourceOffset)
	assert.True(t, record.FirstSeenAt.After(time.Now().UTC().Add(-time.Second)))
}

func TestIdempotencyRecord_Validate(t *testing.T) {
	record := NewIdempotencyRecord("evt-123", "delivery", "delivery-456", "delivery.events", 100)

	tests := []struct {
		name        string
		record      *IdempotencyRecord
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid record",
			record:      record,
			expectError: false,
		},
		{
			name: "nil record",
			record:      nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty event ID",
			record:      func() *IdempotencyRecord { r := NewIdempotencyRecord("", "delivery", "delivery-456", "delivery.events", 100); return r }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "zero first seen at",
			record:      func() *IdempotencyRecord { r := NewIdempotencyRecord("evt-123", "delivery", "delivery-456", "delivery.events", 100); r.FirstSeenAt = time.Time{}; return r }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.record.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewIngestionCheckpoint(t *testing.T) {
	checkpoint := NewIngestionCheckpoint("delivery.events", 0, 100)

	require.NotNil(t, checkpoint)
	assert.Equal(t, "delivery.events", checkpoint.Topic)
	assert.Equal(t, int32(0), checkpoint.Partition)
	assert.Equal(t, int64(100), checkpoint.Offset)
	assert.True(t, checkpoint.UpdatedAt.After(time.Now().UTC().Add(-time.Second)))
}

func TestIngestionCheckpoint_Validate(t *testing.T) {
	checkpoint := NewIngestionCheckpoint("delivery.events", 0, 100)

	tests := []struct {
		name        string
		checkpoint  *IngestionCheckpoint
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid checkpoint",
			checkpoint:  checkpoint,
			expectError: false,
		},
		{
			name: "nil checkpoint",
			checkpoint:  nil,
			expectError: true,
			errorMsg:    "invalid ingestion checkpoint",
		},
		{
			name: "empty topic",
			checkpoint:  func() *IngestionCheckpoint { c := NewIngestionCheckpoint("", 0, 100); return c }(),
			expectError: true,
			errorMsg:    "invalid ingestion checkpoint",
		},
		{
			name: "negative offset",
			checkpoint:  func() *IngestionCheckpoint { c := NewIngestionCheckpoint("delivery.events", 0, -1); return c }(),
			expectError: true,
			errorMsg:    "invalid ingestion checkpoint",
		},
		{
			name: "zero updated at",
			checkpoint:  func() *IngestionCheckpoint { c := NewIngestionCheckpoint("delivery.events", 0, 100); c.UpdatedAt = time.Time{}; return c }(),
			expectError: true,
			errorMsg:    "invalid ingestion checkpoint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.checkpoint.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIngestionCheckpoint_Advance(t *testing.T) {
	checkpoint := NewIngestionCheckpoint("delivery.events", 0, 100)
	originalUpdatedAt := checkpoint.UpdatedAt

	// Advance to higher offset
	advanced := checkpoint.Advance(200, time.Now().UTC().Add(time.Minute))
	assert.True(t, advanced)
	assert.Equal(t, int64(200), checkpoint.Offset)
	assert.True(t, checkpoint.UpdatedAt.After(originalUpdatedAt))

	// Try to advance to same offset
	advanced = checkpoint.Advance(200, time.Now().UTC().Add(2*time.Minute))
	assert.False(t, advanced)
	assert.Equal(t, int64(200), checkpoint.Offset)

	// Try to advance to lower offset
	advanced = checkpoint.Advance(150, time.Now().UTC().Add(3*time.Minute))
	assert.False(t, advanced)
	assert.Equal(t, int64(200), checkpoint.Offset)

	// Advance again
	advanced = checkpoint.Advance(300, time.Now().UTC().Add(4*time.Minute))
	assert.True(t, advanced)
	assert.Equal(t, int64(300), checkpoint.Offset)

	// Nil checkpoint
	var nilCheckpoint *IngestionCheckpoint
	advanced = nilCheckpoint.Advance(100, time.Now().UTC())
	assert.False(t, advanced)
}

func TestRawEventLanding_Validate(t *testing.T) {
	now := time.Now().UTC()
	base := RawEventLanding{
		EventID:         "evt-123",
		EventType:       "delivery.created",
		EventVersion:    1,
		AggregateType:   "delivery",
		AggregateID:     "delivery-456",
		Producer:        "delivery-service",
		OccurredAt:      now,
		IngestedAt:      now,
		CorrelationID:   "corr-123",
		CausationID:     "cause-456",
		SourceTopic:     "delivery.events",
		SourcePartition: 0,
		SourceOffset:    100,
		PayloadJSON:     `{"deliveryId": "delivery-456"}`,
	}

	tests := []struct {
		name        string
		landing     *RawEventLanding
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid landing",
			landing:     &base,
			expectError: false,
		},
		{
			name: "nil landing",
			landing:     nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty event ID",
			landing:     func() *RawEventLanding { l := base; l.EventID = ""; return &l }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "empty event type",
			landing:     func() *RawEventLanding { l := base; l.EventType = ""; return &l }(),
			expectError: true,
			errorMsg:    "eventType is required",
		},
		{
			name: "empty aggregate ID",
			landing:     func() *RawEventLanding { l := base; l.AggregateID = ""; return &l }(),
			expectError: true,
			errorMsg:    "aggregateId is required",
		},
		{
			name: "unsupported event version",
			landing:     func() *RawEventLanding { l := base; l.EventVersion = 2; return &l }(),
			expectError: true,
			errorMsg:    "unsupported event version",
		},
		{
			name: "zero occurred at",
			landing:     func() *RawEventLanding { l := base; l.OccurredAt = time.Time{}; return &l }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.landing.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRawEventLanding_ExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	landing := &RawEventLanding{
		EventID:     "evt-123",
		OccurredAt:  now,
		EventVersion: 1,
		AggregateID: "delivery-456",
		EventType:   "delivery.created",
	}

	expiresAt := landing.ExpiresAt()
	expected := now.Add(RawEventRetentionTTL)
	assert.Equal(t, expected, expiresAt)
}

func TestRawEventLanding_IsExpired(t *testing.T) {
	now := time.Now().UTC()
	landing := &RawEventLanding{
		EventID:     "evt-123",
		OccurredAt:  now.Add(-100 * 24 * time.Hour), // 100 days ago
		EventVersion: 1,
		AggregateID: "delivery-456",
		EventType:   "delivery.created",
	}

	assert.True(t, landing.IsExpired(now))

	landing.OccurredAt = now.Add(-50 * 24 * time.Hour) // 50 days ago
	assert.False(t, landing.IsExpired(now))

	landing.OccurredAt = now.Add(-89 * 24 * time.Hour) // 89 days ago
	assert.False(t, landing.IsExpired(now))

	landing.OccurredAt = now.Add(-91 * 24 * time.Hour) // 91 days ago
	assert.True(t, landing.IsExpired(now))
}