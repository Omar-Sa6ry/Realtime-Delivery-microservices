package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseHandler_NewRawLanding(t *testing.T) {
	handler := &BaseHandler{}
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	env.SourceTopic = "delivery.events"
	env.SourcePartition = 5
	env.SourceOffset = 1000
	env.CorrelationID = "corr-789"
	env.CausationID = "cause-012"
	env.IngestedAt = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	raw := handler.NewRawLanding(env)

	require.NotNil(t, raw)
	assert.Equal(t, "evt-123", raw.EventID)
	assert.Equal(t, "delivery.created", raw.EventType)
	assert.Equal(t, domain.EventVersion(1), raw.EventVersion)
	assert.Equal(t, "delivery", raw.AggregateType)
	assert.Equal(t, "delivery-456", raw.AggregateID)
	assert.Equal(t, "delivery-service", raw.Producer)
	assert.Equal(t, env.OccurredAt, raw.OccurredAt)
	assert.Equal(t, env.IngestedAt, raw.IngestedAt)
	assert.Equal(t, "corr-789", raw.CorrelationID)
	assert.Equal(t, "cause-012", raw.CausationID)
	assert.Equal(t, "delivery.events", raw.SourceTopic)
	assert.Equal(t, int32(5), raw.SourcePartition)
	assert.Equal(t, int64(1000), raw.SourceOffset)
	assert.Empty(t, raw.PayloadJSON)
}

func TestBaseHandler_NewSeen(t *testing.T) {
	handler := &BaseHandler{}
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	env.SourceTopic = "delivery.events"
	env.SourceOffset = 1000

	seen := handler.NewSeen(env)

	require.NotNil(t, seen)
	assert.Equal(t, "evt-123", seen.EventID)
	assert.Equal(t, "delivery", seen.AggregateType)
	assert.Equal(t, "delivery-456", seen.AggregateID)
	assert.Equal(t, "delivery.events", seen.SourceTopic)
	assert.Equal(t, int64(1000), seen.SourceOffset)
	assert.True(t, seen.FirstSeenAt.After(time.Now().UTC().Add(-time.Second)))
}

func TestBaseHandler_Qualify(t *testing.T) {
	handler := &BaseHandler{}
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	env.AggregateType = "delivery"
	env.AggregateID = "delivery-456"

	issue := handler.Qualify(env, domain.IssueNegativeDuration, domain.SeverityWarning, "test details")

	require.NotNil(t, issue)
	assert.Equal(t, "evt-123:negative_duration", issue.IssueID)
	assert.Equal(t, "evt-123", issue.EventID)
	assert.Equal(t, domain.IssueNegativeDuration, issue.IssueType)
	assert.Equal(t, "delivery", issue.AggregateType)
	assert.Equal(t, "delivery-456", issue.AggregateID)
	assert.Equal(t, domain.SeverityWarning, issue.Severity)
	assert.Equal(t, "test details", issue.Details)
	assert.True(t, issue.DetectedAt.After(time.Now().UTC().Add(-time.Second)))
	assert.Nil(t, issue.ResolvedAt)
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		key      string
		expected string
	}{
		{"string value", map[string]any{"key": "  value  "}, "key", "value"},
		{"json number", map[string]any{"key": json.Number("123")}, "key", "123"},
		{"float64", map[string]any{"key": float64(456.78)}, "key", "456.78"},
		{"missing key", map[string]any{}, "key", ""},
		{"nil value", map[string]any{"key": nil}, "key", ""},
		{"wrong type", map[string]any{"key": true}, "key", ""},
		{"empty string", map[string]any{"key": "   "}, "key", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetString(tc.input, tc.key)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetTime(t *testing.T) {
	now := time.Now().UTC()
	fallback := now.Add(-time.Hour)

	tests := []struct {
		name     string
		input    map[string]any
		key      string
		fallback time.Time
		expected time.Time
	}{
		{"RFC3339 string", map[string]any{"key": "2024-01-15T10:30:00Z"}, "key", fallback, time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		{"json number milliseconds", map[string]any{"key": json.Number("1705315800000")}, "key", fallback, time.UnixMilli(1705315800000).UTC()},
		{"float64 milliseconds", map[string]any{"key": float64(1705315800000)}, "key", fallback, time.UnixMilli(1705315800000).UTC()},
		{"invalid string", map[string]any{"key": "not-a-date"}, "key", fallback, fallback},
		{"missing key", map[string]any{}, "key", fallback, fallback},
		{"nil value", map[string]any{"key": nil}, "key", fallback, fallback},
		{"wrong type", map[string]any{"key": true}, "key", fallback, fallback},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetTime(tc.input, tc.key, tc.fallback)
			if tc.expected.Equal(fallback) {
				assert.Equal(t, fallback, result)
			} else {
				assert.True(t, result.Equal(tc.expected) || result.Sub(tc.expected).Abs() < time.Second)
			}
		})
	}
}

func TestGetUint32(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		key      string
		expected *uint32
	}{
		{"json number", map[string]any{"key": json.Number("123")}, "key", func() *uint32 { u := uint32(123); return &u }()},
		{"float64", map[string]any{"key": float64(456)}, "key", func() *uint32 { u := uint32(456); return &u }()},
		{"negative json number", map[string]any{"key": json.Number("-123")}, "key", nil},
		{"negative float64", map[string]any{"key": float64(-456)}, "key", nil},
		{"missing key", map[string]any{}, "key", nil},
		{"nil value", map[string]any{"key": nil}, "key", nil},
		{"wrong type", map[string]any{"key": "string"}, "key", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetUint32(tc.input, tc.key)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tc.expected, *result)
			}
		})
	}
}

func TestGetUint64(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		key      string
		expected *uint64
	}{
		{"json number", map[string]any{"key": json.Number("123456789012")}, "key", func() *uint64 { u := uint64(123456789012); return &u }()},
		{"float64", map[string]any{"key": float64(9876543210)}, "key", func() *uint64 { u := uint64(9876543210); return &u }()},
		{"negative float64", map[string]any{"key": float64(-123)}, "key", nil},
		{"missing key", map[string]any{}, "key", nil},
		{"nil value", map[string]any{"key": nil}, "key", nil},
		{"wrong type", map[string]any{"key": "string"}, "key", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetUint64(tc.input, tc.key)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tc.expected, *result)
			}
		})
	}
}

func TestFactBatch_RowCount(t *testing.T) {
	tests := []struct {
		name     string
		batch    *FactBatch
		expected int
	}{
		{"nil batch", nil, 0},
		{"empty batch", &FactBatch{}, 0},
		{"with raw events", &FactBatch{Raw: []*domain.RawEventLanding{{}, {}}}, 2},
		{"with delivery events", &FactBatch{DeliveryEvents: []*domain.FactDeliveryEvent{{}, {}}}, 2},
		{"with delivery completed", &FactBatch{DeliveryCompleted: []*domain.FactDeliveryCompleted{{}, {}}}, 2},
		{"with driver assignments", &FactBatch{DriverAssignments: []*domain.FactDriverAssignment{{}, {}}}, 2},
		{"with payment transactions", &FactBatch{PaymentTransactions: []*domain.FactPaymentTransaction{{}, {}}}, 2},
		{"with notification events", &FactBatch{NotificationEvents: []*domain.FactNotificationEvent{{}, {}}}, 2},
		{"with data quality", &FactBatch{DataQuality: []*domain.DataQualityIssue{{}, {}}}, 2},
		{"mixed", &FactBatch{
			Raw:                 []*domain.RawEventLanding{{}},
			DeliveryEvents:      []*domain.FactDeliveryEvent{{}},
			DeliveryCompleted:   []*domain.FactDeliveryCompleted{{}},
			DriverAssignments:   []*domain.FactDriverAssignment{{}},
			PaymentTransactions: []*domain.FactPaymentTransaction{{}},
			NotificationEvents:  []*domain.FactNotificationEvent{{}},
			DataQuality:         []*domain.DataQualityIssue{{}},
		}, 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.batch.RowCount())
		})
	}
}