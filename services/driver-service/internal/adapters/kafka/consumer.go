package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// KafkaConsumer consumes messages from Kafka topics.
type KafkaConsumer struct {
	reader *kafka.Reader
}

// NewKafkaConsumer creates a new KafkaConsumer.
func NewKafkaConsumer(brokers []string, topic string, groupID string) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 1e5,
			MaxBytes: 1e6,
		}),
	}
}

// ConsumeMessage consumes a single message from Kafka.
func (c *KafkaConsumer) ConsumeMessage(ctx context.Context) (kafka.Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return kafka.Message{}, err
	}
	return msg, nil
}

// CommitMessage commits the processing of a message.
func (c *KafkaConsumer) CommitMessage(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}