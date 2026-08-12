package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// Publisher is the Outbox Publisher worker.
// It polls DynamoDB for PENDING outbox events and publishes them to Kafka.
// This is the reliability backbone — it guarantees at-least-once delivery even if
// Kafka was unavailable at the time of the state change.
type Publisher struct {
	outboxRepo ports.OutboxRepository
	publisher  ports.EventPublisher
	interval   time.Duration
	batchSize  int
}

// NewPublisher creates a new outbox publisher.
func NewPublisher(outboxRepo ports.OutboxRepository, publisher ports.EventPublisher, interval time.Duration) *Publisher {
	return &Publisher{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		interval:   interval,
		batchSize:  50, // process up to 50 events per tick
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (p *Publisher) Run(ctx context.Context) {
	slog.Info("Outbox publisher started", "interval", p.interval, "batchSize", p.batchSize)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Outbox publisher stopped")
			return
		case <-ticker.C:
			p.processBatch(ctx)
		}
	}
}

// processBatch fetches and publishes a batch of pending outbox events.
func (p *Publisher) processBatch(ctx context.Context) {
	events, err := p.outboxRepo.ListPending(ctx, p.batchSize)
	if err != nil {
		slog.Error("Outbox publisher: failed to list pending events", "error", err)
		return
	}
	if len(events) == 0 {
		return
	}

	slog.Debug("Outbox publisher: processing batch", "count", len(events))

	for _, event := range events {
		if err := p.publisher.Publish(ctx, event.EventType, event.AggregateID, event.Payload, event.TraceID); err != nil {
			slog.Error("Outbox publisher: failed to publish event",
				"eventId", event.EventID,
				"eventType", event.EventType,
				"error", err,
			)
			// Increment attempt counter and mark failed after max retries
			if event.Attempts >= 5 {
				if markErr := p.outboxRepo.MarkFailed(ctx, event.EventID, err.Error()); markErr != nil {
					slog.Error("Outbox publisher: failed to mark event as failed", "eventId", event.EventID, "error", markErr)
				}
			}
			continue
		}

		if markErr := p.outboxRepo.MarkPublished(ctx, event.EventID); markErr != nil {
			slog.Error("Outbox publisher: failed to mark event as published", "eventId", event.EventID, "error", markErr)
		}
	}
}
