package workers

import (
	"context"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	pkgevents "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
)

type OutboxPublisher struct {
	outboxRepo *postgres.OutboxRepository
	publisher  *kafka.EventPublisher
	batchSize  int
}

func NewOutboxPublisher(
	outboxRepo *postgres.OutboxRepository,
	publisher *kafka.EventPublisher,
	batchSize int,
) *OutboxPublisher {
	return &OutboxPublisher{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		batchSize:  batchSize,
	}
}

func (w *OutboxPublisher) Run(ctx context.Context) error {
	events, err := w.outboxRepo.FetchUnpublished(ctx, w.batchSize)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	slog.Debug("outbox_publisher: processing events", "count", len(events))

	for _, evt := range events {
		if err := w.publishOne(ctx, evt); err != nil {
			slog.Error("outbox_publisher: failed to publish event",
				"id", evt.ID,
				"eventType", evt.EventType,
				"error", err,
			)
			// Mark failed for observability, continue with next event.
			_ = w.outboxRepo.MarkFailed(ctx, evt.ID, err.Error())
		} else {
			if err := w.outboxRepo.MarkPublished(ctx, evt.ID); err != nil {
				slog.Error("outbox_publisher: failed to mark published",
					"id", evt.ID, "error", err)
			}
		}
	}
	return nil
}

func (w *OutboxPublisher) publishOne(ctx context.Context, evt *postgres.OutboxRow) error {
	var envelope pkgevents.EventEnvelope
	key := evt.ID // fallback: use event ID as key
	if err := parseEnvelopeKey(evt.Payload, &envelope); err == nil && envelope.AggregateID != "" {
		key = envelope.AggregateID
	}

	topic := eventTypeToTopic(evt.EventType)
	return w.publisher.PublishEvent(ctx, topic, key, evt.EventType, evt.Payload)
}

func eventTypeToTopic(eventType string) string {
	if eventType != "" {
		return eventType
	}
	return "payment-events"
}
