package domain

import (
	"fmt"
	"time"
)

type Attempt struct {
	ID                     string
	PaymentID              string
	AttemptNumber          int
	Operation              string // AUTHORIZE | CAPTURE | VOID | REFUND
	Status                 OperationStatus
	Provider               string
	ProviderRequestID      string
	ProviderTransactionID  string
	ProviderIdempotencyKey string
	ErrorCode              string
	ErrorCategory          string
	SafeErrorMessage       string
	CreatedAt              time.Time
	StartedAt              *time.Time
	CompletedAt            *time.Time
}

func NewAttempt(id, paymentID, operation, provider, idempotencyKey string, attemptNumber int) *Attempt {
	now := time.Now().UTC()
	return &Attempt{
		ID:                     id,
		PaymentID:              paymentID,
		AttemptNumber:          attemptNumber,
		Operation:              operation,
		Status:                 OperationStatusProcessing,
		Provider:               provider,
		ProviderIdempotencyKey: idempotencyKey,
		CreatedAt:              now,
		StartedAt:              &now,
	}
}

func (a *Attempt) Succeed(providerTxID string) {
	now := time.Now().UTC()
	a.Status = OperationStatusSucceeded
	a.ProviderTransactionID = providerTxID
	a.CompletedAt = &now
}

func (a *Attempt) Fail(errorCode, category, safeMessage string) {
	now := time.Now().UTC()
	a.Status = OperationStatusFailed
	a.ErrorCode = errorCode
	a.ErrorCategory = category
	a.SafeErrorMessage = safeMessage
	a.CompletedAt = &now
}

func (a *Attempt) MarkUnknown() {
	now := time.Now().UTC()
	a.Status = OperationStatusUnknown
	a.CompletedAt = &now
}

func (a *Attempt) String() string {
	return fmt.Sprintf("Attempt[id=%s payment=%s op=%s status=%s]",
		a.ID, a.PaymentID, a.Operation, a.Status)
}
