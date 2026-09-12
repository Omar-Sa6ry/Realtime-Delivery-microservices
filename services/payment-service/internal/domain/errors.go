package domain

import "fmt"

// Error represents a domain error with a code and message.
type Error struct {
	Code    string
	Message string
}

// NewError creates a new domain error.
func NewError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Is implements the errors.Is function for domain errors.
func (e *Error) Is(target error) bool {
	if te, ok := target.(*Error); ok {
		return e.Code == te.Code
	}
	return false
}

// Common domain errors.

// ErrPaymentNotFound is returned when a payment is not found.
var ErrPaymentNotFound = NewError("PAYMENT_NOT_FOUND", "payment not found")

// ErrPaymentInvalidStatus is returned when a payment operation is invalid for the current status.
var ErrPaymentInvalidStatus = NewError("PAYMENT_INVALID_STATUS", "payment operation invalid for current status")

// ErrPaymentAmountInvalid is returned when the payment amount is invalid.
var ErrPaymentAmountInvalid = NewError("PAYMENT_AMOUNT_INVALID", "payment amount is invalid")

// ErrPaymentNotCaptureable is returned when a payment cannot be captured.
var ErrPaymentNotCaptureable = NewError("PAYMENT_NOT_CAPTURABLE", "payment is not captureable")

// ErrPaymentNotRefundable is returned when a payment cannot be refunded.
var ErrPaymentNotRefundable = NewError("PAYMENT_NOT_REFUNDABLE", "payment is not refundable")

// ErrPaymentAlreadyRefunded is returned when a payment has already been refunded.
var ErrPaymentAlreadyRefunded = NewError("PAYMENT_ALREADY_REFUNDED", "payment has already been refunded")

// ErrPaymentAlreadyCancelled is returned when a payment has already been cancelled.
var ErrPaymentAlreadyCancelled = NewError("PAYMENT_ALREADY_CANCELLED", "payment has already been cancelled")

// ErrInvalidCurrency is returned when the currency is invalid.
var ErrInvalidCurrency = NewError("INVALID_CURRENCY", "invalid currency code")

// ErrInvalidAmount is returned when the amount is invalid.
var ErrInvalidAmount = NewError("INVALID_AMOUNT", "invalid amount")

// ErrTimeout is returned when an operation times out.
var ErrTimeout = NewError("TIMEOUT", "operation timed out")

// ErrConcurrentModification is returned when there's a concurrent modification.
var ErrConcurrentModification = NewError("CONCURRENT_MODIFICATION", "concurrent modification detected")