package domain

import (
	"fmt"
	"time"
)

// Refund represents a refund request for a payment.
type Refund struct {
	ID         string
	PaymentID  string
	Amount     int64
	Currency   string
	Reason     string
	Status     RefundStatus
	CreatedAt  int64
	CompletedAt int64
	Error      string
}

// RefundStatus represents the status of a refund.
type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "pending"
	RefundStatusProcessed RefundStatus = "processed"
	RefundStatusCompleted RefundStatus = "completed"
	RefundStatusFailed    RefundStatus = "failed"
	RefundStatusCancelled RefundStatus = "cancelled"
)

// NewRefund creates a new refund request.
func NewRefund(paymentID string, amount int64, currency string, reason string) *Refund {
	return &Refund{
		ID:         generateID(),
		PaymentID:  paymentID,
		Amount:     amount,
		Currency:   currency,
		Reason:     reason,
		Status:     RefundStatusPending,
		CreatedAt:  timeNow(),
		CompletedAt: 0,
		Error:      "",
	}
}

// Process processes the refund.
func (r *Refund) Process() {
	r.Status = RefundStatusProcessed
	r.CompletedAt = timeNow()
}

// Complete completes the refund successfully.
func (r *Refund) Complete() {
	r.Status = RefundStatusCompleted
	r.CompletedAt = timeNow()
}

// Fail fails the refund with an error.
func (r *Refund) Fail(err string) {
	r.Status = RefundStatusFailed
	r.Error = err
	r.CompletedAt = timeNow()
}

// Cancel cancels the refund.
func (r *Refund) Cancel() {
	r.Status = RefundStatusCancelled
	r.CompletedAt = timeNow()
}

// IsCompleted returns true if the refund is completed or failed.
func (r *Refund) IsCompleted() bool {
	return r.Status == RefundStatusCompleted || r.Status == RefundStatusFailed
}

// IsFailed returns true if the refund failed.
func (r *Refund) IsFailed() bool {
	return r.Status == RefundStatusFailed
}

// IsTerminal returns true if the refund is in a terminal state.
func (r *Refund) IsTerminal() bool {
	return r.Status == RefundStatusCompleted || r.Status == RefundStatusFailed || r.Status == RefundStatusCancelled
}

// String returns a string representation of the refund.
func (r *Refund) String() string {
	return fmt.Sprintf("Refund[ID=%s, PaymentID=%s, Status=%s, Amount=%d%s]",
		r.ID, r.PaymentID, r.Status, r.Amount, r.Currency)
}

var refundTimeFunc = func() int64 { return time.Now().Unix() }