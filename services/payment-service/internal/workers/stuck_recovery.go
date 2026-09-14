package workers

import (
	"context"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type StuckRecoveryWorker struct {
	attemptRepo      *postgres.AttemptRepository
	thresholdSeconds int
}

func NewStuckRecoveryWorker(attemptRepo *postgres.AttemptRepository, thresholdSeconds int) *StuckRecoveryWorker {
	return &StuckRecoveryWorker{
		attemptRepo:      attemptRepo,
		thresholdSeconds: thresholdSeconds,
	}
}

func (w *StuckRecoveryWorker) Run(ctx context.Context) error {
	stuck, err := w.attemptRepo.FindStuck(ctx, w.thresholdSeconds)
	if err != nil {
		return err
	}
	if len(stuck) == 0 {
		return nil
	}

	slog.Warn("stuck_recovery: found stuck attempts", "count", len(stuck))

	for _, att := range stuck {
		err := w.attemptRepo.UpdateStatus(ctx, att.ID, domain.OperationStatusUnknown, "")
		if err != nil {
			slog.Error("stuck_recovery: failed to mark attempt unknown",
				"attemptID", att.ID,
				"error", err,
			)
		}
	}
	return nil
}
