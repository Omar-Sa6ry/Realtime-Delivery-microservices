package kafka

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	kafkago "github.com/segmentio/kafka-go"
)

func newTestConsumer(t *testing.T, cfg ConsumerConfig) *Consumer {
	t.Helper()
	observability.RegisterMediaMetrics()
	if len(cfg.Brokers) == 0 {
		cfg.Brokers = []string{deadBroker}
	}
	c := NewConsumer(cfg)
	c.retryDelay = time.Millisecond
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func testMessage(offset int64) kafkago.Message {
	return kafkago.Message{
		Topic:  TopicMediaReady,
		Key:    []byte("key-1"),
		Value:  []byte(`{"mediaId":"m1"}`),
		Offset: offset,
	}
}

func countingHandler(calls *int, err error) MessageHandler {
	return func(ctx context.Context, msg kafkago.Message) error {
		*calls++
		return err
	}
}

func TestNewConsumerDefaults(t *testing.T) {
	observability.RegisterMediaMetrics()
	c := NewConsumer(ConsumerConfig{
		Brokers: []string{deadBroker},
		Topic:   TopicScanStarted,
		GroupID: "media-scanner-test",
	})
	t.Cleanup(func() { _ = c.Close() })

	if c.maxRetries != 3 {
		t.Fatalf("maxRetries = %d, want 3", c.maxRetries)
	}
	if c.retryDelay != 2*time.Second {
		t.Fatalf("retryDelay = %v, want 2s", c.retryDelay)
	}
	if c.dlqProducer != nil {
		t.Fatal("dlqProducer must be nil when not configured")
	}
	if got := c.reader.Config().Topic; got != TopicScanStarted {
		t.Fatalf("reader topic = %q, want %q", got, TopicScanStarted)
	}
}

func TestNewConsumerUsesConfiguredMaxRetries(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{
		Topic:      TopicScanStarted,
		MaxRetries: 7,
		DLQ:        NewProducer([]string{deadBroker}),
	})
	if c.maxRetries != 7 {
		t.Fatalf("maxRetries = %d, want 7", c.maxRetries)
	}
	if c.dlqProducer == nil {
		t.Fatal("dlqProducer must be set when configured")
	}
}

func TestProcessWithRetrySucceedsOnFirstAttempt(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})

	calls := 0
	if err := c.processWithRetry(context.Background(), testMessage(1), countingHandler(&calls, nil)); err != nil {
		t.Fatalf("processWithRetry: %v", err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
}

func TestProcessWithRetrySucceedsAfterRetries(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})

	calls := 0
	handler := func(ctx context.Context, msg kafkago.Message) error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	}

	if err := c.processWithRetry(context.Background(), testMessage(1), handler); err != nil {
		t.Fatalf("processWithRetry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("handler calls = %d, want 3", calls)
	}
}

func TestProcessWithRetryExhaustsRetries(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted, MaxRetries: 2})

	boom := errors.New("boom")
	calls := 0
	err := c.processWithRetry(context.Background(), testMessage(1), countingHandler(&calls, boom))
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "exhausted 2 retries") {
		t.Fatalf("error = %v, want exhaustion message", err)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap the handler error", err)
	}
	if calls != 3 {
		t.Fatalf("handler calls = %d, want 3 (initial + 2 retries)", calls)
	}
}

func TestProcessWithRetryPermanentFailureIsNotRetried(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})

	calls := 0
	wrapped := fmt.Errorf("handler failed: %w", ErrPermanent)
	err := c.processWithRetry(context.Background(), testMessage(1), countingHandler(&calls, wrapped))
	if !errors.Is(err, ErrPermanent) {
		t.Fatalf("error = %v, want ErrPermanent", err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1 (no retries for permanent failures)", calls)
	}
}

func TestProcessWithRetryStopsWhenContextCancelledDuringDelay(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})
	c.retryDelay = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	handler := func(ctx context.Context, msg kafkago.Message) error {
		calls++
		cancel()
		return errors.New("boom")
	}

	start := time.Now()
	err := c.processWithRetry(ctx, testMessage(1), handler)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("processWithRetry took %v, cancellation must win over the retry delay", elapsed)
	}
}

func TestRouteToDLQWithoutProducerReturnsError(t *testing.T) {
	t.Setenv("ALERTMANAGER_URL", "")
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})

	dlq := observability.DeadLetterQueueTotal.WithLabelValues(TopicScanCompleted)
	delta := counterDelta(t, dlq, func() {
		err := c.routeToDLQ(context.Background(), testMessage(7), errors.New("handler exploded"))
		if err == nil {
			t.Fatal("routeToDLQ without a DLQ producer must fail")
		}
		if !strings.Contains(err.Error(), "DLQ producer is not configured") {
			t.Fatalf("error = %v, want missing producer message", err)
		}
	})
	if delta != 1 {
		t.Fatalf("DeadLetterQueueTotal delta = %v, want 1", delta)
	}
}

func TestRouteToDLQProducerWriteFailure(t *testing.T) {
	t.Setenv("ALERTMANAGER_URL", "")
	c := newTestConsumer(t, ConsumerConfig{
		Topic: TopicScanCompleted,
		DLQ:   NewProducer([]string{deadBroker}),
	})

	err := c.routeToDLQ(boundedCtx(t), testMessage(7), errors.New("handler exploded"))
	if err == nil {
		t.Fatal("routeToDLQ must fail when the DLQ write fails")
	}
	if !strings.Contains(err.Error(), `write Kafka DLQ "media.scan.completed.dlq"`) {
		t.Fatalf("error = %v, want DLQ topic context", err)
	}
}

func TestRouteToDLQPublishesSuccessfully(t *testing.T) {
	t.Setenv("ALERTMANAGER_URL", "")
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})
	c.dlqProducer = newFakeTransportProducer()
	t.Cleanup(func() { _ = c.dlqProducer.Close() })

	err := c.routeToDLQ(context.Background(), testMessage(7), errors.New("handler exploded"))
	if err != nil {
		t.Fatalf("routeToDLQ: %v", err)
	}
}

func TestConsumerErrorLoggerClassifiesTransientErrors(t *testing.T) {
	observability.RegisterMediaMetrics()
	c := NewConsumer(ConsumerConfig{Brokers: []string{deadBroker}, Topic: TopicScanCompleted})
	t.Cleanup(func() { _ = c.Close() })

	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	logger := c.reader.Config().ErrorLogger
	logger.Printf("error: Rebalance In Progress received")
	logger.Printf("error: Not Leader For Partition received")
	logger.Printf("error: something else")

	out := buf.String()
	if !strings.Contains(out, "Rebalance In Progress") {
		t.Fatalf("slog output = %q, want transient error logged", out)
	}
	if !strings.Contains(out, "level=DEBUG") {
		t.Fatalf("slog output = %q, want DEBUG level for transient errors", out)
	}
	if !strings.Contains(out, "something else") {
		t.Fatalf("slog output = %q, want non-transient error logged", out)
	}
	if strings.Count(out, "level=ERROR") != 1 {
		t.Fatalf("slog output = %q, want exactly one ERROR entry", out)
	}
}

func TestMinDuration(t *testing.T) {
	if got := minDuration(time.Second, 2*time.Second); got != time.Second {
		t.Fatalf("minDuration(1s, 2s) = %v, want 1s", got)
	}
	if got := minDuration(3*time.Minute, 30*time.Second); got != 30*time.Second {
		t.Fatalf("minDuration(3m, 30s) = %v, want 30s", got)
	}
}

func TestRunReturnsNilWhenContextCancelled(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})

	savedLag := observability.KafkaConsumerLag
	observability.KafkaConsumerLag = nil
	t.Cleanup(func() { observability.KafkaConsumerLag = savedLag })

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)

	err := c.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
		t.Error("handler must not be invoked when no message can be fetched")
		return nil
	})
	if err != nil {
		t.Fatalf("Run = %v, want nil on cancellation", err)
	}
}

func TestRunLogsFetchErrorsUntilCancelled(t *testing.T) {
	observability.RegisterMediaMetrics()
	c := newTestConsumer(t, ConsumerConfig{Topic: TopicScanCompleted})

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(2200*time.Millisecond, cancel)

	start := time.Now()
	err := c.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
		t.Error("handler must not be invoked when no message can be fetched")
		return nil
	})
	if err != nil {
		t.Fatalf("Run = %v, want nil on cancellation", err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("Run returned after %v, fetch errors should surface before cancellation", elapsed)
	}
}

func TestClose(t *testing.T) {
	observability.RegisterMediaMetrics()
	c := NewConsumer(ConsumerConfig{Brokers: []string{deadBroker}, Topic: TopicScanCompleted})
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
