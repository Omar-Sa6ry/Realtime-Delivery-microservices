package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	kafkago "github.com/segmentio/kafka-go"
)

// EventEnvelope is the canonical envelope for every Kafka event in the platform.
// It matches the Go events.EventEnvelope and the TypeScript KafkaEventEnvelope.
type EventEnvelope = events.EventEnvelope

// MarshalEnvelope serialises a payload into a standard Kafka event envelope.
func MarshalEnvelope(eventID, eventType, traceID string, payload interface{}) ([]byte, error) {
	return events.MarshalEnvelope(eventID, events.MediaEventType(eventType), traceID, payload)
}

// UnmarshalEnvelope parses a raw Kafka message into an EventEnvelope.
func UnmarshalEnvelope(data []byte) (*EventEnvelope, error) {
	return events.UnmarshalEnvelope(data)
}

// Producer wraps a kafka-go Writer with structured event publishing.
type Producer struct {
	writer *kafkago.Writer
}

// NewProducer creates a new Kafka producer connected to the given brokers.
func NewProducer(brokers []string) *Producer {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll,
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Compression:  kafkago.Snappy,
		Logger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(msg, args...), "component", "kafka-producer")
		}),
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Error(fmt.Sprintf(msg, args...), "component", "kafka-producer")
		}),
	}
	return &Producer{writer: writer}
}

// Publish sends a raw JSON payload to the given topic.
// The topic should come from the shared topic constants (events / nats packages).
// key is used for partitioning, traceID is propagated as a Kafka message header.
func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte, traceID string) error {
	msg := kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Headers: []kafkago.Header{
			{Key: "x-trace-id", Value: []byte(traceID)},
			{Key: "x-timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
		},
		Time: time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish to topic %q: %w", topic, err)
	}
	return nil
}

// PublishEnvelope wraps the payload in the standard EventEnvelope and publishes it.
func (p *Producer) PublishEnvelope(ctx context.Context, topic, key, eventType, traceID string, payload interface{}) error {
	data, err := MarshalEnvelope("", eventType, traceID, payload)
	if err != nil {
		return fmt.Errorf("kafka envelope marshal: %w", err)
	}
	return p.Publish(ctx, topic, key, data, traceID)
}

// Close flushes pending messages and closes the underlying writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// MessageHandler is the signature for processing a single Kafka message.
// Return ErrPermanent to send the message to the DLQ without retrying.
type MessageHandler func(ctx context.Context, msg kafkago.Message) error

// ErrPermanent wraps an error to signal that it should NOT be retried and must go to the DLQ.
var ErrPermanent = errors.New("permanent processing failure")

// ConsumerConfig holds parameters for creating a Consumer.
type ConsumerConfig struct {
	Brokers    []string
	Topic      string
	GroupID    string
	MaxRetries int
	DLQ        *Producer // may be nil — failed messages are just logged
}

// Consumer wraps a kafka-go Reader with controlled, graceful consumption
// (explicit offset commit + exponential backoff on transient errors + DLQ routing).
type Consumer struct {
	reader      *kafkago.Reader
	maxRetries  int
	retryDelay  time.Duration
	dlqProducer *Producer
}

// NewConsumer creates a Kafka consumer bound to a single topic and consumer group.
func NewConsumer(cfg ConsumerConfig) *Consumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
		MaxWait:        500 * time.Millisecond,
		StartOffset:    kafkago.FirstOffset,
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

// Run starts consuming messages, invoking handler for each message.
// Offsets are committed only after the handler returns nil (at-least-once).
// Transient errors retry with exponential backoff; exhausted retries and
// permanent errors are routed to the DLQ. Run blocks until ctx is cancelled.
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}

		lastErr = handler(ctx, msg)
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, ErrPermanent) {
			return lastErr
		}
	}

	return fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

// routeToDLQ sends a failed message to the topic + ".dlq".
func (c *Consumer) routeToDLQ(ctx context.Context, msg kafkago.Message, reason error) {
	topic := c.reader.Config().Topic
	slog.Error("Routing Kafka message to DLQ",
		"topic", topic,
		"offset", msg.Offset,
		"reason", reason.Error(),
	)

	if c.dlqProducer == nil {
		return
	}

	headers := append(msg.Headers, kafkago.Header{Key: "x-dlq-reason", Value: []byte(reason.Error())})
	dlqMsg := kafkago.Message{
		Topic:   topic + ".dlq",
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: headers,
	}
	if err := c.dlqProducer.writer.WriteMessages(ctx, dlqMsg); err != nil {
		slog.Error("Failed to write to DLQ", "error", err, "dlq_topic", topic+".dlq")
	}
}

// Close gracefully shuts down the consumer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
