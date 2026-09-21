package domain

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

type PaymentTransactionType string

const (
	TransactionAuthorization PaymentTransactionType = "AUTHORIZATION"
	TransactionCapture       PaymentTransactionType = "CAPTURE"
	TransactionRefund        PaymentTransactionType = "REFUND"
	TransactionCancel        PaymentTransactionType = "CANCEL"
)

func (t PaymentTransactionType) IsValid() bool {
	switch t {
	case TransactionAuthorization, TransactionCapture, TransactionRefund, TransactionCancel:
		return true
	default:
		return false
	}
}

func ParseDecimalAmount(raw string) (*big.Rat, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("%w: empty", ErrInvalidAmountFormat)
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAmountFormat, raw)
	}
	if r.Sign() < 0 {
		return nil, fmt.Errorf("%w: %q", ErrNegativeAmount, raw)
	}
	return r, nil
}

type FactPaymentTransaction struct {
	EventID           string
	PaymentID         string
	DeliveryID        string
	UserID            string
	Provider          string
	TransactionType   PaymentTransactionType
	Status            string
	Amount            string // exact decimal, e.g. "125.50"
	Currency          string
	ProviderLatencyMs *uint64
	OccurredAt        time.Time
	IngestedAt        time.Time
}

func (f *FactPaymentTransaction) Validate() error {
	if f == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(f.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(f.PaymentID) == "" {
		return ErrMissingPaymentID
	}
	if !f.TransactionType.IsValid() {
		return fmt.Errorf("%w: transaction_type %s", ErrUnknownEventType, f.TransactionType)
	}
	if _, err := ParseDecimalAmount(f.Amount); err != nil {
		return err
	}
	if strings.TrimSpace(f.Currency) == "" {
		return fmt.Errorf("%w: currency", ErrInvalidAmountFormat)
	}
	if f.OccurredAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	return nil
}

func RefundExceedsCapture(refundAmount, capturedAmount string) (bool, error) {
	r, err := ParseDecimalAmount(refundAmount)
	if err != nil {
		return false, err
	}
	c, err := ParseDecimalAmount(capturedAmount)
	if err != nil {
		return false, err
	}
	return r.Cmp(c) > 0, nil
}
