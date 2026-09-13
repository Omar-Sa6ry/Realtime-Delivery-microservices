package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/realtime-delivery/payment-service/internal/domain"
	"github.com/realtime-delivery/payment-service/internal/ports"
	"github.com/segmentio/kafka-go"
)

// OutboxKafkaPublisher implements the OutboxKafkaPublisher pattern.
type OutboxKafkaPublisher struct {
	outboxRepo domain.OutboxRepository
	publisher  *EventPublisher
	batchSize  int
	interval   time.Duration
	stopCh     chan struct{}
}

// NewOutboxKafkaPublisher creates a new OutboxKafkaPublisher.
func NewOutboxKafkaPublisher(outboxRepo domain.OutboxRepository, publisher *EventPublisher, batchSize int, interval time.Duration) *OutboxKafkaPublisher {
	return &OutboxKafkaPublisher{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		batchSize:  batchSize,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the outbox publisher worker.
func (p *OutboxKafkaPublisher) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishPending(ctx); err != nil {
				// Log error
				fmt.Printf("Outbox publisher error: %v\n", err)
			}
		}
	}
}

// Stop stops the publisher.
func (p *OutboxKafkaPublisher) Stop() {
	close(p.stopCh)
}

// publishPending publishes pending outbox messages.
func (p *OutboxKafkaPublisher) publishPending(ctx context.Context) error {
	messages, err := p.outboxRepo.GetPendingOutboxMessages(p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending outbox messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	ids := make([]string, len(messages))
	for i, msg := range messages {
		ids[i] = msg.ID
	}

	// Mark as processing
	if err := p.outboxRepo.MarkAsProcessing(ids); err != nil {
		return fmt.Errorf("failed to mark messages as processing: %w", err)
	}

	for _, msg := range messages {
		if err := p.publishMessage(ctx, msg); err != nil {
			p.outboxRepo.MarkAsFailed([]string{msg.ID}, err.Error())
		} else {
			p.outboxRepo.MarkAsProcessed([]string{msg.ID})
		}
	}

	return nil
}

// publishMessage publishes a single message to Kafka.
func (p *OutboxKafkaPublisher) publishMessage(ctx context.Context, msg *domain.OutboxMessage) error {
	return p.publisher.PublishEvent(ctx, p.publisher.topic, msg.ID, msg.EventType, []byte(msg.Payload))
}

// EnsureTopics ensures the required Kafka topics exist.
func EnsureTopics(brokers []string) error {
	conn, err := kafka.DialLeader(context.Background(), "tcp", brokers[0], "payment.created", 0)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	// Create topics if they don't exist
	topicConfigs := []kafka.TopicConfig{
		{Topic: "payment.created", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.authorization.started", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.authorized", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.authorization.failed", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.capture.started", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.captured", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.capture.failed", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.cancelled", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.refund.started", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.refunded", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.refund.failed", NumPartitions: 3, ReplicationFactor: 1},
		{Topic: "payment.failed", NumPartitions: 3, ReplicationFactor: 1},
	}

	controllerConn, err := kafka.DialLeader(context.Background(), "tcp", "kafka-srv:9092", "__consumer_offsets", 0)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka controller: %w", err)
	}
	defer controllerConn.Close()

	err := controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("failed to create topics: %w", err)
	}

	return nil
}