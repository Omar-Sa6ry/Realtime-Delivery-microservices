package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafkago.Writer
}

// NewProducer creates a new Kafka producer.
func NewProducer(brokers []string) *Producer {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll, // durability: all ISR must ack
		Async:        false,              // synchronous — outbox publisher handles retries
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Compression:  kafkago.Snappy,
		Logger:       kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(msg, args...), "component", "kafka-producer")
		}),
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Error(fmt.Sprintf(msg, args...), "component", "kafka-producer")
		}),
	}
	return &Producer{writer: writer}
}

// EventEnvelope is the standard schema for all Kafka messages published by the media service.
// Consumers depend on this structure; do not change field names without a versioning strategy.
type EventEnvelope struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	TraceID   string          `json:"traceId,omitempty"`
	Timestamp int64           `json:"timestamp"` // unix milli
	Payload   json.RawMessage `json:"payload"`
}

func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte, traceID string) error {
	msg := kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Headers: []kafkago.Header{
			{Key: "x-trace-id", Value: []byte(traceID)},
			{Key: "x-source", Value: []byte("media-service")},
			{Key: "x-timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
		},
		Time: time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish to topic %q: %w", topic, err)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func MarshalEnvelope(eventID, eventType, traceID string, payload interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	env := EventEnvelope{
		EventID:   eventID,
		EventType: eventType,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payloadBytes,
	}
	return json.Marshal(env)
}
