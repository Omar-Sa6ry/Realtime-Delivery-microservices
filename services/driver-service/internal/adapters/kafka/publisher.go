package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaPublisher implements event publishing to Kafka topics.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher creates a new KafkaPublisher.
func NewKafkaPublisher(brokers []string, defaultTopic string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        defaultTopic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// PublishMessage publishes a message to a specific Kafka topic with a key.
func (p *KafkaPublisher) PublishMessage(ctx context.Context, topic, key string, value []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
		Headers: []kafka.Header{
			{Key: "x-source", Value: []byte("driver-service")},
			{Key: "x-timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
		},
		Time: time.Now(),
	}
	return p.writer.WriteMessages(ctx, msg)
}

// Close closes the underlying Kafka writer.
func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}