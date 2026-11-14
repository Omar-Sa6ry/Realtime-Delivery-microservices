package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	appservices "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"

	pkgsnowflake "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/snowflake"
)

type fakeE2EProvider struct {
	authorize func(ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError)
	capture   func(ports.CaptureRequest) (*ports.ProviderResult, *ports.NormalizedError)
	void      func(ports.VoidRequest) (*ports.ProviderResult, *ports.NormalizedError)
	refund    func(ports.RefundRequest) (*ports.ProviderResult, *ports.NormalizedError)
	getStatus func(string) (*ports.ProviderResult, *ports.NormalizedError)
}

func (f *fakeE2EProvider) Authorize(_ context.Context, req ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.authorize != nil {
		return f.authorize(req)
	}
	return &ports.ProviderResult{
		ProviderTransactionID: "pi_e2e_tx",
		ProviderPaymentID:     "pi_e2e_id",
		Status:                "AUTHORIZED",
		ClientSecret:          "cs_e2e_secret",
		CheckoutURL:           "https://checkout.stripe.test/pay",
	}, nil
}

func (f *fakeE2EProvider) Capture(_ context.Context, req ports.CaptureRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.capture != nil {
		return f.capture(req)
	}
	return &ports.ProviderResult{
		ProviderTransactionID: "cap_e2e_tx",
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                "SUCCEEDED",
	}, nil
}

func (f *fakeE2EProvider) Void(_ context.Context, req ports.VoidRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.void != nil {
		return f.void(req)
	}
	return &ports.ProviderResult{
		ProviderTransactionID: "void_e2e_tx",
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                "CANCELLED",
	}, nil
}

func (f *fakeE2EProvider) Refund(_ context.Context, req ports.RefundRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.refund != nil {
		return f.refund(req)
	}
	return &ports.ProviderResult{
		ProviderTransactionID: "ref_e2e_tx",
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                "SUCCEEDED",
	}, nil
}

func (f *fakeE2EProvider) GetStatus(_ context.Context, providerPaymentID string) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.getStatus != nil {
		return f.getStatus(providerPaymentID)
	}
	return &ports.ProviderResult{
		ProviderPaymentID: providerPaymentID,
		Status:            "SUCCEEDED",
	}, nil
}

func paymentColumns() []string {
	return []string{
		"id", "delivery_id", "user_id", "amount_minor", "currency", "status",
		"provider", "provider_payment_id", "gateway_session_id",
		"authorized_amount_minor", "captured_amount_minor", "refunded_amount_minor", "pending_refund_minor",
		"version", "correlation_id", "causation_id",
		"created_at", "updated_at", "authorized_at", "captured_at", "cancelled_at", "failed_at",
	}
}

func TestE2E_PaymentProcessingFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sf, err := pkgsnowflake.NewSnowflake(pkgsnowflake.Config{WorkerID: 1})
	require.NoError(t, err)

	prov := &fakeE2EProvider{}

	svc := appservices.NewPaymentService(
		pgadapter.NewPaymentRepository(db),
		pgadapter.NewRefundRepository(db),
		pgadapter.NewAttemptRepository(db),
		pgadapter.NewOutboxRepository(db),
		prov,
		nil,
		sf,
	)

	ctx := context.Background()

	// 1. Create Payment
	mock.ExpectExec(`INSERT INTO payments \(`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO payment_attempts`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE payment_attempts SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))

	createRes, err := svc.CreatePayment(ctx, appservices.CreatePaymentInput{
		DeliveryID:     "del_e2e_1",
		UserID:         "usr_e2e_1",
		AmountMinor:    5000,
		Currency:       "USD",
		IdempotencyKey: "idem_e2e_create",
	})
	require.NoError(t, err)
	require.NotNil(t, createRes)
	assert.Equal(t, domain.PaymentStatusAuthorized, createRes.Status)
	assert.Equal(t, "cs_e2e_secret", createRes.ClientSecret)
	assert.Equal(t, "https://checkout.stripe.test/pay", createRes.CheckoutURL)

	// 2. Capture Payment
	now := time.Now().UTC()
	mock.ExpectQuery(`WHERE id = \$1`).
		WithArgs(createRes.PaymentID).
		WillReturnRows(sqlmock.NewRows(paymentColumns()).AddRow(
			createRes.PaymentID, "del_e2e_1", "usr_e2e_1", 5000, "USD", string(domain.PaymentStatusAuthorized),
			"stripe", "pi_e2e_id", "cs_e2e_secret",
			5000, 0, 0, 0,
			1, "corr_1", "cause_1",
			now, now, &now, nil, nil, nil,
		))
	expectAttempt := func() {
		mock.ExpectExec(`INSERT INTO payment_attempts`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`UPDATE payment_attempts SET`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
	}
	expectAttempt()

	captured, err := svc.CapturePayment(ctx, appservices.CapturePaymentInput{
		PaymentID:   createRes.PaymentID,
		AmountMinor: 5000,
	})
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, domain.PaymentStatusCaptured, captured.Status)
	assert.Equal(t, int64(5000), captured.CapturedAmountMinor)

	// 3. Create Refund
	mock.ExpectQuery(`WHERE id = \$1`).
		WithArgs(createRes.PaymentID).
		WillReturnRows(sqlmock.NewRows(paymentColumns()).AddRow(
			createRes.PaymentID, "del_e2e_1", "usr_e2e_1", 5000, "USD", string(domain.PaymentStatusCaptured),
			"stripe", "pi_e2e_id", "cs_e2e_secret",
			5000, 5000, 0, 0,
			2, "corr_2", "cause_2",
			now, now, &now, &now, nil, nil,
		))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO refunds`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE refunds SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))

	refund, err := svc.CreateRefund(ctx, appservices.CreateRefundInput{
		PaymentID:   createRes.PaymentID,
		AmountMinor: 2500,
		Reason:      "customer_request",
	})
	require.NoError(t, err)
	require.NotNil(t, refund)
	assert.Equal(t, domain.RefundStatusCompleted, refund.Status)
	assert.Equal(t, int64(2500), refund.AmountMinor)

	assert.NoError(t, mock.ExpectationsWereMet())
}
