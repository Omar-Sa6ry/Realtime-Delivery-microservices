package domain

import (
	"errors"
	"fmt"
)

type Money struct {
	AmountMinor int64
	Currency    string
}

func NewMoney(amountMinor int64, currency string) (Money, error) {
	if amountMinor <= 0 {
		return Money{}, fmt.Errorf("%w: got %d", ErrInvalidAmount, amountMinor)
	}
	if currency == "" {
		return Money{}, ErrInvalidCurrency
	}
	return Money{AmountMinor: amountMinor, Currency: currency}, nil
}

func (m Money) Add(other Money) (Money, error) {
	if err := m.assertSameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{AmountMinor: m.AmountMinor + other.AmountMinor, Currency: m.Currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if err := m.assertSameCurrency(other); err != nil {
		return Money{}, err
	}
	result := m.AmountMinor - other.AmountMinor
	if result < 0 {
		return Money{}, fmt.Errorf("%w: subtraction would result in negative amount", ErrInvalidAmount)
	}
	return Money{AmountMinor: result, Currency: m.Currency}, nil
}

func (m Money) IsZero() bool {
	return m.AmountMinor == 0
}

func (m Money) LessThanOrEqual(other Money) bool {
	if m.Currency != other.Currency {
		return false
	}
	return m.AmountMinor <= other.AmountMinor
}

func (m Money) GreaterThan(other Money) bool {
	if m.Currency != other.Currency {
		return false
	}
	return m.AmountMinor > other.AmountMinor
}

func (m Money) Equal(other Money) bool {
	return m.Currency == other.Currency && m.AmountMinor == other.AmountMinor
}

func (m Money) String() string {
	return fmt.Sprintf("%d %s", m.AmountMinor, m.Currency)
}

func (m Money) assertSameCurrency(other Money) error {
	if m.Currency != other.Currency {
		return errors.New(fmt.Sprintf("%v: %s vs %s", ErrCurrencyMismatch, m.Currency, other.Currency))
	}
	return nil
}
