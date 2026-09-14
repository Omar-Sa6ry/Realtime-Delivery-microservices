package domain

import "errors"

var (
	ErrPaymentNotFound = errors.New("payment not found")
	ErrPaymentInvalidStatus = errors.New("invalid payment status for this operation")
	ErrPaymentAmountInvalid = errors.New("payment amount must be positive")
	ErrPaymentNotCaptureable = errors.New("payment is not in a capturable state")
	ErrPaymentNotRefundable = errors.New("payment cannot be refunded (invalid state or insufficient balance)")
	ErrPaymentAlreadyRefunded = errors.New("payment has already been fully refunded")
	ErrInvalidCurrency = errors.New("invalid or missing currency code")
	ErrInvalidAmount = errors.New("invalid amount: must be greater than zero")
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrTimeout = errors.New("provider operation timed out — outcome unknown")
	ErrConcurrentModification = errors.New("concurrent modification detected — retry the operation")
	ErrDuplicateIdempotency = errors.New("idempotency key conflict: different request for same key")
	ErrProviderUnknownOutcome = errors.New("provider outcome is unknown — reconciliation required")
	ErrRefundNotFound = errors.New("refund not found")
)
