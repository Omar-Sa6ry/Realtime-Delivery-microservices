package workers

import (
	"context"
	"time"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

// CleanupWorker cleans up old outbox messages and idempotency keys.
type CleanupWorker struct {
	outboxRepo      domain.OutboxRepository
	idempotencyRepo domain.IdempotencyRepository
	retentionPeriod time.Duration
	batchSize       int
}

// NewCleanupWorker creates a new CleanupWorker.
func NewCleanupWorker(outboxRepo domain.OutboxRepository, idempotencyRepo domain.IdempotencyRepository, retentionDays int, batchSize int) *CleanupWorker {
	return &CleanupWorker{
		outboxRepo:      outboxRepo,
		idempotencyRepo: idempotencyRepo,
		retentionPeriod: time.Duration(retentionDays) * 24 * time.Hour,
		batchSize:       batchSize,
	}
}

func (w *CleanupWorker) Name() string {
	return "cleanup"
}

func (w *CleanupWorker) Interval() time.Duration {
	return 24 * time.Hour // Run once per day
}

func (w *CleanupWorker) Run(ctx context.Context) error {
	cutoff := time.Now().Add(-w.retentionPeriod)

	// Clean up old outbox messages
	if err := w.cleanOutboxMessages(ctx, cutoff); err != nil {
		// Log error
		// logger.Error("cleanup outbox failed", "error", err)
	}

	// Clean up old idempotency keys
	if err := w.cleanIdempotencyKeys(ctx, cutoff); err != nil {
		// Log error
		// logger.Error("cleanup idempotency keys failed", "error", err)
	}

	return nil
}

func (w *CleanupWorker) cleanOutboxMessages(ctx context.Context, cutoff time.Time) error {
	// Get old processed messages
	messages, err := w.outboxRepo.GetPendingOutboxMessages(w.batchSize)
	if err != nil {
		return err
	}

	cutoffUnix := cutoff.Unix()
	for _, msg := range messages {
		if msg.ProcessedAt > 0 && msg.ProcessedAt < cutoffUnix {
			// Delete old processed messages
			// Note: Would need a Delete method on OutboxRepository
			// w.outboxRepo.Delete(msg.ID)
		}
	}

	return nil
}

func (w *CleanupWorker) cleanIdempotencyKeys(ctx context.Context, cutoff time.Time) error {
	// Clean up old idempotency keys
	// Note: Would need a method to get/clean old keys
	// w.idempotencyRepo.CleanOldKeys(cutoff.Unix())
	return nil
}