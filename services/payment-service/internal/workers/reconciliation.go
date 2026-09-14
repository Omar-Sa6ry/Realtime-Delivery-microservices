package workers

import (
	"context"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/providers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type ReconciliationWorker struct {
	attemptRepo *postgres.AttemptRepository
	paymentRepo *postgres.PaymentRepository
	provider    providers.PaymentProvider
}

func NewReconciliationWorker(
	attemptRepo *postgres.AttemptRepository,
	paymentRepo *postgres.PaymentRepository,
	provider providers.PaymentProvider,
) *ReconciliationWorker {
	return &ReconciliationWorker{
		attemptRepo: attemptRepo,
		paymentRepo: paymentRepo,
		provider:    provider,
	}
}

func (w *ReconciliationWorker) Run(ctx context.Context) error {
	attempts, err := w.attemptRepo.FindUnknown(ctx, 20)
	if err != nil {
		return err
	}
	if len(attempts) == 0 {
		return nil
	}

	slog.Info("reconciliation_worker: reconciling attempts", "count", len(attempts))

	for _, att := range attempts {
		if err := w.reconcileOne(ctx, att); err != nil {
			slog.Error("reconciliation_worker: failed to reconcile attempt",
				"attemptID", att.ID,
				"paymentID", att.PaymentID,
				"error", err,
			)
		}
	}
	return nil
}

func (w *ReconciliationWorker) reconcileOne(ctx context.Context, att *domain.Attempt) error {
	res, nErr := w.provider.GetStatus(ctx, att.ProviderTransactionID)
	if nErr != nil {
		return nErr
	}

	if res.Status == "SUCCEEDED" || res.Status == "AUTHORIZED" {
		_ = w.attemptRepo.UpdateStatus(ctx, att.ID, domain.OperationStatusSucceeded, res.ProviderTransactionID)
	} else if res.Status == "FAILED" || res.Status == "CANCELED" {
		_ = w.attemptRepo.UpdateStatus(ctx, att.ID, domain.OperationStatusFailed, res.ProviderTransactionID)
	}
	return nil
}
