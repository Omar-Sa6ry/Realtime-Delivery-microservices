package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
)

// MessageHandler is the function signature for processing a single Kafka message.
// If it returns a transient error, the consumer will retry.
// If it returns a permanent error (wrapped with ErrPermanent), the message is sent to the DLQ.
type MessageHandler func(ctx context.Context, msg kafkago.Message) error

// ErrPermanent wraps an error to signal that it should NOT be retried and must go to the DLQ.
var ErrPermanent = errors.New("permanent processing failure")

// Consumer wraps a kafka-go Reader with controlled, graceful consumption.
type Consumer struct {
	reader      *kafkago.Reader
	maxRetries  int
	retryDelay  time.Duration
	dlqProducer *Producer // optional: publish failed messages to a DLQ topic
}

// ConsumerConfig holds parameters for creating a Consumer.
type ConsumerConfig struct {
	Brokers    []string
	Topic      string
	GroupID    string
	MaxRetries int
	DLQ        *Producer // may be nil — failed messages are just logged
}

// NewConsumer creates a Kafka consumer bound to a single topic and consumer group.
func NewConsumer(cfg ConsumerConfig) *Consumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,              // 10 MB
		CommitInterval: 0,                 // explicit commit only — after successful processing
		MaxWait:        500 * time.Millisecond,
		StartOffset:    kafkago.FirstOffset,
		RetentionTime:  7 * 24 * time.Hour, // match Kafka topic retention
		Logger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(msg, args...), "component", "kafka-consumer", "topic", cfg.Topic)
		}),
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Error(fmt.Sprintf(msg, args...), "component", "kafka-consumer", "topic", cfg.Topic)
		}),
	})

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &Consumer{
		reader:      reader,
		maxRetries:  maxRetries,
		retryDelay:  2 * time.Second,
		dlqProducer: cfg.DLQ,
	}
}

// Run starts consuming messages from the topic, invoking handler for each message.
// Offset is committed only after the handler returns nil.
// Transient errors trigger exponential backoff retries up to maxRetries.
// Permanent errors (ErrPermanent) and exhausted retries route to the DLQ.
// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	slog.Info("Kafka consumer started", "topic", c.reader.Config().Topic, "group", c.reader.Config().GroupID)
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Info("Kafka consumer stopped", "topic", c.reader.Config().Topic)
				return nil
			}
			slog.Error("Failed to fetch Kafka message", "error", err, "topic", c.reader.Config().Topic)
			continue
		}

		if processErr := c.processWithRetry(ctx, msg, handler); processErr != nil {
			c.routeToDLQ(ctx, msg, processErr)
		}

		// Commit AFTER successful processing — at-least-once delivery guarantee.
		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			slog.Error("Failed to commit Kafka offset", "error", commitErr, "topic", c.reader.Config().Topic)
		}
	}
}

// processWithRetry invokes the handler with exponential backoff on transient errors.
func (c *Consumer) processWithRetry(ctx context.Context, msg kafkago.Message, handler MessageHandler) error {
	var lastErr error
	delay := c.retryDelay

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("Retrying Kafka message processing",
				"attempt", attempt,
				"topic", c.reader.Config().Topic,
				"offset", msg.Offset,
				"delay_ms", delay.Milliseconds(),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay = delay * 2 // exponential backoff
		}

		lastErr = handler(ctx, msg)
		if lastErr == nil {
			return nil
		}

		if errors.Is(lastErr, ErrPermanent) {
			return lastErr // do not retry permanent failures
		}
	}

	return fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

// routeToDLQ sends a failed message to the Dead Letter Queue topic and increments the DLQ metric.
func (c *Consumer) routeToDLQ(ctx context.Context, msg kafkago.Message, reason error) {
	topic := c.reader.Config().Topic
	slog.Error("Routing Kafka message to DLQ",
		"topic", topic,
		"offset", msg.Offset,
		"reason", reason.Error(),
	)

	// Increment DLQ metric for alerting/dashboards
	observability.DeadLetterQueueTotal.WithLabelValues(topic).Inc()

	if c.dlqProducer == nil {
		return
	}
	dlqTopic := topic + ".dlq"
	headers := append(msg.Headers, kafkago.Header{Key: "x-dlq-reason", Value: []byte(reason.Error())})
	dlqMsg := kafkago.Message{
		Topic:   dlqTopic,
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: headers,
	}
	if err := c.dlqProducer.writer.WriteMessages(ctx, dlqMsg); err != nil {
		slog.Error("Failed to write to DLQ", "error", err, "dlq_topic", dlqTopic)
	}
}

// Close gracefully shuts down the consumer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
