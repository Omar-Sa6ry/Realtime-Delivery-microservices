package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventEnvelope(t *testing.T) {
	before := time.Now().UnixMilli()
	env, err := NewEventEnvelope("evt-1", "delivery.created", "trace-1", map[string]string{"id": "d-1"})
	require.NoError(t, err)
	after := time.Now().UnixMilli()

	assert.Equal(t, "evt-1", env.EventID)
	assert.Equal(t, "delivery.created", env.EventType)
	assert.Equal(t, 1, env.EventVersion)
	assert.Equal(t, "trace-1", env.TraceID)
	assert.Equal(t, "unknown", env.Producer)
	assert.GreaterOrEqual(t, env.Timestamp, before)
	assert.LessOrEqual(t, env.Timestamp, after)
	assert.Equal(t, env.Timestamp, env.OccurredAt)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, "d-1", payload["id"])
}

func TestNewEventEnvelopeMarshalError(t *testing.T) {
	env, err := NewEventEnvelope("evt-1", "x", "trace", make(chan int))
	require.Error(t, err)
	assert.Nil(t, env)
	assert.Contains(t, err.Error(), "marshal event payload")
}

func TestMarshalEnvelopeRoundTrip(t *testing.T) {
	raw, err := MarshalEnvelope("evt-2", "payment.captured", "trace-2",
		PaymentCapturedPayload{PaymentID: "p-1", AmountMinor: 1500, Currency: "USD"})
	require.NoError(t, err)

	env, err := UnmarshalEnvelope(raw)
	require.NoError(t, err)

	assert.Equal(t, "evt-2", env.EventID)
	assert.Equal(t, "payment.captured", env.EventType)
	assert.Equal(t, "trace-2", env.TraceID)

	var payload PaymentCapturedPayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, "p-1", payload.PaymentID)
	assert.Equal(t, int64(1500), payload.AmountMinor)
}

func TestMarshalEnvelopeInvalidPayload(t *testing.T) {
	raw, err := MarshalEnvelope("evt", "type", "trace", func() {})
	require.Error(t, err)
	assert.Nil(t, raw)
}

func TestUnmarshalEnvelopeInvalidJSON(t *testing.T) {
	env, err := UnmarshalEnvelope([]byte("{not json"))
	require.Error(t, err)
	assert.Nil(t, env)
	assert.Contains(t, err.Error(), "unmarshal event envelope")
}

func TestMarshalMediaEnvelope(t *testing.T) {
	raw, err := MarshalMediaEnvelope("m-1", MediaReady, "trace", MediaReadyPayload{
		MediaID:  "media-1",
		FileName: "a.png",
		Versions: []MediaVersionInfo{{VersionType: "THUMB", ObjectKey: "k"}},
	})
	require.NoError(t, err)

	env, err := UnmarshalEnvelope(raw)
	require.NoError(t, err)
	assert.Equal(t, string(MediaReady), env.EventType)

	// The media helper must produce the same envelope shape as NewMediaEventEnvelope.
	direct, err := NewMediaEventEnvelope("m-1", MediaReady, "trace", MediaReadyPayload{MediaID: "media-1"})
	require.NoError(t, err)
	assert.Equal(t, env.EventType, direct.EventType)
	assert.Equal(t, env.EventID, direct.EventID)
}

func TestMarshalPaymentEnvelope(t *testing.T) {
	raw, err := MarshalPaymentEnvelope("pay-1", PaymentAuthorized, "trace",
		PaymentAuthorizedPayload{PaymentID: "p-9", AmountMinor: 42})
	require.NoError(t, err)

	env, err := UnmarshalEnvelope(raw)
	require.NoError(t, err)
	assert.Equal(t, string(PaymentAuthorized), env.EventType)

	direct, err := NewPaymentEventEnvelope("pay-1", PaymentAuthorized, "trace",
		PaymentAuthorizedPayload{PaymentID: "p-9"})
	require.NoError(t, err)
	assert.Equal(t, env.EventID, direct.EventID)
}

func TestEnvelopeFlexibleTimeParsing(t *testing.T) {
	tests := []struct {
		name           string
		occurredAt     string
		timestamp      string
		wantOccurredAt int64
		wantTimestamp  int64
	}{
		{
			name:           "iso occurredAt with numeric timestamp",
			occurredAt:     `"2026-09-22T14:44:00Z"`,
			timestamp:      "1790088240000",
			wantOccurredAt: time.Date(2026, 9, 22, 14, 44, 0, 0, time.UTC).UnixMilli(),
			wantTimestamp:  1790088240000,
		},
		{
			name:           "numeric occurredAt with iso timestamp",
			occurredAt:     "1790088240000",
			timestamp:      `"2026-09-22T14:44:00Z"`,
			wantOccurredAt: 1790088240000,
			wantTimestamp:  time.Date(2026, 9, 22, 14, 44, 0, 0, time.UTC).UnixMilli(),
		},
		{
			name:           "unparseable timestamp becomes zero",
			occurredAt:     `"not-a-date"`,
			timestamp:      `"also-not-a-date"`,
			wantOccurredAt: 0,
			wantTimestamp:  0,
		},
		{
			name:           "missing fields become zero",
			occurredAt:     "",
			timestamp:      "",
			wantOccurredAt: 0,
			wantTimestamp:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := `{"eventId":"e","eventType":"t","eventVersion":1,` +
				`"occurredAt":` + tt.occurredAt + `,"timestamp":` + tt.timestamp + `}`
			if tt.occurredAt == "" {
				raw = `{"eventId":"e","eventType":"t","eventVersion":1}`
			}

			env, err := UnmarshalEnvelope([]byte(raw))
			require.NoError(t, err)
			assert.Equal(t, tt.wantOccurredAt, env.OccurredAt)
			assert.Equal(t, tt.wantTimestamp, env.Timestamp)
		})
	}
}

func TestEnvelopeUnmarshalStructError(t *testing.T) {
	var env EventEnvelope
	err := env.UnmarshalJSON([]byte("not json"))
	require.Error(t, err)
}

func TestPayloadStructsJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name string
		in   interface{}
		out  interface{}
	}{
		{
			name: "delivery created",
			in: DeliveryCreatedPayload{
				DeliveryID: "d-1", CustomerID: "c-1", Status: "PENDING",
				Pickup:    DeliveryAddress{City: "Cairo", Country: "EG", Location: DeliveryLocation{Lat: 1, Lon: 2}},
				Dropoff:   DeliveryAddress{City: "Giza", Country: "EG"},
				CreatedAt: now, UpdatedAt: now, SourceVersion: 3,
			},
			out: &DeliveryCreatedPayload{},
		},
		{
			name: "delivery driver assigned",
			in:   DeliveryDriverAssignedPayload{DeliveryID: "d-1", DriverID: "dr-1", AssignedAt: now, SourceVersion: 2},
			out:  &DeliveryDriverAssignedPayload{},
		},
		{
			name: "delivery deleted",
			in:   DeliveryDeletedPayload{DeliveryID: "d-1", DeletedAt: now},
			out:  &DeliveryDeletedPayload{},
		},
		{
			name: "payment authorized",
			in:   PaymentAuthorizedPayload{PaymentID: "p-1", AmountMinor: 100, AuthorizedAmountMinor: 100, AuthorizedAt: now},
			out:  &PaymentAuthorizedPayload{},
		},
		{
			name: "payment refund failed",
			in: PaymentRefundFailedPayload{
				PaymentID: "p-1", ErrorCode: "E", ErrorCategory: "C", ErrorMessage: "M", FailedAt: now,
			},
			out: &PaymentRefundFailedPayload{},
		},
		{
			name: "driver assignment offered",
			in: DriverAssignmentOfferedPayload{
				AssignmentID: "a-1", DriverID: "dr-1", DeliveryID: "d-1",
				ExpiresAt: now.Format(time.RFC3339), RadiusKm: 5, DistanceMeters: 1200,
				PickupAddress: &DeliveryAddress{City: "Cairo"},
			},
			out: &DriverAssignmentOfferedPayload{},
		},
		{
			name: "driver created",
			in: DriverCreatedPayload{
				DriverID: "dr-1", Name: "Ali", Status: "AVAILABLE",
				VehicleType: "CAR", Rating: 4.5,
				Location:  &DriverGeoPoint{Lat: 30, Lon: 31},
				UpdatedAt: now, SourceVersion: 1,
			},
			out: &DriverCreatedPayload{},
		},
		{
			name: "driver no driver available",
			in:   DriverNoDriverAvailablePayload{DeliveryID: "d-1", CustomerID: "c-1", AttemptNumber: 2, Reason: "NO_DRIVERS_NEARBY", TriedAt: now.Format(time.RFC3339)},
			out:  &DriverNoDriverAvailablePayload{},
		},
		{
			name: "user created",
			in:   UserCreatedPayload{UserID: "u-1", Email: "a@b.c", Role: "user", CreatedAt: now},
			out:  &UserCreatedPayload{},
		},
		{
			name: "user updated with avatar",
			in: UserUpdatedPayload{
				UserID: "u-1", Email: "a@b.c", Role: "user", IsActive: true,
				CreatedAt: now, UpdatedAt: now, AvatarID: strPtr("media-1"),
			},
			out: &UserUpdatedPayload{},
		},
		{
			name: "media ready",
			in: MediaReadyPayload{
				MediaID: "m-1", FileName: "a.png", Size: 10, Versions: []MediaVersionInfo{
					{VersionType: "ORIGINAL", ObjectKey: "k", Width: 100, Height: 100},
				},
			},
			out: &MediaReadyPayload{},
		},
		{
			name: "media scan completed",
			in:   MediaScanCompletedPayload{MediaID: "m-1", UserID: "u-1", Infected: true, Threat: "EICAR"},
			out:  &MediaScanCompletedPayload{},
		},
		{
			name: "notification failed",
			in:   NotificationFailedPayload{NotificationID: "n-1", Error: "boom", FailedAt: now, RetryCount: 3},
			out:  &NotificationFailedPayload{},
		},
		{
			name: "search query completed",
			in: SearchQueryCompletedPayload{
				QueryHash: "h", Index: "deliveries", LatencyMs: 12,
				ResultCount: 4, CacheHit: true, CompletedAt: now,
			},
			out: &SearchQueryCompletedPayload{},
		},
		{
			name: "analytics dlq",
			in: AnalyticsDLQPayload{
				EventID: "e-1", EventType: "x", OccurredAt: now.Format(time.RFC3339),
				Producer: "svc", Error: "err", AttemptCount: 3, FailedAt: now.Format(time.RFC3339),
			},
			out: &AnalyticsDLQPayload{},
		},
		{
			name: "notification payload",
			in: NotificationPayload{
				NotificationID: "n-1", RecipientType: "email", RecipientValue: "a@b.c",
				Channel: "EMAIL", TemplateData: map[string]string{"k": "v"}, SentAt: now.Format(time.RFC3339),
			},
			out: &NotificationPayload{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.in)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, tt.out))

			again, err := json.Marshal(tt.out)
			require.NoError(t, err)

			var first, second map[string]interface{}
			require.NoError(t, json.Unmarshal(raw, &first))
			require.NoError(t, json.Unmarshal(again, &second))
			assert.Equal(t, first, second)
		})
	}
}

func strPtr(s string) *string { return &s }
