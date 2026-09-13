package workers

import (
	"context"
	"time"

	"github.com/realtime-delivery/payment-service/internal/domain"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// OutboxPublisherWorker publishes outbox messages to Kafka.
type OutboxPublisherWorker struct {
	outboxRepo domain.OutboxRepository
	publisher  ports.EventPublisher
	topic      string
}

// NewOutboxPublisherWorker creates a new OutboxPublisherWorker.
func NewOutboxPublisherWorker(outboxRepo domain.OutboxRepository, publisher ports.EventPublisher, topic string) *OutboxPublisherWorker {
	return &OutboxPublisherWorker{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		topic:      topic,
	}
}

func (w *OutboxPublisherWorker) Name() string {
	return "outbox-publisher"
}

func (w *OutboxPublisherWorker) Interval() time.Duration {
	return 5 * time.Second
}

func (w *OutboxPublisherWorker) Run(ctx context.Context) error {
	messages, err := w.outboxRepo.GetPendingOutboxMessages(100)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		return nil
	}

	ids := make([]string, len(messages))
	for i, msg := range messages {
		ids[i] = msg.ID
	}

	if err := w.outboxRepo.MarkAsProcessing(ids); err != nil {
		return err
	}

	for _, msg := range messages {
		if err := w.publishMessage(ctx, msg); err != nil {
			w.outboxRepo.MarkAsFailed([]string{msg.ID}, err.Error())
		} else {
			w.outboxRepo.MarkAsProcessed([]string{msg.ID})
		}
	}

	return nil
}

func (w *OutboxPublisherWorker) publishMessage(ctx context.Context, msg *domain.OutboxMessage) error {
	// Publish based on event type
	switch msg.EventType {
	case "payment.created":
		return w.publisher.PublishPaymentCreated(ctx, w.topic, msg.Payload)
	case "payment.authorized":
		return w.publisher.PublishPaymentAuthorized(ctx, w.topic, msg.Payload)
	case "payment.captured":
		return w.publisher.PublishPaymentCaptured(ctx, w.topic, msg.Payload)
	case "payment.cancelled":
		return w.publisher.PublishPaymentCancelled(ctx, w.topic, msg.Payload)
	case "payment.refunded":
		return w.publisher.PublishPaymentRefunded(ctx, w.topic, msg.Payload)
	case "payment.failed":
		return w.publisher.PublishPaymentFailed(ctx, w.topic, msg.Payload)
	}
	return nil
}