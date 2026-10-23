package handlers

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentHandler_Handles(t *testing.T) {
	handler := &PaymentHandler{}

	tests := []struct {
		name     string
		eventType string
		expected bool
	}{
		{"payment.created", "payment.created", true},
		{"payment.authorization.started", "payment.authorization.started", true},
		{"payment.authorized", "payment.authorized", true},
		{"payment.authorization.failed", "payment.authorization.failed", true},
		{"payment.capture.started", "payment.capture.started", true},
		{"payment.captured", "payment.captured", true},
		{"payment.completed", "payment.completed", true},
		{"payment.capture.failed", "payment.capture.failed", true},
		{"payment.cancelled", "payment.cancelled", true},
		{"payment.refund.started", "payment.refund.started", true},
		{"payment.refunded", "payment.refunded", true},
		{"payment.refund.failed", "payment.refund.failed", true},
		{"payment.failed", "payment.failed", true},
		{"delivery.created", "delivery.created", false},
		{"driver.available", "driver.available", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, handler.Handles(tc.eventType))
		})
	}
}

func TestPaymentHandler_Handle_PaymentCreated(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "payment-456")
	payload, _ := testutil.NewTestPaymentCreatedPayload("payment-456", "delivery-789")
	p := unmarshalPayload(payload)

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, p)

	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Len(t, batch.Raw, 1)
	assert.Len(t, batch.Seen, 1)
	assert.Len(t, batch.PaymentTransactions, 1)

	tx := batch.PaymentTransactions[0]
	assert.Equal(t, "evt-123", tx.EventID)
	assert.Equal(t, "payment-456", tx.PaymentID)
	assert.Equal(t, "delivery-789", tx.DeliveryID)
	assert.Equal(t, "user-123", tx.UserID)
	assert.Equal(t, "STRIPE", tx.Provider)
	assert.Equal(t, domain.TransactionAuthorization, tx.TransactionType)
	assert.Equal(t, "PENDING", tx.Status) // .started -> PENDING
	assert.Equal(t, "100.50", tx.Amount)
	assert.Equal(t, "USD", tx.Currency)
}

func TestPaymentHandler_Handle_PaymentAuthorized(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.authorized", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "100.50",
		"currency": "USD",
		"provider": "STRIPE",
		"status": "AUTHORIZED"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	tx := batch.PaymentTransactions[0]
	assert.Equal(t, domain.TransactionAuthorization, tx.TransactionType)
	assert.Equal(t, "AUTHORIZED", tx.Status)
}

func TestPaymentHandler_Handle_PaymentCaptured(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.captured", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "100.50",
		"currency": "USD",
		"provider": "STRIPE",
		"status": "CAPTURED"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	tx := batch.PaymentTransactions[0]
	assert.Equal(t, domain.TransactionCapture, tx.TransactionType)
	assert.Equal(t, "CAPTURED", tx.Status)
}

func TestPaymentHandler_Handle_PaymentRefunded(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.refunded", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "50.00",
		"currency": "USD",
		"provider": "STRIPE",
		"status": "REFUNDED"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	tx := batch.PaymentTransactions[0]
	assert.Equal(t, domain.TransactionRefund, tx.TransactionType)
	assert.Equal(t, "REFUNDED", tx.Status)
}

func TestPaymentHandler_Handle_ProviderUppercase(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "100.50",
		"currency": "USD",
		"provider": "stripe"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	tx := batch.PaymentTransactions[0]
	assert.Equal(t, "STRIPE", tx.Provider)
}

func TestPaymentHandler_Handle_DefaultProvider(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "100.50",
		"currency": "USD"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	tx := batch.PaymentTransactions[0]
	assert.Equal(t, "STRIPE", tx.Provider)
}

func TestPaymentHandler_Handle_AmountFromMinor(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amountMinor": "10050",
		"currency": "USD"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	require.NoError(t, err)
	require.NotNil(t, batch)
	tx := batch.PaymentTransactions[0]
	assert.Equal(t, "100.50", tx.Amount)
}

func TestPaymentHandler_Handle_MissingPaymentID(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "")
	payload := unmarshalPayload([]byte(`{"deliveryId": "delivery-789", "userId": "user-123"}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "paymentId")
}

func TestPaymentHandler_Handle_InvalidAmount(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "invalid",
		"currency": "USD"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
}

func TestPaymentHandler_Handle_NegativeAmount(t *testing.T) {
	handler := &PaymentHandler{}
	env := testutil.NewTestEventEnvelope("payment.created", "evt-123", "payment-456")
	payload := unmarshalPayload([]byte(`{
		"paymentId": "payment-456",
		"deliveryId": "delivery-789",
		"userId": "user-123",
		"amount": "-50.00",
		"currency": "USD"
	}`))

	ctx := context.Background()
	batch, err := handler.Handle(ctx, env, payload)

	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "negative payment amount")
}

func TestDerivePaymentStatus(t *testing.T) {
	tests := []struct {
		name     string
		eventType string
		payload  map[string]any
		expected string
	}{
		{"explicit status", "payment.created", map[string]any{"status": "AUTHORIZED"}, "AUTHORIZED"},
		{"pending .started", "payment.authorization.started", map[string]any{}, "PENDING"},
		{"failed .failed", "payment.authorization.failed", map[string]any{}, "FAILED"},
		{"cancelled .cancelled", "payment.cancelled", map[string]any{}, "CANCELLED"},
		{"authorized", "payment.authorized", map[string]any{}, "AUTHORIZED"},
		{"captured", "payment.captured", map[string]any{}, "CAPTURED"},
		{"completed", "payment.completed", map[string]any{}, "CAPTURED"},
		{"refunded", "payment.refunded", map[string]any{}, "REFUNDED"},
		{"unknown", "payment.unknown", map[string]any{}, "UNKNOWN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := derivePaymentStatus(tc.eventType, tc.payload)
			assert.Equal(t, tc.expected, result)
		})
	}
}