package domain

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentTransactionType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		txType   PaymentTransactionType
		expected bool
	}{
		{"authorization", TransactionAuthorization, true},
		{"capture", TransactionCapture, true},
		{"refund", TransactionRefund, true},
		{"cancel", TransactionCancel, true},
		{"invalid", PaymentTransactionType("INVALID"), false},
		{"empty", PaymentTransactionType(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.txType.IsValid())
		})
	}
}

func TestParseDecimalAmount(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    *big.Rat
	}{
		{
			name:     "valid positive decimal",
			input:    "100.50",
			expected: big.NewRat(10050, 100),
		},
		{
			name:     "valid integer",
			input:    "100",
			expected: big.NewRat(100, 1),
		},
		{
			name:     "valid zero",
			input:    "0",
			expected: big.NewRat(0, 1),
		},
		{
			name:     "valid with many decimals",
			input:    "123.456789",
			expected: big.NewRat(123456789, 1000000),
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "negative amount",
			input:       "-50.00",
			expectError: true,
		},
		{
			name:        "invalid format",
			input:       "abc",
			expectError: true,
		},
		{
			name:        "just minus sign",
			input:       "-",
			expectError: true,
		},
		{
			name:        "whitespace trimmed",
			input:       "  50.25  ",
			expected:    big.NewRat(5025, 100),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseDecimalAmount(tc.input)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 0, tc.expected.Cmp(result))
			}
		})
	}
}

func TestFactPaymentTransaction_Validate(t *testing.T) {
	base := FactPaymentTransaction{
		EventID:         "evt-123",
		PaymentID:       "payment-456",
		DeliveryID:      "delivery-789",
		UserID:          "user-123",
		Provider:        "STRIPE",
		TransactionType: TransactionAuthorization,
		Status:          "AUTHORIZED",
		Amount:          "100.50",
		Currency:        "USD",
		OccurredAt:      timeNow(),
		IngestedAt:      timeNow(),
	}

	tests := []struct {
		name        string
		tx          *FactPaymentTransaction
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid transaction",
			tx:          &base,
			expectError: false,
		},
		{
			name: "nil transaction",
			tx:          nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty event ID",
			tx:          func() *FactPaymentTransaction { t := base; t.EventID = ""; return &t }(),
			expectError: true,
			errorMsg:    "eventId is required",
		},
		{
			name: "empty payment ID",
			tx:          func() *FactPaymentTransaction { t := base; t.PaymentID = ""; return &t }(),
			expectError: true,
			errorMsg:    "paymentId is required",
		},
		{
			name: "invalid transaction type",
			tx:          func() *FactPaymentTransaction { t := base; t.TransactionType = "INVALID"; return &t }(),
			expectError: true,
			errorMsg:    "unknown event type",
		},
		{
			name: "invalid amount format",
			tx:          func() *FactPaymentTransaction { t := base; t.Amount = "abc"; return &t }(),
			expectError: true,
			errorMsg:    "invalid decimal amount format",
		},
		{
			name: "negative amount",
			tx:          func() *FactPaymentTransaction { t := base; t.Amount = "-50.00"; return &t }(),
			expectError: true,
			errorMsg:    "negative payment amount",
		},
		{
			name: "empty currency",
			tx:          func() *FactPaymentTransaction { t := base; t.Currency = ""; return &t }(),
			expectError: true,
			errorMsg:    "currency",
		},
		{
			name: "zero occurred at",
			tx:          func() *FactPaymentTransaction { t := base; t.OccurredAt = time.Time{}; return &t }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tx.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRefundExceedsCapture(t *testing.T) {
	tests := []struct {
		name         string
		refundAmount string
		capturedAmt  string
		expectError  bool
		expected     bool
	}{
		{
			name:         "refund less than capture",
			refundAmount: "50.00",
			capturedAmt:  "100.00",
			expected:     false,
		},
		{
			name:         "refund equals capture",
			refundAmount: "100.00",
			capturedAmt:  "100.00",
			expected:     false,
		},
		{
			name:         "refund exceeds capture",
			refundAmount: "150.00",
			capturedAmt:  "100.00",
			expected:     true,
		},
		{
			name:        "invalid refund amount",
			refundAmount: "abc",
			capturedAmt:  "100.00",
			expectError:  true,
		},
		{
			name:        "invalid captured amount",
			refundAmount: "50.00",
			capturedAmt:  "xyz",
			expectError:  true,
		},
		{
			name:        "negative refund amount",
			refundAmount: "-50.00",
			capturedAmt:  "100.00",
			expectError:  true,
		},
		{
			name:        "negative captured amount",
			refundAmount: "50.00",
			capturedAmt:  "-100.00",
			expectError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := RefundExceedsCapture(tc.refundAmount, tc.capturedAmt)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func timeNow() time.Time {
	return time.Now().UTC()
}