package kafka_test

import (
	"context"
	"testing"
	"time"

	adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/kafka"
	"github.com/segmentio/kafka-go"
)

// TestKafkaPublisher_PublishRoundTrip publishes a message with the adapter and
// reads it back from the live broker, asserting key, value and headers.
func TestKafkaPublisher_PublishRoundTrip(t *testing.T) {
	probeKafka(t)

	topic := uniqueName(t)
	ensureTopics(t, topic)
	t.Cleanup(func() { deleteTopics(t, topic) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pub := adapter.NewKafkaPublisher([]string{kafkaBroker}, topic)
	t.Cleanup(func() { _ = pub.Close() })

	value := []byte("payload-" + topic)
	// The topic was created moments ago; brokers can still answer a first
	// produce with UNKNOWN_TOPIC_OR_PARTITION while leadership settles, so
	// retry briefly instead of failing the run on that race.
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		if err = pub.PublishMessage(ctx, topic, "key-1", value); err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("PublishMessage: %v", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{kafkaBroker},
		Topic:     topic,
		MinBytes:  1,
		MaxBytes:  10e6,
		MaxWait:   100 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	defer func() { _ = reader.Close() }()

	msg, err := reader.FetchMessage(ctx)
	if err != nil {
		t.Fatalf("FetchMessage: %v", err)
	}

	if string(msg.Key) != "key-1" {
		t.Errorf("key = %q, want %q", string(msg.Key), "key-1")
	}
	if string(msg.Value) != string(value) {
		t.Errorf("value = %q, want %q", string(msg.Value), string(value))
	}

	headers := make(map[string]string, len(msg.Headers))
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}
	if headers["x-source"] != "driver-service" {
		t.Errorf("header x-source = %q, want %q", headers["x-source"], "driver-service")
	}
	if headers["x-timestamp"] == "" {
		t.Errorf("header x-timestamp is empty")
	}
	if msg.Time.IsZero() {
		t.Errorf("message timestamp is zero")
	}
}

// TestKafkaPublisher_PublishFailures covers the failure branches: an invalid
// topic name rejected by the broker and writes on a closed writer.
func TestKafkaPublisher_PublishFailures(t *testing.T) {
	probeKafka(t)

	topic := uniqueName(t)
	ensureTopics(t, topic)
	t.Cleanup(func() { deleteTopics(t, topic) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("invalid topic name", func(t *testing.T) {
		pub := adapter.NewKafkaPublisher([]string{kafkaBroker}, topic)
		defer func() { _ = pub.Close() }()

		bctx, bcancel := context.WithTimeout(ctx, 5*time.Second)
		defer bcancel()
		err := pub.PublishMessage(bctx, "bad topic with spaces", "k", []byte("v"))
		if err == nil {
			t.Errorf("PublishMessage to invalid topic name: got nil error, want error")
		}
	})

	t.Run("closed writer", func(t *testing.T) {
		pub := adapter.NewKafkaPublisher([]string{kafkaBroker}, topic)
		if err := pub.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if err := pub.PublishMessage(ctx, topic, "k", []byte("v")); err == nil {
			t.Errorf("PublishMessage after Close: got nil error, want error")
		}
	})
}
