package domain

import (
	"fmt"
	"time"
)

type Refund struct {
	ID               string
	PaymentID        string
	DeliveryID       string
	AmountMinor      int64
	Currency         string
	Status           RefundStatus
	Reason           string
	ProviderRefundID string // Stripe Refund ID
	IdempotencyKey   string
	CorrelationID    string
	CausationID      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	FailedAt         *time.Time
	FailureReason    string
}

func NewRefund(id, paymentID, deliveryID string, amountMinor int64, currency, reason, idempotencyKey, correlationID, causationID string) (*Refund, error) {
	if id == "" {
		return nil, ErrInvalidAmount
	}
	if amountMinor <= 0 {
		return nil, ErrInvalidAmount
	}
	now := time.Now().UTC()
	return &Refund{
		ID:             id,
		PaymentID:      paymentID,
		DeliveryID:     deliveryID,
		AmountMinor:    amountMinor,
		Currency:       currency,
		Status:         RefundStatusPending,
		Reason:         reason,
		IdempotencyKey: idempotencyKey,
		CorrelationID:  correlationID,
		CausationID:    causationID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (r *Refund) Complete(providerRefundID string) {
	now := time.Now().UTC()
	r.Status = RefundStatusCompleted
	r.ProviderRefundID = providerRefundID
	r.CompletedAt = &now
	r.UpdatedAt = now
}

func (r *Refund) Fail(safeReason string) {
	now := time.Now().UTC()
	r.Status = RefundStatusFailed
	r.FailureReason = safeReason
	r.FailedAt = &now
	r.UpdatedAt = now
}

func (r *Refund) IsTerminal() bool {
	return r.Status == RefundStatusCompleted || r.Status == RefundStatusFailed
}

func (r *Refund) String() string {
	return fmt.Sprintf("Refund[id=%s payment=%s amount=%d status=%s]",
		r.ID, r.PaymentID, r.AmountMinor, r.Status)
}
