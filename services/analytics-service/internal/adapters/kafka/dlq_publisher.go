package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pkgkafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
)

type DLQMessage struct {
	EventID       string `json:"eventId"`
	EventType     string `json:"eventType"`
	EventVersion  int    `json:"eventVersion"`
	ConsumerGroup string `json:"consumerGroup"`
	OriginalTopic string `json:"originalTopic"`
	Partition     int32  `json:"partition"`
	Offset        int64  `json:"offset"`
	AttemptCount  int    `json:"attemptCount"`
	Error         string `json:"error"`
	FailedAt      string `json:"failedAt"`
	CorrelationID string `json:"correlationId,omitempty"`
	Payload       []byte `json:"payload"`
}

type DLQPublisher struct {
	producer *pkgkafka.Producer
	topic    string
	group    string
}

func NewDLQPublisher(brokers []string, topic, group string) *DLQPublisher {
	return &DLQPublisher{
		producer: pkgkafka.NewProducer(brokers),
		topic:    topic,
		group:    group,
	}
}

func (p *DLQPublisher) Publish(ctx context.Context, msg DLQMessage) error {
	if msg.FailedAt == "" {
		msg.FailedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if msg.ConsumerGroup == "" {
		msg.ConsumerGroup = p.group
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("dlq marshal: %w", err)
	}
	key := msg.EventID
	if key == "" {
		key = fmt.Sprintf("%s-%d-%d", msg.OriginalTopic, msg.Partition, msg.Offset)
	}
	return p.producer.Publish(ctx, p.topic, key, data, msg.CorrelationID)
}

func (p *DLQPublisher) Close() error {
	return p.producer.Close()
}
