package workers

import (
	"context"
	"time"

	"github.com/realtime-delivery/payment-service/internal/domain"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// ReconciliationWorker reconciles payment states with the provider.
type ReconciliationWorker struct {
	paymentRepo  domain.PaymentRepo
	provider     ports.PaymentProvider
	intervalSec  int
	stuckSec     int
}

// NewReconciliationWorker creates a new ReconciliationWorker.
func NewReconciliationWorker(paymentRepo domain.PaymentRepo, provider ports.PaymentProvider, intervalSec, stuckSec int) *ReconciliationWorker {
	return &ReconciliationWorker{
		paymentRepo: paymentRepo,
		provider:    provider,
		intervalSec: intervalSec,
		stuckSec:    stuckSec,
	}
}

func (w *ReconciliationWorker) Name() string {
	return "reconciliation"
}

func (w *ReconciliationWorker) Interval() time.Duration {
	return time.Duration(w.intervalSec) * time.Second
}

func (w *ReconciliationWorker) Run(ctx context.Context) error {
	payments, err := w.paymentRepo.List()
	if err != nil {
		return err
	}

	for _, payment := range payments {
		if err := w.reconcilePayment(ctx, payment); err != nil {
			// Log error but continue with other payments
			// logger.Error("reconciliation error", "paymentID", payment.ID, "error", err)
		}
	}

	return nil
}

func (w *ReconciliationWorker) reconcilePayment(ctx context.Context, payment *domain.Payment) error {
	// Skip terminal states
	if payment.IsTerminal() {
		return nil
	}

	// Check if payment is stuck in PROCESSING state
	if payment.Status == "processing" || payment.Status == "pending" {
		elapsed := time.Now().Unix() - payment.UpdatedAt
		if elapsed > int64(w.stuckSec) {
			// Mark as UNKNOWN for reconciliation
			payment.Status = "unknown"
			// Note: In a real implementation, you'd update the repo here
			// w.paymentRepo.Update(payment)
		}
	}

	// Query provider for current state
	// This would call the provider's GetPayment or similar method
	// For now, we'll skip the actual provider call
	// providerPayment, err := w.provider.GetPayment(ctx, payment.ID)

	return nil
}