package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// KafkaPublisher implements event publishing to Kafka topics.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher creates a new KafkaPublisher.
func NewKafkaPublisher(brokers []string, topic string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

// PublishMessage publishes a message to the Kafka topic.
func (p *KafkaPublisher) PublishMessage(ctx context.Context, key, value string) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: []byte(value),
	}
	return p.writer.WriteMessages(ctx, msg)
}