package kafka_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	"github.com/segmentio/kafka-go"
)

// fetchEnvelope reads from topic until a message containing marker appears and
// decodes it into an envelope.
func fetchEnvelope(ctx context.Context, t *testing.T, topic, marker string) (*events.EventEnvelope, kafka.Message) {
	t.Helper()
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{kafkaBroker},
		Topic:       topic,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     100 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	defer func() { _ = reader.Close() }()

	lctx, lcancel := context.WithTimeout(ctx, 20*time.Second)
	defer lcancel()
	for {
		msg, err := reader.FetchMessage(lctx)
		if err != nil {
			t.Fatalf("FetchMessage from %q while looking for marker %q: %v", topic, marker, err)
		}
		if !bytes.Contains(msg.Value, []byte(marker)) {
			continue
		}
		env, err := events.UnmarshalEnvelope(msg.Value)
		if err != nil {
			t.Fatalf("UnmarshalEnvelope: %v", err)
		}
		return env, msg
	}
}

func decodePayload(t *testing.T, env *events.EventEnvelope, out interface{}) {
	t.Helper()
	if err := json.Unmarshal(env.Payload, out); err != nil {
		t.Fatalf("decode payload %s: %v", string(env.Payload), err)
	}
}

// TestEventPublisherAdapter_PublishRoundTrips calls every Publish* method on
// the adapter, consumes the produced message from the live broker and asserts
// the envelope decodes back to the expected event type, key and payload fields.
func TestEventPublisherAdapter_PublishRoundTrips(t *testing.T) {
	probeKafka(t)

	topics := []string{
		"driver.created",
		"driver.updated",
		"driver.deleted",
		"driver.assignment.offered",
		"driver.assignment.accepted",
		"driver.assignment.rejected",
		"driver.assignment.expired",
		"driver.assignment.released",
		"driver.available",
		"driver.unavailable",
		"driver.location.updated",
		"driver.no_driver_available",
		"driver.search.retry",
	}
	ensureTopics(t, topics...)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	writer := adapter.NewKafkaPublisher([]string{kafkaBroker}, topics[0])
	t.Cleanup(func() { _ = writer.Close() })
	pub := adapter.NewEventPublisherAdapter(writer)

	runID := uniqueName(t)
	driverID := "drv-" + runID
	userID := "user-" + runID
	assignmentID := "asg-" + runID
	deliveryID := "del-" + runID
	customerID := "cust-" + runID

	tests := []struct {
		name   string
		topic  string
		key    string
		call   func(ctx context.Context) error
		verify func(t *testing.T, env *events.EventEnvelope)
	}{
		{
			name:  "driver created",
			topic: "driver.created",
			key:   driverID,
			call:  func(ctx context.Context) error { return pub.PublishDriverCreated(ctx, driverID, userID) },
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverCreatedPayload
				decodePayload(t, env, &p)
				if p.DriverID != driverID {
					t.Errorf("DriverID = %q, want %q", p.DriverID, driverID)
				}
				if p.Name != userID {
					t.Errorf("Name = %q, want %q", p.Name, userID)
				}
				if p.Status != "AVAILABLE" {
					t.Errorf("Status = %q, want AVAILABLE", p.Status)
				}
				if p.VehicleType != "CAR" {
					t.Errorf("VehicleType = %q, want CAR", p.VehicleType)
				}
				if p.Rating != 5.0 {
					t.Errorf("Rating = %v, want 5.0", p.Rating)
				}
				if p.SourceVersion != 1 {
					t.Errorf("SourceVersion = %d, want 1", p.SourceVersion)
				}
				if p.UpdatedAt.IsZero() {
					t.Errorf("UpdatedAt is zero")
				}
			},
		},
		{
			name:  "driver updated",
			topic: "driver.updated",
			key:   driverID,
			call: func(ctx context.Context) error {
				return pub.PublishDriverUpdated(ctx, driverID, "MOTORCYCLE", []string{"cargo"})
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverUpdatedPayload
				decodePayload(t, env, &p)
				if p.DriverID != driverID {
					t.Errorf("DriverID = %q, want %q", p.DriverID, driverID)
				}
				if p.VehicleType != "MOTORCYCLE" {
					t.Errorf("VehicleType = %q, want MOTORCYCLE", p.VehicleType)
				}
				if p.SourceVersion != 1 {
					t.Errorf("SourceVersion = %d, want 1", p.SourceVersion)
				}
				if p.UpdatedAt.IsZero() {
					t.Errorf("UpdatedAt is zero")
				}
			},
		},
		{
			name:  "driver deleted",
			topic: "driver.deleted",
			key:   driverID,
			call:  func(ctx context.Context) error { return pub.PublishDriverDeleted(ctx, driverID) },
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverDeletedPayload
				decodePayload(t, env, &p)
				if p.DriverID != driverID {
					t.Errorf("DriverID = %q, want %q", p.DriverID, driverID)
				}
				if p.DeletedAt.IsZero() {
					t.Errorf("DeletedAt is zero")
				}
			},
		},
		{
			name:  "assignment offered",
			topic: "driver.assignment.offered",
			key:   assignmentID,
			call: func(ctx context.Context) error {
				return pub.PublishAssignmentOffered(ctx, assignmentID, deliveryID, driverID)
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverAssignmentOfferedPayload
				decodePayload(t, env, &p)
				if p.AssignmentID != assignmentID {
					t.Errorf("AssignmentID = %q, want %q", p.AssignmentID, assignmentID)
				}
				if p.DeliveryID != deliveryID {
					t.Errorf("DeliveryID = %q, want %q", p.DeliveryID, deliveryID)
				}
				if p.DriverID != driverID {
					t.Errorf("DriverID = %q, want %q", p.DriverID, driverID)
				}
				if _, err := time.Parse(time.RFC3339, p.ExpiresAt); err != nil {
					t.Errorf("ExpiresAt %q is not RFC3339: %v", p.ExpiresAt, err)
				}
			},
		},
		{
			name:  "assignment accepted",
			topic: "driver.assignment.accepted",
			key:   assignmentID,
			call: func(ctx context.Context) error {
				return pub.PublishAssignmentAccepted(ctx, assignmentID, deliveryID, driverID)
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverAssignmentAcceptedPayload
				decodePayload(t, env, &p)
				if p.AssignmentID != assignmentID || p.DeliveryID != deliveryID || p.DriverID != driverID {
					t.Errorf("payload ids = (%q,%q,%q), want (%q,%q,%q)",
						p.AssignmentID, p.DeliveryID, p.DriverID, assignmentID, deliveryID, driverID)
				}
				if _, err := time.Parse(time.RFC3339, p.AcceptedAt); err != nil {
					t.Errorf("AcceptedAt %q is not RFC3339: %v", p.AcceptedAt, err)
				}
			},
		},
		{
			name:  "assignment rejected",
			topic: "driver.assignment.rejected",
			key:   assignmentID,
			call: func(ctx context.Context) error {
				return pub.PublishAssignmentRejected(ctx, assignmentID, deliveryID, driverID, "too far away")
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverAssignmentRejectedPayload
				decodePayload(t, env, &p)
				if p.AssignmentID != assignmentID || p.DeliveryID != deliveryID || p.DriverID != driverID {
					t.Errorf("payload ids = (%q,%q,%q), want (%q,%q,%q)",
						p.AssignmentID, p.DeliveryID, p.DriverID, assignmentID, deliveryID, driverID)
				}
				if p.Reason != "too far away" {
					t.Errorf("Reason = %q, want %q", p.Reason, "too far away")
				}
			},
		},
		{
			name:  "assignment expired",
			topic: "driver.assignment.expired",
			key:   assignmentID,
			call: func(ctx context.Context) error {
				return pub.PublishAssignmentExpired(ctx, assignmentID, deliveryID)
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverAssignmentExpiredPayload
				decodePayload(t, env, &p)
				if p.AssignmentID != assignmentID || p.DeliveryID != deliveryID {
					t.Errorf("payload ids = (%q,%q), want (%q,%q)", p.AssignmentID, p.DeliveryID, assignmentID, deliveryID)
				}
				if _, err := time.Parse(time.RFC3339, p.ExpiredAt); err != nil {
					t.Errorf("ExpiredAt %q is not RFC3339: %v", p.ExpiredAt, err)
				}
			},
		},
		{
			name:  "assignment released",
			topic: "driver.assignment.released",
			key:   assignmentID,
			call: func(ctx context.Context) error {
				return pub.PublishAssignmentReleased(ctx, assignmentID)
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverAssignmentReleasedPayload
				decodePayload(t, env, &p)
				if p.AssignmentID != assignmentID {
					t.Errorf("AssignmentID = %q, want %q", p.AssignmentID, assignmentID)
				}
				if _, err := time.Parse(time.RFC3339, p.ReleasedAt); err != nil {
					t.Errorf("ReleasedAt %q is not RFC3339: %v", p.ReleasedAt, err)
				}
			},
		},
		{
			name:  "driver available",
			topic: "driver.available",
			key:   driverID,
			call:  func(ctx context.Context) error { return pub.PublishDriverAvailable(ctx, driverID) },
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverAvailablePayload
				decodePayload(t, env, &p)
				if p.DriverID != driverID {
					t.Errorf("DriverID = %q, want %q", p.DriverID, driverID)
				}
			},
		},
		{
			name:  "driver unavailable",
			topic: "driver.unavailable",
			key:   driverID,
			call:  func(ctx context.Context) error { return pub.PublishDriverUnavailable(ctx, driverID) },
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverUnavailablePayload
				decodePayload(t, env, &p)
				if p.DriverID != driverID {
					t.Errorf("DriverID = %q, want %q", p.DriverID, driverID)
				}
			},
		},
		{
			name:  "location updated",
			topic: "driver.location.updated",
			key:   driverID,
			call: func(ctx context.Context) error {
				return pub.PublishLocationUpdated(ctx, driverID, "30.0444", "31.2357")
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p struct {
					DriverID  string `json:"driverId"`
					Latitude  string `json:"latitude"`
					Longitude string `json:"longitude"`
				}
				decodePayload(t, env, &p)
				if p.DriverID != driverID {
					t.Errorf("driverId = %q, want %q", p.DriverID, driverID)
				}
				if p.Latitude != "30.0444" || p.Longitude != "31.2357" {
					t.Errorf("coordinates = (%q,%q), want (30.0444,31.2357)", p.Latitude, p.Longitude)
				}
			},
		},
		{
			name:  "no driver available",
			topic: "driver.no_driver_available",
			key:   deliveryID,
			call: func(ctx context.Context) error {
				return pub.PublishNoDriverAvailable(ctx, deliveryID, customerID, 3, "NO_DRIVERS_NEARBY")
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverNoDriverAvailablePayload
				decodePayload(t, env, &p)
				if p.DeliveryID != deliveryID || p.CustomerID != customerID {
					t.Errorf("ids = (%q,%q), want (%q,%q)", p.DeliveryID, p.CustomerID, deliveryID, customerID)
				}
				if p.AttemptNumber != 3 {
					t.Errorf("AttemptNumber = %d, want 3", p.AttemptNumber)
				}
				if p.Reason != "NO_DRIVERS_NEARBY" {
					t.Errorf("Reason = %q, want NO_DRIVERS_NEARBY", p.Reason)
				}
				if _, err := time.Parse(time.RFC3339, p.TriedAt); err != nil {
					t.Errorf("TriedAt %q is not RFC3339: %v", p.TriedAt, err)
				}
			},
		},
		{
			name:  "search retry",
			topic: "driver.search.retry",
			key:   deliveryID,
			call: func(ctx context.Context) error {
				return pub.PublishSearchRetry(ctx, deliveryID, customerID, 1, 45)
			},
			verify: func(t *testing.T, env *events.EventEnvelope) {
				var p events.DriverSearchRetryPayload
				decodePayload(t, env, &p)
				if p.DeliveryID != deliveryID || p.CustomerID != customerID {
					t.Errorf("ids = (%q,%q), want (%q,%q)", p.DeliveryID, p.CustomerID, deliveryID, customerID)
				}
				if p.AttemptNumber != 1 {
					t.Errorf("AttemptNumber = %d, want 1", p.AttemptNumber)
				}
				if p.RetryInterval != 45 {
					t.Errorf("RetryInterval = %d, want 45", p.RetryInterval)
				}
				if _, err := time.Parse(time.RFC3339, p.NextRetryAt); err != nil {
					t.Errorf("NextRetryAt %q is not RFC3339: %v", p.NextRetryAt, err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(ctx); err != nil {
				t.Fatalf("publish: %v", err)
			}
			env, msg := fetchEnvelope(ctx, t, tt.topic, runID)
			if env.EventType != tt.topic {
				t.Errorf("EventType = %q, want %q", env.EventType, tt.topic)
			}
			if env.EventID == "" {
				t.Errorf("EventID is empty")
			}
			if env.EventVersion != 1 {
				t.Errorf("EventVersion = %d, want 1", env.EventVersion)
			}
			if string(msg.Key) != tt.key {
				t.Errorf("key = %q, want %q", string(msg.Key), tt.key)
			}
			headers := make(map[string]string, len(msg.Headers))
			for _, h := range msg.Headers {
				headers[h.Key] = string(h.Value)
			}
			if headers["x-source"] != "driver-service" {
				t.Errorf("header x-source = %q, want driver-service", headers["x-source"])
			}
			if headers["x-timestamp"] == "" {
				t.Errorf("header x-timestamp is empty")
			}
			tt.verify(t, env)
		})
	}
}

// TestEventPublisherAdapter_PublishFailures exercises every Publish* method
// against a closed writer and the envelope marshal-failure branch.
func TestEventPublisherAdapter_PublishFailures(t *testing.T) {
	probeKafka(t)

	topic := uniqueName(t)
	ensureTopics(t, topic)
	t.Cleanup(func() { deleteTopics(t, topic) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	writer := adapter.NewKafkaPublisher([]string{kafkaBroker}, topic)
	pub := adapter.NewEventPublisherAdapter(writer)

	// Marshal failure branch: a channel cannot be marshalled to JSON.
	if err := pub.PublishEnvelope(ctx, "test.invalid", "key", make(chan int)); err == nil {
		t.Errorf("PublishEnvelope with chan payload: got nil error, want marshal error")
	}

	if err := writer.Close(); err != nil {
		t.Logf("writer Close: %v", err)
	}

	failures := []struct {
		name    string
		publish func() error
	}{
		{"driver created", func() error { return pub.PublishDriverCreated(ctx, "d", "u") }},
		{"driver updated", func() error { return pub.PublishDriverUpdated(ctx, "d", "CAR", nil) }},
		{"driver deleted", func() error { return pub.PublishDriverDeleted(ctx, "d") }},
		{"assignment offered", func() error { return pub.PublishAssignmentOffered(ctx, "a", "d", "x") }},
		{"assignment accepted", func() error { return pub.PublishAssignmentAccepted(ctx, "a", "d", "x") }},
		{"assignment rejected", func() error { return pub.PublishAssignmentRejected(ctx, "a", "d", "x", "r") }},
		{"assignment expired", func() error { return pub.PublishAssignmentExpired(ctx, "a", "d") }},
		{"assignment released", func() error { return pub.PublishAssignmentReleased(ctx, "a") }},
		{"driver available", func() error { return pub.PublishDriverAvailable(ctx, "d") }},
		{"driver unavailable", func() error { return pub.PublishDriverUnavailable(ctx, "d") }},
		{"location updated", func() error { return pub.PublishLocationUpdated(ctx, "d", "1", "2") }},
		{"no driver available", func() error { return pub.PublishNoDriverAvailable(ctx, "d", "c", 1, "r") }},
		{"search retry", func() error { return pub.PublishSearchRetry(ctx, "d", "c", 1, 30) }},
	}
	for _, tt := range failures {
		t.Run(tt.name+" on closed writer", func(t *testing.T) {
			if err := tt.publish(); err == nil {
				t.Errorf("expected error publishing on a closed writer, got nil")
			}
		})
	}
}
