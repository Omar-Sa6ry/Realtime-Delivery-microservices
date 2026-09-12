package domain

// Money represents an amount of money with a currency.
type Money struct {
	Amount int64 // Amount in minor units (e.g., cents for USD)
	Currency string
}

// NewMoney creates a new Money instance.
func NewMoney(amount int64, currency string) *Money {
	if amount < 0 {
		amount = 0
	}
	return &Money{
		Amount: amount,
		Currency: currency,
	}
}

// Amount returns the amount in the base unit (e.g., dollars for USD).
func (m *Money) AmountInBaseUnit() float64 {
	return float64(m.Amount) / 100.0
}

// Cents returns the amount in cents.
func (m *Money) Cents() int64 {
	return m.Amount
}

// Currency returns the currency code.
func (m *Money) CurrencyCode() string {
	return m.Currency
}

// Add adds two Money values. Both must have the same currency.
func (m *Money) Add(other *Money) *Money {
	if m.Currency != other.Currency {
		// In a real implementation, this would use currency conversion
		panic("cannot add Money with different currencies")
	}
	return NewMoney(m.Amount+other.Amount, m.Currency)
}

// Subtract subtracts another Money value. Both must have the same currency.
func (m *Money) Subtract(other *Money) *Money {
	if m.Currency != other.Currency {
		panic("cannot subtract Money with different currencies")
	}
	newAmount := m.Amount - other.Amount
	if newAmount < 0 {
		newAmount = 0
	}
	return NewMoney(newAmount, m.Currency)
}

// Multiply multiplies Money by an integer factor.
func (m *Money) Multiply(factor int64) *Money {
	return NewMoney(m.Amount*factor, m.Currency)
}

// IsZero returns true if the money amount is zero.
func (m *Money) IsZero() bool {
	return m.Amount == 0
}

// GreaterThan returns true if m > other.
func (m *Money) GreaterThan(other *Money) bool {
	if m.Currency != other.Currency {
		panic("cannot compare Money with different currencies")
	}
	return m.Amount > other.Amount
}

// EqualTo returns true if m == other.
func (m *Money) EqualTo(other *Money) bool {
	if m.Currency != other.Currency {
		panic("cannot compare Money with different currencies")
	}
	return m.Amount == other.Amount
}