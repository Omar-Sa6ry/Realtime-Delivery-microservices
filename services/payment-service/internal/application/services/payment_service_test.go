package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	natsadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/nats"
	pgadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"

	pkgevents "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	pkgsnowflake "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/snowflake"
)

// ---------------------------------------------------------------------------
// fakes + harness
// ---------------------------------------------------------------------------

type fakeProvider struct {
	authorize func(ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError)
	capture   func(ports.CaptureRequest) (*ports.ProviderResult, *ports.NormalizedError)
	void      func(ports.VoidRequest) (*ports.ProviderResult, *ports.NormalizedError)
	refund    func(ports.RefundRequest) (*ports.ProviderResult, *ports.NormalizedError)
	getStatus func(string) (*ports.ProviderResult, *ports.NormalizedError)
}

func (f *fakeProvider) Authorize(_ context.Context, req ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.authorize != nil {
		return f.authorize(req)
	}
	return &ports.ProviderResult{
		ProviderTransactionID: "pi_ok",
		ProviderPaymentID:     "pi_ok",
		Status:                "AUTHORIZED",
		ClientSecret:          "sec_ok",
		CheckoutURL:           "https://checkout.test/pay",
	}, nil
}

func (f *fakeProvider) Capture(_ context.Context, req ports.CaptureRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.capture != nil {
		return f.capture(req)
	}
	return &ports.ProviderResult{ProviderTransactionID: "cap_ok", ProviderPaymentID: req.ProviderPaymentID, Status: "SUCCEEDED"}, nil
}

func (f *fakeProvider) Void(_ context.Context, req ports.VoidRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.void != nil {
		return f.void(req)
	}
	return &ports.ProviderResult{ProviderTransactionID: "void_ok", ProviderPaymentID: req.ProviderPaymentID, Status: "CANCELLED"}, nil
}

func (f *fakeProvider) Refund(_ context.Context, req ports.RefundRequest) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.refund != nil {
		return f.refund(req)
	}
	return &ports.ProviderResult{ProviderTransactionID: "re_ok", ProviderPaymentID: req.ProviderPaymentID, Status: "SUCCEEDED"}, nil
}

func (f *fakeProvider) GetStatus(_ context.Context, providerPaymentID string) (*ports.ProviderResult, *ports.NormalizedError) {
	if f.getStatus != nil {
		return f.getStatus(providerPaymentID)
	}
	return &ports.ProviderResult{ProviderPaymentID: providerPaymentID, Status: "SUCCEEDED"}, nil
}

type harness struct {
	svc   *PaymentService
	mock  sqlmock.Sqlmock
	prov  *fakeProvider
	sf    *pkgsnowflake.Snowflake
}

func newHarness(t *testing.T, provider ports.PaymentProvider) *harness {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sf, err := pkgsnowflake.NewSnowflake(pkgsnowflake.Config{WorkerID: 1})
	require.NoError(t, err)

	p, ok := provider.(*fakeProvider)
	if !ok {
		p = &fakeProvider{}
	}

	svc := NewPaymentService(
		pgadapter.NewPaymentRepository(db),
		pgadapter.NewRefundRepository(db),
		pgadapter.NewAttemptRepository(db),
		pgadapter.NewOutboxRepository(db),
		provider,
		nil,
		sf,
	)
	return &harness{svc: svc, mock: mock, prov: p, sf: sf}
}

func (h *harness) withPublisher(t *testing.T) {
	t.Helper()
	opts := &server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("embedded nats server did not start")
	}
	t.Cleanup(ns.Shutdown)

	pub, err := natsadapter.NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(pub.Close)
	h.svc.nats = pub
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

func paymentRow(status string, amount, authorized, captured, refunded, pending, version int64) *sqlmock.Rows {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ts := now
	return sqlmock.NewRows(paymentColumns()).AddRow(
		"pay_1", "del_1", "usr_1", amount, "USD", status,
		"stripe", "pi_db", "cs_db",
		authorized, captured, refunded, pending,
		version, "corr_1", "cause_1",
		now, now, &ts, nil, nil, nil,
	)
}

func (h *harness) expectPaymentCreate() {
	h.mock.ExpectExec(`INSERT INTO payments \(`).WillReturnResult(sqlmock.NewResult(1, 1))
}

func (h *harness) expectOutboxInsert() {
	h.mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
}

func (h *harness) expectAttemptCreate() {
	h.mock.ExpectExec(`INSERT INTO payment_attempts`).WillReturnResult(sqlmock.NewResult(1, 1))
}

func (h *harness) expectAttemptUpdate() {
	h.mock.ExpectExec(`UPDATE payment_attempts SET`).WillReturnResult(sqlmock.NewResult(0, 1))
}

func (h *harness) expectPaymentUpdate() {
	h.mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
}

func (h *harness) expectFindByID(status string, amount, authorized, captured, refunded, pending, version int64) {
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(paymentRow(status, amount, authorized, captured, refunded, pending, version))
}

// ---------------------------------------------------------------------------
// CreatePayment
// ---------------------------------------------------------------------------

func TestCreatePaymentSuccess(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectPaymentCreate()
	h.expectOutboxInsert()
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()

	res, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{
		DeliveryID:     "del_1",
		UserID:         "usr_1",
		AmountMinor:    5000,
		Currency:       "USD",
		IdempotencyKey: "idem-1",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotEmpty(t, res.PaymentID)
	assert.Equal(t, domain.PaymentStatusAuthorized, res.Status)
	assert.Equal(t, "sec_ok", res.ClientSecret)
	assert.Equal(t, "https://checkout.test/pay", res.CheckoutURL)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentPublishesRealtimeNotification(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.withPublisher(t)
	h.expectPaymentCreate()
	h.expectOutboxInsert()
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()

	res, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{
		DeliveryID:  "del_1",
		UserID:      "usr_1",
		AmountMinor: 5000,
		Currency:    "USD",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentRejectsInvalidInput(t *testing.T) {
	h := newHarness(t, &fakeProvider{})

	_, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 0, Currency: "USD"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid input")

	_, err = h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "XXX"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid input")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentPersistFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectExec(`INSERT INTO payments \(`).WillReturnError(errors.New("insert failed"))

	_, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "USD"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "create_payment: persist")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentOutboxFailureIsNonFatal(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectPaymentCreate()
	h.mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnError(errors.New("outbox down"))
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnError(errors.New("outbox down"))

	res, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "USD"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentAttemptPersistFailureIsNonFatal(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectPaymentCreate()
	h.expectOutboxInsert()
	h.mock.ExpectExec(`INSERT INTO payment_attempts`).WillReturnError(errors.New("attempt down"))
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()

	res, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "USD"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentProviderTimeout(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		authorize: func(ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryTimeout, Message: "timeout"}
		},
	})
	h.expectPaymentCreate()
	h.expectOutboxInsert()
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	res, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "USD"})
	require.ErrorIs(t, err, domain.ErrProviderUnknownOutcome)
	require.NotNil(t, res)
	assert.Equal(t, domain.PaymentStatusPending, res.Status)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentProviderPermanentFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		authorize: func(ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryDeclined, Message: "declined"}
		},
	})
	h.expectPaymentCreate()
	h.expectOutboxInsert()
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()

	res, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "USD"})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorContains(t, err, "create_payment: authorization failed")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreatePaymentUpdateFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectPaymentCreate()
	h.expectOutboxInsert()
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.mock.ExpectExec(`version=\$15`).WillReturnError(errors.New("conflict"))

	_, err := h.svc.CreatePayment(t.Context(), CreatePaymentInput{AmountMinor: 100, Currency: "USD"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "create_payment: update payment")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CapturePayment
// ---------------------------------------------------------------------------

func TestCapturePaymentSuccess(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 3)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()
	h.expectOutboxInsert()

	payment, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{
		PaymentID:   "pay_1",
		AmountMinor: 5000,
	})
	require.NoError(t, err)
	require.NotNil(t, payment)
	assert.Equal(t, domain.PaymentStatusCaptured, payment.Status)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCapturePaymentNotFound(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(sqlmock.NewRows(paymentColumns()))

	_, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{PaymentID: "pay_1", AmountMinor: 100})
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCapturePaymentNotCaptureable(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)

	_, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{PaymentID: "pay_1", AmountMinor: 100})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPaymentNotCaptureable)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCapturePaymentAmountExceedsAuthorization(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("AUTHORIZED", 5000, 1000, 0, 0, 0, 2)
	h.expectAttemptCreate()

	_, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{PaymentID: "pay_1", AmountMinor: 5000})
	require.ErrorIs(t, err, domain.ErrInvalidAmount)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCapturePaymentProviderTimeout(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		capture: func(ports.CaptureRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryTimeout, Message: "timeout"}
		},
	})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 2)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	_, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{PaymentID: "pay_1", AmountMinor: 100})
	require.ErrorIs(t, err, domain.ErrProviderUnknownOutcome)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCapturePaymentProviderFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		capture: func(ports.CaptureRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryPermanent, Message: "nope"}
		},
	})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 2)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	_, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{PaymentID: "pay_1", AmountMinor: 100})
	require.Error(t, err)
	assert.ErrorContains(t, err, "capture failed")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCapturePaymentUpdateFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 2)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.mock.ExpectExec(`version=\$15`).WillReturnError(errors.New("conflict"))

	_, err := h.svc.CapturePayment(t.Context(), CapturePaymentInput{PaymentID: "pay_1", AmountMinor: 5000})
	require.Error(t, err)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// AuthorizePayment
// ---------------------------------------------------------------------------

func TestAuthorizePaymentSuccess(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()

	payment, err := h.svc.AuthorizePayment(t.Context(), AuthorizePaymentInput{PaymentID: "pay_1", IdempotencyKey: "idem"})
	require.NoError(t, err)
	require.NotNil(t, payment)
	assert.Equal(t, domain.PaymentStatusAuthorized, payment.Status)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestAuthorizePaymentNotFound(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(sqlmock.NewRows(paymentColumns()))

	_, err := h.svc.AuthorizePayment(t.Context(), AuthorizePaymentInput{PaymentID: "pay_1"})
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestAuthorizePaymentInvalidStatus(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 1)

	_, err := h.svc.AuthorizePayment(t.Context(), AuthorizePaymentInput{PaymentID: "pay_1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPaymentInvalidStatus)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestAuthorizePaymentProviderTimeout(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		authorize: func(ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryTimeout}
		},
	})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	_, err := h.svc.AuthorizePayment(t.Context(), AuthorizePaymentInput{PaymentID: "pay_1"})
	require.ErrorIs(t, err, domain.ErrProviderUnknownOutcome)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestAuthorizePaymentProviderFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		authorize: func(ports.AuthorizeRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryDeclined, Message: "declined"}
		},
	})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	_, err := h.svc.AuthorizePayment(t.Context(), AuthorizePaymentInput{PaymentID: "pay_1"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "authorization failed")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestAuthorizePaymentUpdateFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.mock.ExpectExec(`version=\$15`).WillReturnError(errors.New("conflict"))

	_, err := h.svc.AuthorizePayment(t.Context(), AuthorizePaymentInput{PaymentID: "pay_1"})
	require.Error(t, err)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CancelAuthorization
// ---------------------------------------------------------------------------

func TestCancelAuthorizationSuccess(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 2)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()

	payment, err := h.svc.CancelAuthorization(t.Context(), "pay_1", "idem")
	require.NoError(t, err)
	require.NotNil(t, payment)
	assert.Equal(t, domain.PaymentStatusCancelled, payment.Status)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCancelAuthorizationNotFound(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(sqlmock.NewRows(paymentColumns()))

	_, err := h.svc.CancelAuthorization(t.Context(), "pay_1", "")
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCancelAuthorizationInvalidStatus(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("CAPTURED", 5000, 5000, 5000, 0, 0, 3)

	_, err := h.svc.CancelAuthorization(t.Context(), "pay_1", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPaymentInvalidStatus)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCancelAuthorizationProviderTimeout(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		void: func(ports.VoidRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryTimeout}
		},
	})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 1)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	_, err := h.svc.CancelAuthorization(t.Context(), "pay_1", "")
	require.ErrorIs(t, err, domain.ErrProviderUnknownOutcome)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCancelAuthorizationProviderFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		void: func(ports.VoidRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryPermanent, Message: "nope"}
		},
	})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 1)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()

	_, err := h.svc.CancelAuthorization(t.Context(), "pay_1", "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "void failed")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCancelAuthorizationUpdateFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("AUTHORIZED", 5000, 5000, 0, 0, 0, 1)
	h.expectAttemptCreate()
	h.expectAttemptUpdate()
	h.mock.ExpectExec(`version=\$15`).WillReturnError(errors.New("conflict"))

	_, err := h.svc.CancelAuthorization(t.Context(), "pay_1", "")
	require.Error(t, err)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateRefund
// ---------------------------------------------------------------------------

func (h *harness) expectRefundCreate() {
	h.mock.ExpectExec(`INSERT INTO refunds`).WillReturnResult(sqlmock.NewResult(1, 1))
}

func (h *harness) expectRefundUpdate() {
	h.mock.ExpectExec(`UPDATE refunds SET`).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestCreateRefundSuccess(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("CAPTURED", 5000, 5000, 5000, 0, 0, 4)
	h.expectPaymentUpdate()
	h.expectRefundCreate()
	h.expectRefundUpdate()
	h.expectPaymentUpdate()
	h.expectOutboxInsert()

	refund, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{
		PaymentID:   "pay_1",
		AmountMinor: 1000,
		Reason:      "duplicate",
	})
	require.NoError(t, err)
	require.NotNil(t, refund)
	assert.Equal(t, domain.RefundStatusCompleted, refund.Status)
	assert.Equal(t, "re_ok", refund.ProviderRefundID)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreateRefundNotFound(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(sqlmock.NewRows(paymentColumns()))

	_, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{PaymentID: "pay_1", AmountMinor: 100})
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreateRefundNotRefundable(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)

	_, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{PaymentID: "pay_1", AmountMinor: 100})
	require.ErrorIs(t, err, domain.ErrPaymentNotRefundable)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreateRefundReserveConflict(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("CAPTURED", 5000, 5000, 5000, 0, 0, 4)
	h.mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{PaymentID: "pay_1", AmountMinor: 100})
	require.Error(t, err)
	assert.ErrorContains(t, err, "create_refund: reserve refund concurrency conflict")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreateRefundInvalidCurrencyReleasesReservation(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	now := time.Now()
	ts := now
	rows := sqlmock.NewRows(paymentColumns()).AddRow(
		"pay_1", "del_1", "usr_1", int64(5000), "XXX", "CAPTURED",
		"stripe", "pi_db", "cs_db",
		int64(5000), int64(5000), int64(0), int64(0),
		int64(4), "corr", "cause",
		now, now, &ts, nil, nil, nil,
	)
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(rows)
	h.expectPaymentUpdate()
	h.expectPaymentUpdate()

	_, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{PaymentID: "pay_1", AmountMinor: 100})
	require.Error(t, err)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreateRefundPersistFailureReleasesReservation(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("CAPTURED", 5000, 5000, 5000, 0, 0, 4)
	h.expectPaymentUpdate()
	h.mock.ExpectExec(`INSERT INTO refunds`).WillReturnError(errors.New("insert failed"))
	h.expectPaymentUpdate()

	_, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{PaymentID: "pay_1", AmountMinor: 100})
	require.Error(t, err)
	assert.ErrorContains(t, err, "create_refund: persist refund")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestCreateRefundProviderFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{
		refund: func(ports.RefundRequest) (*ports.ProviderResult, *ports.NormalizedError) {
			return nil, &ports.NormalizedError{Category: ports.ErrCategoryDeclined, Message: "nope"}
		},
	})
	h.expectFindByID("CAPTURED", 5000, 5000, 5000, 0, 0, 4)
	h.expectPaymentUpdate()
	h.expectRefundCreate()
	h.expectRefundUpdate()
	h.expectPaymentUpdate()

	_, err := h.svc.CreateRefund(t.Context(), CreateRefundInput{PaymentID: "pay_1", AmountMinor: 100})
	require.Error(t, err)
	assert.ErrorContains(t, err, "refund failed")
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// reads + helpers
// ---------------------------------------------------------------------------

func TestGetPaymentAndStatusAndList(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)
	payment, err := h.svc.GetPayment(t.Context(), "pay_1")
	require.NoError(t, err)
	require.NotNil(t, payment)

	h.expectFindByID("PENDING", 5000, 0, 0, 0, 0, 0)
	status, err := h.svc.GetPaymentStatus(t.Context(), "pay_1")
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, status)

	h.mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	h.mock.ExpectQuery(`LIMIT \$1 OFFSET \$2`).WillReturnRows(paymentRow("PENDING", 5000, 0, 0, 0, 0, 0))
	list, total, err := h.svc.ListPayments(t.Context(), 1, 20, "")
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, 1, total)

	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestGetPaymentErrors(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(sqlmock.NewRows(paymentColumns()))
	_, err := h.svc.GetPayment(t.Context(), "pay_1")
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)

	h.mock.ExpectQuery(`WHERE id = \$1`).WillReturnRows(sqlmock.NewRows(paymentColumns()))
	_, err = h.svc.GetPaymentStatus(t.Context(), "pay_1")
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestPaymentRepositoryAccessor(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	require.NotNil(t, h.svc.PaymentRepository())
}

func TestNewPaymentServiceKeepsDependencies(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	assert.Nil(t, h.svc.nats)
	require.NotNil(t, h.svc.provider)
	require.NotNil(t, h.svc.snowflake)
}

func TestPublishEventMarshalFailure(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	err := h.svc.publishEvent(t.Context(), pkgevents.PaymentCreated, "pay_1", make(chan int))
	require.Error(t, err)
	assert.ErrorContains(t, err, "publishEvent: marshal")
}

func TestPublishEventDelegatesToOutbox(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.expectOutboxInsert()
	require.NoError(t, h.svc.publishEvent(t.Context(), pkgevents.PaymentCreated, "pay_1", pkgevents.PaymentCreatedPayload{PaymentID: "pay_1"}))
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestPublishEventOutboxError(t *testing.T) {
	h := newHarness(t, &fakeProvider{})
	h.mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnError(errors.New("down"))
	require.Error(t, h.svc.publishEvent(t.Context(), pkgevents.PaymentCreated, "pay_1", pkgevents.PaymentCreatedPayload{}))
	assert.NoError(t, h.mock.ExpectationsWereMet())
}

func TestPublishRealtimeMarshalsPayload(t *testing.T) {
	// nil publisher: nothing to do
	h := newHarness(t, &fakeProvider{})
	h.svc.publishRealtime(t.Context(), "pay_1", "del_1", "usr_1", "PENDING")

	// non-nil publisher: payload is marshalled and published
	h.withPublisher(t)
	h.svc.publishRealtime(t.Context(), "pay_1", "del_1", "usr_1", "PENDING")

	// the marshalled shape stays stable for realtime consumers
	data, err := json.Marshal(natsadapter.PaymentStatusUpdate{PaymentID: "pay_1", Status: "PENDING"})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"paymentId":"pay_1"`)
}
