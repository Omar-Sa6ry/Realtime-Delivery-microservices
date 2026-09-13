package workers

import (
	"context"
	"time"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

// StuckRecoveryWorker recovers payments stuck in PROCESSING state.
type StuckRecoveryWorker struct {
	paymentRepo domain.PaymentRepo
	threshold   time.Duration
}

// NewStuckRecoveryWorker creates a new StuckRecoveryWorker.
func NewStuckRecoveryWorker(paymentRepo domain.PaymentRepo, thresholdSec int) *StuckRecoveryWorker {
	return &StuckRecoveryWorker{
		paymentRepo: paymentRepo,
		threshold:   time.Duration(thresholdSec) * time.Second,
	}
}

func (w *StuckRecoveryWorker) Name() string {
	return "stuck-recovery"
}

func (w *StuckRecoveryWorker) Interval() time.Duration {
	return w.threshold
}

func (w *StuckRecoveryWorker) Run(ctx context.Context) error {
	payments, err := w.paymentRepo.List()
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, payment := range payments {
		if payment.Status == "processing" || payment.Status == "pending" {
			elapsed := now - payment.UpdatedAt
			if elapsed > int64(w.threshold.Seconds()) {
				// Mark as UNKNOWN for reconciliation
				payment.Status = "unknown"
				if err := w.paymentRepo.Update(payment); err != nil {
					// Log error but continue
					// logger.Error("stuck recovery update failed", "paymentID", payment.ID, "error", err)
				}
			}
		}
	}

	return nil
}