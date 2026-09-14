package domain

import (
	"fmt"
	"time"
)

type Payment struct {
	ID                    string
	DeliveryID            string
	UserID                string
	AmountMinor           int64 // always in minor units (e.g. cents)
	Currency              string
	Status                PaymentStatus
	Provider              string // "stripe"
	ProviderPaymentID     string // Stripe PaymentIntent ID
	GatewaySessionID      string // Stripe client secret / checkout session
	AuthorizedAmountMinor int64
	CapturedAmountMinor   int64
	RefundedAmountMinor   int64
	PendingRefundMinor    int64
	Version               int64 // optimistic concurrency version
	CorrelationID         string
	CausationID           string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	AuthorizedAt          *time.Time
	CapturedAt            *time.Time
	CancelledAt           *time.Time
	FailedAt              *time.Time
}

func NewPayment(id, deliveryID, userID string, amountMinor int64, currency, correlationID, causationID string) (*Payment, error) {
	if id == "" {
		return nil, ErrPaymentAmountInvalid
	}
	if amountMinor <= 0 {
		return nil, ErrInvalidAmount
	}
	curr, err := ParseCurrency(currency)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Payment{
		ID:            id,
		DeliveryID:    deliveryID,
		UserID:        userID,
		AmountMinor:   amountMinor,
		Currency:      string(curr),
		Status:        PaymentStatusPending,
		Provider:      "stripe",
		Version:       0,
		CorrelationID: correlationID,
		CausationID:   causationID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (p *Payment) CanAuthorize() bool {
	return p.Status == PaymentStatusPending
}

func (p *Payment) CanCapture() bool {
	return p.Status == PaymentStatusAuthorized
}

func (p *Payment) CanCancel() bool {
	return p.Status == PaymentStatusPending || p.Status == PaymentStatusAuthorized
}

func (p *Payment) CanRefund(amountMinor int64) bool {
	if p.Status != PaymentStatusCaptured {
		return false
	}
	remaining := p.CapturedAmountMinor - p.RefundedAmountMinor - p.PendingRefundMinor
	return amountMinor > 0 && amountMinor <= remaining
}

func (p *Payment) Authorize(providerPaymentID, clientSecret string, authorizedAmount int64) error {
	if !p.CanAuthorize() {
		return fmt.Errorf("%w: cannot authorize from status %s", ErrPaymentInvalidStatus, p.Status)
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusAuthorized
	p.ProviderPaymentID = providerPaymentID
	p.GatewaySessionID = clientSecret
	p.AuthorizedAmountMinor = authorizedAmount
	p.AuthorizedAt = &now
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *Payment) Capture(capturedAmount int64) error {
	if !p.CanCapture() {
		return fmt.Errorf("%w: cannot capture from status %s", ErrPaymentInvalidStatus, p.Status)
	}
	if capturedAmount <= 0 || capturedAmount > p.AuthorizedAmountMinor {
		return ErrInvalidAmount
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusCaptured
	p.CapturedAmountMinor = capturedAmount
	p.CapturedAt = &now
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *Payment) Cancel() error {
	if !p.CanCancel() {
		return fmt.Errorf("%w: cannot cancel from status %s", ErrPaymentInvalidStatus, p.Status)
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusCancelled
	p.CancelledAt = &now
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *Payment) Fail() error {
	if p.Status == PaymentStatusCaptured || p.Status == PaymentStatusCancelled || p.Status == PaymentStatusFailed {
		return fmt.Errorf("%w: cannot fail from terminal status %s", ErrPaymentInvalidStatus, p.Status)
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusFailed
	p.FailedAt = &now
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *Payment) ReserveRefund(amountMinor int64) error {
	if !p.CanRefund(amountMinor) {
		return fmt.Errorf("%w: cannot refund %d from payment %s (status=%s, captured=%d, refunded=%d, pending=%d)",
			ErrPaymentNotRefundable, amountMinor, p.ID, p.Status,
			p.CapturedAmountMinor, p.RefundedAmountMinor, p.PendingRefundMinor)
	}
	p.PendingRefundMinor += amountMinor
	p.UpdatedAt = time.Now().UTC()
	p.Version++
	return nil
}

func (p *Payment) CommitRefund(amountMinor int64) error {
	if p.PendingRefundMinor < amountMinor {
		return ErrPaymentNotRefundable
	}
	p.PendingRefundMinor -= amountMinor
	p.RefundedAmountMinor += amountMinor
	if p.RefundedAmountMinor >= p.CapturedAmountMinor {
		p.Status = PaymentStatusRefunded
	}
	p.UpdatedAt = time.Now().UTC()
	p.Version++
	return nil
}

func (p *Payment) ReleaseRefundReservation(amountMinor int64) error {
	if p.PendingRefundMinor < amountMinor {
		return ErrPaymentNotRefundable
	}
	p.PendingRefundMinor -= amountMinor
	p.UpdatedAt = time.Now().UTC()
	p.Version++
	return nil
}

func (p *Payment) IsTerminal() bool {
	switch p.Status {
	case PaymentStatusCaptured, PaymentStatusCancelled, PaymentStatusFailed, PaymentStatusRefunded:
		return true
	default:
		return false
	}
}
