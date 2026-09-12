package domain

import (
	"fmt"
	"time"
)

// Attempt represents a payment attempt.
type Attempt struct {
	ID        string
	PaymentID string
	Amount    int64
	Currency  string
	Status    string
	CreatedAt int64
	CompletedAt int64
	Error     string
}

// NewAttempt creates a new payment attempt.
func AttemptNew(paymentID string, amount int64, currency string) *Attempt {
	return &Attempt{
		ID:         generateID(),
		PaymentID:  paymentID,
		Amount:     amount,
		Currency:   currency,
		Status:     "pending",
		CreatedAt:  timeNow(),
		CompletedAt: 0,
		Error:     "",
	}
}

// Complete completes the attempt with success.
func (a *Attempt) Complete() {
	a.Status = "completed"
	a.CompletedAt = timeNow()
}

// Fail marks the attempt as failed with an error.
func (a *Attempt) Fail(err string) {
	a.Status = "failed"
	a.Error = err
	a.CompletedAt = timeNow()
}

// IsCompleted returns true if the attempt is completed (success or fail).
func (a *Attempt) IsCompleted() bool {
	return a.Status == "completed" || a.Status == "failed"
}

// IsFailed returns true if the attempt failed.
func (a *Attempt) IsFailed() bool {
	return a.Status == "failed"
}

// String returns a string representation of the attempt.
func (a *Attempt) String() string {
	return fmt.Sprintf("Attempt[ID=%s, PaymentID=%s, Status=%s, Amount=%d%s]",
		a.ID, a.PaymentID, a.Status, a.Amount, a.Currency)
}

var attemptTimeFunc = func() int64 { return time.Now().Unix() }

func timeNow() int64 {
	if attemptTimeFunc != nil {
		return attemptTimeFunc()
	}
	return time.Now().Unix()
}