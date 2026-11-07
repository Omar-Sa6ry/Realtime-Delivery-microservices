package kafka_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/kafka"
)

// TestKafkaConsumer_ConsumeAndCommit publishes a payload larger than the
// reader's MinBytes (100 KiB) so ConsumeMessage returns promptly, then commits.
func TestKafkaConsumer_ConsumeAndCommit(t *testing.T) {
	probeKafka(t)

	topic := uniqueName(t)
	group := uniqueName(t)
	ensureTopics(t, topic)
	t.Cleanup(func() { deleteTopics(t, topic) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 130 KiB payload exceeds the consumer's MinBytes of 1e5 so the fetch does
	// not have to wait for MaxWait to elapse.
	value := bytes.Repeat([]byte("kafka-consumer-test."), 7000) // ~133 KiB
	pub := adapter.NewKafkaPublisher([]string{kafkaBroker}, topic)
	if err := pub.PublishMessage(ctx, topic, "consumer-key", value); err != nil {
		t.Fatalf("PublishMessage: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Logf("publisher Close: %v", err)
	}

	consumer := adapter.NewKafkaConsumer([]string{kafkaBroker}, topic, group)

	msg, err := consumer.ConsumeMessage(ctx)
	if err != nil {
		t.Fatalf("ConsumeMessage: %v", err)
	}
	if string(msg.Key) != "consumer-key" {
		t.Errorf("key = %q, want %q", string(msg.Key), "consumer-key")
	}
	if !bytes.Equal(msg.Value, value) {
		t.Errorf("value length = %d, want %d", len(msg.Value), len(value))
	}
	if msg.Topic != topic {
		t.Errorf("topic = %q, want %q", msg.Topic, topic)
	}

	if err := consumer.CommitMessage(ctx, msg); err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
}

// TestKafkaConsumer_ContextCanceled covers the FetchMessage error branch.
func TestKafkaConsumer_ContextCanceled(t *testing.T) {
	probeKafka(t)

	topic := uniqueName(t)
	group := uniqueName(t)
	ensureTopics(t, topic)
	t.Cleanup(func() { deleteTopics(t, topic) })

	consumer := adapter.NewKafkaConsumer([]string{kafkaBroker}, topic, group)

	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()

	// Run with a hard deadline guard in case the reader is slow to observe the
	// canceled context while joining the group.
	resCh := make(chan error, 1)
	go func() {
		_, err := consumer.ConsumeMessage(cctx)
		resCh <- err
	}()

	select {
	case err := <-resCh:
		if err == nil {
			t.Errorf("ConsumeMessage with canceled context: got nil error, want error")
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("ConsumeMessage did not return with a canceled context within 15s")
	}
}
