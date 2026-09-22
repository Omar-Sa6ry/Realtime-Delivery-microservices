package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventEnvelope_UnmarshalJSON(t *testing.T) {
	// 1. Test unmarshalling with ISO string occurredAt
	isoJSON := []byte(`{
		"eventId": "test-123",
		"eventType": "delivery.created",
		"eventVersion": 1,
		"occurredAt": "2026-09-22T14:44:00Z",
		"timestamp": 1790088240000,
		"producer": "delivery-service",
		"payload": {"hello":"world"}
	}`)

	var env1 EventEnvelope
	if err := json.Unmarshal(isoJSON, &env1); err != nil {
		t.Fatalf("expected no error unmarshalling ISO string occurredAt, got: %v", err)
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2026-09-22T14:44:00Z")
	if env1.OccurredAt != expectedTime.UnixMilli() {
		t.Errorf("expected OccurredAt %d, got %d", expectedTime.UnixMilli(), env1.OccurredAt)
	}
	if env1.Timestamp != 1790088240000 {
		t.Errorf("expected Timestamp 1790088240000, got %d", env1.Timestamp)
	}

	// 2. Test unmarshalling with numeric occurredAt
	numJSON := []byte(`{
		"eventId": "test-456",
		"eventType": "delivery.created",
		"eventVersion": 1,
		"occurredAt": 1790088240000,
		"timestamp": "2026-09-22T14:44:00Z",
		"producer": "delivery-service",
		"payload": {"hello":"world"}
	}`)

	var env2 EventEnvelope
	if err := json.Unmarshal(numJSON, &env2); err != nil {
		t.Fatalf("expected no error unmarshalling numeric occurredAt, got: %v", err)
	}
	if env2.OccurredAt != 1790088240000 {
		t.Errorf("expected OccurredAt 1790088240000, got %d", env2.OccurredAt)
	}
	if env2.Timestamp != expectedTime.UnixMilli() {
		t.Errorf("expected Timestamp %d, got %d", expectedTime.UnixMilli(), env2.Timestamp)
	}
}
