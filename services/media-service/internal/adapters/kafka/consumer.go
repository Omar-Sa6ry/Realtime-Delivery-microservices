package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	kafkago "github.com/segmentio/kafka-go"
)

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
		MaxBytes:       10e6, // 10 MB
		CommitInterval: 0,    // explicit commit only — after successful processing
		MaxWait:        500 * time.Millisecond,
		StartOffset:    kafkago.FirstOffset,
		RetentionTime:  7 * 24 * time.Hour, // match Kafka topic retention
		Logger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(msg, args...), "component", "kafka-consumer", "topic", cfg.Topic)
		}),
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			formatted := fmt.Sprintf(msg, args...)
			if strings.Contains(formatted, "Rebalance In Progress") ||
				strings.Contains(formatted, "i/o timeout") ||
				strings.Contains(formatted, "Not Coordinator For Group") ||
				strings.Contains(formatted, "Group Coordinator Not Available") ||
				strings.Contains(formatted, "Not Leader For Partition") {
				slog.Debug(formatted, "component", "kafka-consumer", "topic", cfg.Topic)
			} else {
				slog.Error(formatted, "component", "kafka-consumer", "topic", cfg.Topic)
			}
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

func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	slog.Info("Kafka consumer started", "topic", c.reader.Config().Topic, "group", c.reader.Config().GroupID)
	for {
		msg, err := c.reader.FetchMessage(ctx)
		stats := c.reader.Stats()
		if observability.KafkaConsumerLag != nil {
			observability.KafkaConsumerLag.WithLabelValues(c.reader.Config().Topic, c.reader.Config().GroupID).Set(float64(stats.Lag))
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Info("Kafka consumer stopped", "topic", c.reader.Config().Topic)
				return nil
			}
			if strings.Contains(err.Error(), "Rebalance In Progress") ||
				strings.Contains(err.Error(), "Not Coordinator For Group") ||
				strings.Contains(err.Error(), "Group Coordinator Not Available") ||
				strings.Contains(err.Error(), "Not Leader For Partition") {
				slog.Debug("Kafka consumer transient coordinator/rebalance/leader state, waiting...", "topic", c.reader.Config().Topic)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(500 * time.Millisecond):
				}
				continue
			}
			slog.Error("Failed to fetch Kafka message", "error", err, "topic", c.reader.Config().Topic)
			continue
		}

		if processErr := c.processWithRetry(ctx, msg, handler); processErr != nil {
			if dlqErr := c.routeToDLQ(ctx, msg, processErr); dlqErr != nil {
				// Do not acknowledge a message if its DLQ handoff failed. The
				// broker will redeliver it after the consumer restarts/rebalances.
				slog.Error("DLQ handoff failed; leaving message uncommitted", "error", dlqErr, "topic", c.reader.Config().Topic, "offset", msg.Offset)
				continue
			}
		}

		// Commit after successful processing or a confirmed DLQ handoff — this
		// preserves at-least-once delivery without silently dropping failures.
		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			slog.Error("Failed to commit Kafka offset", "error", commitErr, "topic", c.reader.Config().Topic)
		}
	}
}

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
			jitter := time.Duration(rand.Int63n(int64(delay/2) + 1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay + jitter):
			}
			delay = minDuration(delay*2, 30*time.Second) // capped exponential backoff
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

func (c *Consumer) routeToDLQ(ctx context.Context, msg kafkago.Message, reason error) error {
	topic := c.reader.Config().Topic
	slog.Error("Routing Kafka message to DLQ",
		"topic", topic,
		"offset", msg.Offset,
		"reason", reason.Error(),
	)

	observability.DeadLetterQueueTotal.WithLabelValues(topic).Inc()

	if c.dlqProducer == nil {
		return fmt.Errorf("DLQ producer is not configured for topic %q", topic)
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
		return fmt.Errorf("write Kafka DLQ %q: %w", dlqTopic, err)
	}
	return nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// Close gracefully shuts down the consumer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
