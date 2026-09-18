package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type EventPublisher struct {
	writer  *kafka.Writer
	timeout time.Duration
}

func NewEventPublisher(brokers []string, defaultTopic string) *EventPublisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.Hash{}, // partition by key for ordering
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		// Compression improves throughput at minimal CPU cost.
		Compression: kafka.Snappy,
	}
	return &EventPublisher{
		writer:  writer,
		timeout: 10 * time.Second,
	}
}

func (p *EventPublisher) PublishEvent(ctx context.Context, topic, key, eventType string, payload []byte) error {
	headers := []kafka.Header{
		{Key: "event_type", Value: []byte(eventType)},
		{Key: "timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
		{Key: "service", Value: []byte("payment-service")},
	}

	targetTopic := topic
	if targetTopic == "" {
		targetTopic = "payment-events"
	}

	msg := kafka.Message{
		Topic:   targetTopic,
		Key:     []byte(key),
		Value:   payload,
		Headers: headers,
		Time:    time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		slog.Error("kafka event_publisher: failed to publish",
			"topic", msg.Topic,
			"key", key,
			"eventType", eventType,
			"error", err,
		)
		return fmt.Errorf("event_publisher.PublishEvent [%s]: %w", eventType, err)
	}

	slog.Debug("kafka event_publisher: published",
		"topic", msg.Topic,
		"eventType", eventType,
		"key", key,
	)
	return nil
}

func (p *EventPublisher) Publish(ctx context.Context, topic, key string, payload []byte) error {
	return p.PublishEvent(ctx, topic, key, "", payload)
}

func (p *EventPublisher) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("event_publisher.Close: %w", err)
	}
	return nil
}

func EnsureTopics(brokers []string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("kafka.EnsureTopics: no brokers provided")
	}

	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("kafka.EnsureTopics: dial failed: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("kafka.EnsureTopics: controller lookup failed: %w", err)
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("kafka.EnsureTopics: controller dial failed: %w", err)
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{Topic: "payment-events", NumPartitions: 6, ReplicationFactor: 1},
		{Topic: "payment.created", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.authorized", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.captured", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.cancelled", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.failed", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.refunded", NumPartitions: 3, ReplicationFactor: 1},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		slog.Warn("kafka.EnsureTopics: create topics (some may already exist)", "error", err)
	}

	slog.Info("kafka.EnsureTopics: topics ensured", "brokers", brokers)
	return nil
}
