package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

func newPending(t *testing.T) *domain.Payment {
	t.Helper()
	p, err := domain.NewPayment("pay_1", "del_1", "usr_1", 1000, "USD", "corr", "caus")
	require.NoError(t, err)
	return p
}

func TestNewPaymentValidation(t *testing.T) {
	_, err := domain.NewPayment("", "del_1", "usr_1", 1000, "USD", "", "")
	assert.ErrorIs(t, err, domain.ErrPaymentAmountInvalid)

	_, err = domain.NewPayment("pay_1", "del_1", "usr_1", 0, "USD", "", "")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)

	_, err = domain.NewPayment("pay_1", "del_1", "usr_1", -5, "USD", "", "")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)

	_, err = domain.NewPayment("pay_1", "del_1", "usr_1", 1000, "XYZ", "", "")
	assert.ErrorIs(t, err, domain.ErrInvalidCurrency)

	p, err := domain.NewPayment("pay_1", "del_1", "usr_1", 1000, " egp ", "corr", "caus")
	require.NoError(t, err)
	assert.Equal(t, domain.CurrencyEGP, domain.Currency(p.Currency))
	assert.Equal(t, "stripe", p.Provider)
	assert.Equal(t, int64(0), p.Version)
	assert.Equal(t, domain.PaymentStatusPending, p.Status)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
}

func TestPaymentCapabilityPredicates(t *testing.T) {
	p := newPending(t)
	assert.True(t, p.CanAuthorize())
	assert.False(t, p.CanCapture())
	assert.True(t, p.CanCancel())

	require.NoError(t, p.Authorize("pi_1", "sec", 1000))
	assert.False(t, p.CanAuthorize())
	assert.True(t, p.CanCapture())
	assert.True(t, p.CanCancel())

	require.NoError(t, p.Capture(1000))
	assert.False(t, p.CanAuthorize())
	assert.False(t, p.CanCapture())
	assert.False(t, p.CanCancel())
}

func TestPaymentAuthorizeFailureAndSuccess(t *testing.T) {
	p := newPending(t)
	require.NoError(t, p.Authorize("pi_1", "sec", 1000))
	assert.Equal(t, domain.PaymentStatusAuthorized, p.Status)
	assert.Equal(t, "pi_1", p.ProviderPaymentID)
	assert.Equal(t, "sec", p.GatewaySessionID)
	assert.Equal(t, int64(1000), p.AuthorizedAmountMinor)
	require.NotNil(t, p.AuthorizedAt)
	assert.Equal(t, int64(1), p.Version)

	err := p.Authorize("pi_2", "sec2", 1000)
	assert.ErrorIs(t, err, domain.ErrPaymentInvalidStatus)
	assert.Contains(t, err.Error(), "cannot authorize from status AUTHORIZED")
}

func TestPaymentCaptureValidation(t *testing.T) {
	p := newPending(t)
	err := p.Capture(100)
	assert.ErrorIs(t, err, domain.ErrPaymentInvalidStatus)
	assert.Contains(t, err.Error(), "cannot capture from status PENDING")

	require.NoError(t, p.Authorize("pi_1", "sec", 1000))
	assert.ErrorIs(t, p.Capture(0), domain.ErrInvalidAmount)
	assert.ErrorIs(t, p.Capture(-1), domain.ErrInvalidAmount)
	assert.ErrorIs(t, p.Capture(1001), domain.ErrInvalidAmount)

	require.NoError(t, p.Capture(500))
	assert.Equal(t, domain.PaymentStatusCaptured, p.Status)
	assert.Equal(t, int64(500), p.CapturedAmountMinor)
	require.NotNil(t, p.CapturedAt)
	assert.Equal(t, int64(2), p.Version)
}

func TestPaymentCancel(t *testing.T) {
	p := newPending(t)
	require.NoError(t, p.Cancel())
	assert.Equal(t, domain.PaymentStatusCancelled, p.Status)
	require.NotNil(t, p.CancelledAt)

	err := p.Cancel()
	assert.ErrorIs(t, err, domain.ErrPaymentInvalidStatus)
	assert.Contains(t, err.Error(), "cannot cancel from status CANCELLED")

	// cancel is allowed from AUTHORIZED as well
	p2 := newPending(t)
	require.NoError(t, p2.Authorize("pi_1", "sec", 1000))
	require.NoError(t, p2.Cancel())
	assert.Equal(t, domain.PaymentStatusCancelled, p2.Status)
}

func TestPaymentFailFromEachStatus(t *testing.T) {
	for _, status := range []domain.PaymentStatus{
		domain.PaymentStatusPending, domain.PaymentStatusAuthorized,
	} {
		p := newPending(t)
		if status == domain.PaymentStatusAuthorized {
			require.NoError(t, p.Authorize("pi", "s", 1000))
		}
		require.NoError(t, p.Fail())
		assert.Equal(t, domain.PaymentStatusFailed, p.Status)
		require.NotNil(t, p.FailedAt)
	}

	terminal := []domain.PaymentStatus{
		domain.PaymentStatusCaptured, domain.PaymentStatusCancelled, domain.PaymentStatusFailed,
	}
	for _, status := range terminal {
		p := newPending(t)
		switch status {
		case domain.PaymentStatusCaptured:
			require.NoError(t, p.Authorize("pi", "s", 1000))
			require.NoError(t, p.Capture(1000))
		case domain.PaymentStatusCancelled:
			require.NoError(t, p.Cancel())
		default:
			require.NoError(t, p.Fail())
		}
		err := p.Fail()
		assert.ErrorIs(t, err, domain.ErrPaymentInvalidStatus)
		assert.Contains(t, err.Error(), "cannot fail from terminal status "+string(status))
	}
}

func TestPaymentRefundReservationLifecycle(t *testing.T) {
	p := newPending(t)
	// not captured -> cannot refund
	assert.False(t, p.CanRefund(100))
	err := p.ReserveRefund(100)
	assert.ErrorIs(t, err, domain.ErrPaymentNotRefundable)
	assert.Contains(t, err.Error(), "cannot refund 100")

	require.NoError(t, p.Authorize("pi", "s", 1000))
	require.NoError(t, p.Capture(1000))

	assert.False(t, p.CanRefund(0))
	assert.False(t, p.CanRefund(-10))
	assert.False(t, p.CanRefund(1001))
	assert.True(t, p.CanRefund(1000))

	require.NoError(t, p.ReserveRefund(400))
	assert.Equal(t, int64(400), p.PendingRefundMinor)
	assert.Equal(t, int64(3), p.Version)
	assert.False(t, p.CanRefund(700)) // 1000 captured - 0 refunded - 400 pending

	// release then re-reserve
	require.NoError(t, p.ReleaseRefundReservation(400))
	assert.Equal(t, int64(0), p.PendingRefundMinor)
	assert.ErrorIs(t, p.ReleaseRefundReservation(500), domain.ErrPaymentNotRefundable)

	require.NoError(t, p.ReserveRefund(600))
	// commit a partial refund: still not fully refunded
	require.NoError(t, p.CommitRefund(600))
	assert.Equal(t, int64(600), p.RefundedAmountMinor)
	assert.Equal(t, domain.PaymentStatusCaptured, p.Status)

	// commit more than reserved -> error
	require.NoError(t, p.ReserveRefund(400))
	require.NoError(t, p.CommitRefund(400))
	assert.Equal(t, int64(1000), p.RefundedAmountMinor)
	assert.Equal(t, domain.PaymentStatusRefunded, p.Status)

	assert.ErrorIs(t, p.CommitRefund(1), domain.ErrPaymentNotRefundable)
}

func TestPaymentIsTerminal(t *testing.T) {
	cases := map[domain.PaymentStatus]bool{
		domain.PaymentStatusPending:    false,
		domain.PaymentStatusAuthorized: false,
		domain.PaymentStatusCaptured:   true,
		domain.PaymentStatusCancelled:  true,
		domain.PaymentStatusFailed:     true,
		domain.PaymentStatusRefunded:   true,
	}
	for status, want := range cases {
		p := newPending(t)
		p.Status = status
		assert.Equal(t, want, p.IsTerminal(), string(status))
	}
}

func TestAttemptLifecycle(t *testing.T) {
	a := domain.NewAttempt("att_1", "pay_1", "AUTHORIZE", "stripe", "idem-1", 2)
	assert.Equal(t, domain.OperationStatusProcessing, a.Status)
	assert.Equal(t, "att_1", a.ID)
	assert.Equal(t, "pay_1", a.PaymentID)
	assert.Equal(t, 2, a.AttemptNumber)
	assert.Equal(t, "AUTHORIZE", a.Operation)
	assert.Equal(t, "stripe", a.Provider)
	assert.Equal(t, "idem-1", a.ProviderIdempotencyKey)
	require.NotNil(t, a.StartedAt)
	assert.Nil(t, a.CompletedAt)

	a.Succeed("ch_1")
	assert.Equal(t, domain.OperationStatusSucceeded, a.Status)
	assert.Equal(t, "ch_1", a.ProviderTransactionID)
	require.NotNil(t, a.CompletedAt)

	b := domain.NewAttempt("att_2", "pay_1", "CAPTURE", "stripe", "idem-2", 1)
	b.Fail("card_declined", "DECLINED", "safe message")
	assert.Equal(t, domain.OperationStatusFailed, b.Status)
	assert.Equal(t, "card_declined", b.ErrorCode)
	assert.Equal(t, "DECLINED", b.ErrorCategory)
	assert.Equal(t, "safe message", b.SafeErrorMessage)
	require.NotNil(t, b.CompletedAt)

	c := domain.NewAttempt("att_3", "pay_1", "VOID", "stripe", "idem-3", 1)
	c.MarkUnknown()
	assert.Equal(t, domain.OperationStatusUnknown, c.Status)
	require.NotNil(t, c.CompletedAt)

	assert.Equal(t, "Attempt[id=att_3 payment=pay_1 op=VOID status=UNKNOWN]", c.String())
}

func TestNewRefundValidation(t *testing.T) {
	_, err := domain.NewRefund("", "pay_1", "del_1", 100, "USD", "r", "k", "c", "caus")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)

	_, err = domain.NewRefund("ref_1", "pay_1", "del_1", 0, "USD", "r", "k", "c", "caus")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)

	_, err = domain.NewRefund("ref_1", "pay_1", "del_1", 100, "XYZ", "r", "k", "c", "caus")
	assert.ErrorIs(t, err, domain.ErrInvalidCurrency)

	r, err := domain.NewRefund("ref_1", "pay_1", "del_1", 100, "usd", "r", "k", "c", "caus")
	require.NoError(t, err)
	assert.Equal(t, domain.RefundStatusPending, r.Status)
	assert.Equal(t, domain.CurrencyUSD, domain.Currency(r.Currency))
	assert.Equal(t, "ref_1", r.ID)
	assert.Equal(t, "del_1", r.DeliveryID)
	assert.Equal(t, int64(100), r.AmountMinor)
	assert.Equal(t, "r", r.Reason)
	assert.Equal(t, "k", r.IdempotencyKey)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.UpdatedAt.IsZero())
}

func TestRefundLifecycle(t *testing.T) {
	r, err := domain.NewRefund("ref_1", "pay_1", "del_1", 100, "USD", "r", "k", "c", "caus")
	require.NoError(t, err)
	assert.False(t, r.IsTerminal())

	r.Complete("re_1")
	assert.Equal(t, domain.RefundStatusCompleted, r.Status)
	assert.Equal(t, "re_1", r.ProviderRefundID)
	require.NotNil(t, r.CompletedAt)
	assert.True(t, r.IsTerminal())
	assert.Equal(t, "Refund[id=ref_1 payment=pay_1 amount=100 status=REFUNDED]", r.String())

	r2, err := domain.NewRefund("ref_2", "pay_1", "del_1", 100, "USD", "r", "k", "c", "caus")
	require.NoError(t, err)
	r2.Fail("issuer rejected")
	assert.Equal(t, domain.RefundStatusFailed, r2.Status)
	assert.Equal(t, "issuer rejected", r2.FailureReason)
	require.NotNil(t, r2.FailedAt)
	assert.True(t, r2.IsTerminal())
}

func TestNewMoney(t *testing.T) {
	_, err := domain.NewMoney(0, "USD")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)
	_, err = domain.NewMoney(-1, "USD")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)
	_, err = domain.NewMoney(10, "")
	assert.ErrorIs(t, err, domain.ErrInvalidCurrency)

	m, err := domain.NewMoney(10, "USD")
	require.NoError(t, err)
	assert.Equal(t, domain.Money{AmountMinor: 10, Currency: "USD"}, m)
}

func TestMoneyAddSub(t *testing.T) {
	usd := domain.Money{AmountMinor: 100, Currency: "USD"}
	other := domain.Money{AmountMinor: 40, Currency: "USD"}
	egp := domain.Money{AmountMinor: 10, Currency: "EGP"}

	sum, err := usd.Add(other)
	require.NoError(t, err)
	assert.Equal(t, int64(140), sum.AmountMinor)

	_, err = usd.Add(egp)
	assert.EqualError(t, err, "currency mismatch: USD vs EGP")

	diff, err := usd.Sub(other)
	require.NoError(t, err)
	assert.Equal(t, int64(60), diff.AmountMinor)

	_, err = usd.Sub(egp)
	assert.EqualError(t, err, "currency mismatch: USD vs EGP")

	_, err = other.Sub(usd)
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)
}

func TestMoneyComparisons(t *testing.T) {
	a := domain.Money{AmountMinor: 0, Currency: "USD"}
	assert.True(t, a.IsZero())
	assert.False(t, domain.Money{AmountMinor: 1, Currency: "USD"}.IsZero())

	usd100 := domain.Money{AmountMinor: 100, Currency: "USD"}
	usd50 := domain.Money{AmountMinor: 50, Currency: "USD"}
	egp100 := domain.Money{AmountMinor: 100, Currency: "EGP"}

	assert.True(t, usd50.LessThanOrEqual(usd100))
	assert.True(t, usd100.LessThanOrEqual(usd100))
	assert.False(t, usd100.LessThanOrEqual(usd50))
	assert.False(t, usd100.LessThanOrEqual(egp100))

	assert.True(t, usd100.GreaterThan(usd50))
	assert.False(t, usd50.GreaterThan(usd100))
	assert.False(t, usd100.GreaterThan(egp100))

	assert.True(t, usd100.Equal(domain.Money{AmountMinor: 100, Currency: "USD"}))
	assert.False(t, usd100.Equal(usd50))
	assert.False(t, usd100.Equal(egp100))

	assert.Equal(t, "100 USD", usd100.String())
}

func TestCurrencyValidation(t *testing.T) {
	for _, c := range []domain.Currency{
		domain.CurrencyEGP, domain.CurrencyUSD, domain.CurrencyEUR,
		domain.CurrencySAR, domain.CurrencyAED,
	} {
		assert.True(t, c.IsValid())
	}
	assert.False(t, domain.Currency("XYZ").IsValid())
	assert.False(t, domain.Currency("").IsValid())

	parsed, err := domain.ParseCurrency(" usd ")
	require.NoError(t, err)
	assert.Equal(t, domain.CurrencyUSD, parsed)

	_, err = domain.ParseCurrency("xyz")
	assert.ErrorIs(t, err, domain.ErrInvalidCurrency)
	assert.Contains(t, err.Error(), "'xyz'")
}

func TestPaymentStatusValidity(t *testing.T) {
	for _, s := range []domain.PaymentStatus{
		domain.PaymentStatusPending, domain.PaymentStatusAuthorized, domain.PaymentStatusCaptured,
		domain.PaymentStatusCancelled, domain.PaymentStatusFailed, domain.PaymentStatusRefunded,
	} {
		assert.True(t, s.IsValid())
	}
	assert.False(t, domain.PaymentStatus("BOGUS").IsValid())
}

func TestRefundStatusValidity(t *testing.T) {
	for _, s := range []domain.RefundStatus{
		domain.RefundStatusPending, domain.RefundStatusCompleted, domain.RefundStatusFailed,
	} {
		assert.True(t, s.IsValid())
	}
	assert.False(t, domain.RefundStatus("BOGUS").IsValid())
}

func TestAllowedTransitionsTable(t *testing.T) {
	// data table is part of the public contract
	assert.Equal(t,
		[]domain.PaymentStatus{domain.PaymentStatusAuthorized, domain.PaymentStatusCancelled, domain.PaymentStatusFailed},
		domain.AllowedTransitions[domain.PaymentStatusPending])
	assert.Equal(t,
		[]domain.PaymentStatus{domain.PaymentStatusCaptured, domain.PaymentStatusCancelled, domain.PaymentStatusFailed},
		domain.AllowedTransitions[domain.PaymentStatusAuthorized])
	assert.Equal(t,
		[]domain.PaymentStatus{domain.PaymentStatusRefunded},
		domain.AllowedTransitions[domain.PaymentStatusCaptured])
	assert.Empty(t, domain.AllowedTransitions[domain.PaymentStatusCancelled])
	assert.Empty(t, domain.AllowedTransitions[domain.PaymentStatusFailed])
	assert.Empty(t, domain.AllowedTransitions[domain.PaymentStatusRefunded])
}

func TestErrorValuesAreStable(t *testing.T) {
	assert.Equal(t, "payment not found", domain.ErrPaymentNotFound.Error())
	assert.Equal(t, "provider outcome is unknown — reconciliation required", domain.ErrProviderUnknownOutcome.Error())
	assert.Equal(t, "concurrent modification detected — retry the operation", domain.ErrConcurrentModification.Error())
	assert.Equal(t, "refund not found", domain.ErrRefundNotFound.Error())
}
