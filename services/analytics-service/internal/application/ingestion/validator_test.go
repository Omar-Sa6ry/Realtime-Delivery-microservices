package ingestion

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeEnvelope(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		checkEnv    func(*testing.T, *domain.EventEnvelope, handlers.Payload)
	}{
		{
			name: "valid envelope with all fields",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"correlationId": "corr-789",
				"causationId": "cause-012",
				"payload": {"deliveryId": "delivery-456", "userId": "user-123"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "evt-123", env.EventID)
				assert.Equal(t, "delivery.created", env.EventType)
				assert.Equal(t, domain.EventVersion(1), env.EventVersion)
				assert.Equal(t, "delivery-service", env.Producer)
				assert.Equal(t, "delivery", env.AggregateType)
				assert.Equal(t, "delivery-456", env.AggregateID)
				assert.Equal(t, "corr-789", env.CorrelationID)
				assert.Equal(t, "cause-012", env.CausationID)
				assert.Equal(t, "delivery-456", payload.Map["deliveryId"])
				assert.Equal(t, "user-123", payload.Map["userId"])
			},
		},
		{
			name: "valid envelope with timestamp fallback",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"timestamp": 1705315800000,
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"payload": {"deliveryId": "delivery-456"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "evt-123", env.EventID)
				assert.Equal(t, "delivery.created", env.EventType)
				// timestamp is in milliseconds
				expectedTime := time.UnixMilli(1705315800000).UTC()
				assert.True(t, env.OccurredAt.Equal(expectedTime) || env.OccurredAt.Sub(expectedTime).Abs() < time.Second)
			},
		},
		{
			name: "missing occurredAt and timestamp",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"payload": {"deliveryId": "delivery-456"}
			}`,
			expectError: true,
		},
		{
			name: "invalid occurredAt format",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"occurredAt": "invalid-date",
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"payload": {"deliveryId": "delivery-456"}
			}`,
			expectError: true,
		},
		{
			name: "empty payload",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"payload": {}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "delivery-456", env.AggregateID) // from envelope
			},
		},
		{
			name: "aggregateId from payload fallback",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "",
				"payload": {"deliveryId": "delivery-from-payload"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "delivery-from-payload", env.AggregateID)
			},
		},
		{
			name: "producer fallback",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"payload": {"deliveryId": "delivery-456"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "delivery-service", env.Producer)
			},
		},
		{
			name: "invalid JSON",
			jsonData: `{invalid json}`,
			expectError: true,
		},
		{
			name: "eventVersion defaults to 1",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "delivery-service",
				"aggregateType": "delivery",
				"aggregateId": "delivery-456",
				"payload": {"deliveryId": "delivery-456"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, domain.EventVersion(1), env.EventVersion)
			},
		},
		{
			name: "aggregateType from eventType fallback",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "delivery.created",
				"eventVersion": 1,
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "delivery-service",
				"aggregateType": "",
				"aggregateId": "delivery-456",
				"payload": {"deliveryId": "delivery-456"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "delivery", env.AggregateType)
			},
		},
		{
			name: "aggregateId from driverId fallback",
			jsonData: `{
				"eventId": "evt-123",
				"eventType": "driver.assignment.offered",
				"eventVersion": 1,
				"occurredAt": "2024-01-15T10:30:00Z",
				"producer": "driver-service",
				"aggregateType": "driver",
				"aggregateId": "",
				"payload": {"assignmentId": "assign-123", "driverId": "driver-456"}
			}`,
			expectError: false,
			checkEnv: func(t *testing.T, env *domain.EventEnvelope, payload handlers.Payload) {
				assert.Equal(t, "driver-456", env.AggregateID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, payload, err := DecodeEnvelope([]byte(tc.jsonData))
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.checkEnv != nil {
					tc.checkEnv(t, env, payload)
				}
			}
		})
	}
}

func TestParseFlexTime(t *testing.T) {
	tests := []struct {
		name        string
		input       json.RawMessage
		expectError bool
		expected    time.Time
	}{
		{
			name:     "RFC3339 string",
			input:    json.RawMessage(`"2024-01-15T10:30:00Z"`),
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "unix milliseconds number",
			input:    json.RawMessage(`1705315800000`),
			expected: time.UnixMilli(1705315800000).UTC(),
		},
		{
			name:     "unix milliseconds string",
			input:    json.RawMessage(`"1705315800000"`),
			expected: time.UnixMilli(1705315800000).UTC(),
		},
		{
			name:        "empty string",
			input:        json.RawMessage(`""`),
			expectError:   true,
		},
		{
			name:        "null",
			input:        json.RawMessage(`null`),
			expectError:   true,
		},
		{
			name:        "missing",
			input:        json.RawMessage(``),
			expectError:   true,
		},
		{
			name:        "invalid format",
			input:        json.RawMessage(`"not-a-date"`),
			expectError:   true,
		},
		{
			name:        "invalid number",
			input:        json.RawMessage(`not-a-number`),
			expectError:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseFlexTime(tc.input)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, result.Equal(tc.expected) || result.Sub(tc.expected).Abs() < time.Second)
			}
		})
	}
}

func TestValidator_ValidateEnvelope(t *testing.T) {
	validator := NewValidator()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		env         *domain.EventEnvelope
		expectError bool
	}{
		{
			name: "valid envelope",
			env: &domain.EventEnvelope{
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
			},
			expectError: false,
		},
		{
			name: "nil envelope",
			env:  nil,
			expectError: true,
		},
		{
			name: "empty event ID",
			env: func() *domain.EventEnvelope {
				e := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
				e.EventID = ""
				return e
			}(),
			expectError: true,
		},
		{
			name: "empty event type",
			env: func() *domain.EventEnvelope {
				e := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
				e.EventType = ""
				return e
			}(),
			expectError: true,
		},
		{
			name: "empty aggregate ID",
			env: func() *domain.EventEnvelope {
				e := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
				e.AggregateID = ""
				return e
			}(),
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.ValidateEnvelope(tc.env)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateSchema(t *testing.T) {
	validator := NewValidator()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		eventType   string
		payload     map[string]any
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid delivery.created",
			eventType: "delivery.created",
			payload:   map[string]any{"deliveryId": "delivery-123"},
			expectError: false,
		},
		{
			name:      "valid delivery.driver.assigned",
			eventType: "delivery.driver.assigned",
			payload:   map[string]any{"deliveryId": "delivery-123", "driverId": "driver-456"},
			expectError: false,
		},
		{
			name:      "valid driver.assignment.offered",
			eventType: "driver.assignment.offered",
			payload:   map[string]any{"assignmentId": "assign-123", "driverId": "driver-456"},
			expectError: false,
		},
		{
			name:      "valid payment.created",
			eventType: "payment.created",
			payload:   map[string]any{"paymentId": "payment-123"},
			expectError: false,
		},
		{
			name:      "valid notification.created",
			eventType: "notification.created",
			payload:   map[string]any{"notificationId": "notif-123"},
			expectError: false,
		},
		{
			name:      "missing required field",
			eventType: "delivery.created",
			payload:   map[string]any{"userId": "user-123"},
			expectError: true,
			errorMsg:   "deliveryId",
		},
		{
			name:      "empty required field",
			eventType: "delivery.created",
			payload:   map[string]any{"deliveryId": "   "},
			expectError: true,
			errorMsg:   "deliveryId",
		},
		{
			name:      "nil required field",
			eventType: "delivery.created",
			payload:   map[string]any{"deliveryId": nil},
			expectError: true,
			errorMsg:   "deliveryId",
		},
		{
			name:      "unknown event type",
			eventType: "unknown.event",
			payload:   map[string]any{},
			expectError: true,
			errorMsg:   "unknown event type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := &domain.EventEnvelope{
				EventID:       "evt-123",
				EventType:     tc.eventType,
				EventVersion:  1,
				OccurredAt:    now,
				Producer:      "test",
				AggregateType: "test",
				AggregateID:   "test-123",
				IngestedAt:    now,
			}
			payload := handlers.Payload{Map: tc.payload, Raw: []byte{}}
			err := validator.ValidateSchema(env, payload)
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	assert.NotNil(t, validator)
}