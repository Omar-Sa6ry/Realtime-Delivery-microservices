package domain

import (
	"fmt"
	"strings"
)

type Currency string

const (
	CurrencyEGP Currency = "EGP"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencySAR Currency = "SAR"
	CurrencyAED Currency = "AED"
)

func (c Currency) IsValid() bool {
	switch c {
	case CurrencyEGP, CurrencyUSD, CurrencyEUR, CurrencySAR, CurrencyAED:
		return true
	default:
		return false
	}
}

func ParseCurrency(s string) (Currency, error) {
	c := Currency(strings.ToUpper(strings.TrimSpace(s)))
	if !c.IsValid() {
		return "", fmt.Errorf("%w: '%s'", ErrInvalidCurrency, s)
	}
	return c, nil
}

type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "PENDING"
	PaymentStatusAuthorized PaymentStatus = "AUTHORIZED"
	PaymentStatusCaptured   PaymentStatus = "CAPTURED"
	PaymentStatusCancelled  PaymentStatus = "CANCELLED"
	PaymentStatusFailed     PaymentStatus = "FAILED"
	PaymentStatusRefunded   PaymentStatus = "REFUNDED"
)

func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentStatusPending, PaymentStatusAuthorized, PaymentStatusCaptured,
		PaymentStatusCancelled, PaymentStatusFailed, PaymentStatusRefunded:
		return true
	default:
		return false
	}
}

type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "REFUND_PENDING"
	RefundStatusCompleted RefundStatus = "REFUNDED"
	RefundStatusFailed    RefundStatus = "REFUND_FAILED"
)

func (s RefundStatus) IsValid() bool {
	switch s {
	case RefundStatusPending, RefundStatusCompleted, RefundStatusFailed:
		return true
	default:
		return false
	}
}

type OperationStatus string

const (
	OperationStatusNotStarted OperationStatus = "NOT_STARTED"
	OperationStatusProcessing OperationStatus = "PROCESSING"
	OperationStatusSucceeded  OperationStatus = "SUCCEEDED"
	OperationStatusFailed     OperationStatus = "FAILED"
	OperationStatusUnknown    OperationStatus = "UNKNOWN"
)

var AllowedTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentStatusPending:    {PaymentStatusAuthorized, PaymentStatusCancelled, PaymentStatusFailed},
	PaymentStatusAuthorized: {PaymentStatusCaptured, PaymentStatusCancelled, PaymentStatusFailed},
	PaymentStatusCaptured:   {PaymentStatusRefunded},
	PaymentStatusCancelled:  {},
	PaymentStatusFailed:     {},
	PaymentStatusRefunded:   {},
}
