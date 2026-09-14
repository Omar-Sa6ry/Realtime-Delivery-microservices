package workers

import (
	"context"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
)

type CleanupWorker struct {
	outboxRepo    *postgres.OutboxRepository
	olderThanDays int
}

func NewCleanupWorker(outboxRepo *postgres.OutboxRepository) *CleanupWorker {
	return &CleanupWorker{
		outboxRepo:    outboxRepo,
		olderThanDays: 7,
	}
}

func (w *CleanupWorker) Run(ctx context.Context) error {
	deleted, err := w.outboxRepo.Cleanup(ctx, w.olderThanDays)
	if err != nil {
		slog.Error("cleanup_worker: outbox cleanup failed", "error", err)
		return err
	}
	if deleted > 0 {
		slog.Info("cleanup_worker: cleaned up published outbox messages", "count", deleted)
	}
	return nil
}
