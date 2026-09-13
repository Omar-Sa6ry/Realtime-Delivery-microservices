package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/realtime-delivery/payment-service/internal/ports"
	"github.com/segmentio/kafka-go"
)

// EventPublisher implements the ports.EventPublisher interface using Kafka.
type EventPublisher struct {
	writer  *kafka.Writer
	topic   string
	timeout time.Duration
}

// NewEventPublisher creates a new EventPublisher.
func NewEventPublisher(brokers []string, topic string) *EventPublisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}

	return &EventPublisher{
		writer:  writer,
		topic:   topic,
		timeout: 10 * time.Second,
	}
}

// PublishEvent publishes an event to Kafka.
func (p *EventPublisher) PublishEvent(ctx context.Context, topic, key, eventType string, payload []byte) error {
	// Add event type to headers
	headers := []kafka.Header{
		{Key: "event_type", Value: []byte(eventType)},
		{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
	}

	msg := kafka.Message{
		Topic:   p.topic,
		Key:     []byte(fmt.Sprintf("%s:%s", eventType, topic)), // compound key for partitioning
		Value:   payload,
		Headers: headers,
		Time:    time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// Close closes the Kafka writer.
func (p *EventPublisher) Close() error {
	return p.writer.Close()
}