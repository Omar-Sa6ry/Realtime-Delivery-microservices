package stripe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripego "github.com/stripe/stripe-go/v78"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

// serveStripe points the package-global Stripe backend at a fake server for the
// duration of the test and restores the real one afterwards.
func serveStripe(t *testing.T, h http.HandlerFunc, timeout time.Duration) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	origBackend := stripego.GetBackend(stripego.APIBackend)
	origKey := stripego.Key
	stripego.Key = "sk_test_fake"

	cfg := &stripego.BackendConfig{
		URL:               stripego.String(srv.URL),
		MaxNetworkRetries: stripego.Int64(0),
	}
	if timeout > 0 {
		cfg.HTTPClient = &http.Client{Timeout: timeout}
	}
	stripego.SetBackend(stripego.APIBackend, stripego.GetBackendWithConfig(stripego.APIBackend, cfg))

	t.Cleanup(func() {
		stripego.SetBackend(stripego.APIBackend, origBackend)
		stripego.Key = origKey
	})
}

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}

func writeError(w http.ResponseWriter, code int, typ, errCode string) {
	writeJSON(w, code, fmt.Sprintf(`{"error":{"type":%q,"code":%q,"message":"stripe failure"}}`, typ, errCode))
}

func TestNewStripeProviderEmptyKeyStillConstructs(t *testing.T) {
	p := NewStripeProvider("")
	require.NotNil(t, p)
	assert.Empty(t, p.secretKey)

	p2 := NewStripeProvider("sk_live_x")
	assert.Equal(t, "sk_live_x", p2.secretKey)
}

func TestNormalizePaymentIntentStatus(t *testing.T) {
	cases := map[stripego.PaymentIntentStatus]string{
		stripego.PaymentIntentStatusRequiresPaymentMethod: "PROCESSING",
		stripego.PaymentIntentStatusRequiresConfirmation:  "PROCESSING",
		stripego.PaymentIntentStatusRequiresAction:        "PROCESSING",
		stripego.PaymentIntentStatusProcessing:            "PROCESSING",
		stripego.PaymentIntentStatusRequiresCapture:       "AUTHORIZED",
		stripego.PaymentIntentStatusSucceeded:             "SUCCEEDED",
		stripego.PaymentIntentStatusCanceled:              "CANCELLED",
		"made_up_status":                                  "UNKNOWN",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizePaymentIntentStatus(in), in)
	}
}

func TestNormalizeRefundStatus(t *testing.T) {
	cases := map[stripego.RefundStatus]string{
		stripego.RefundStatusSucceeded: "SUCCEEDED",
		stripego.RefundStatusPending:   "PROCESSING",
		stripego.RefundStatusFailed:    "FAILED",
		stripego.RefundStatusCanceled:  "FAILED",
		"made_up_status":               "UNKNOWN",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeRefundStatus(in), in)
	}
}

func TestRefundReasonToStripe(t *testing.T) {
	cases := map[string]string{
		"duplicate":             "duplicate",
		"fraudulent":            "fraudulent",
		"FRAUD":                 "fraudulent",
		"requested_by_customer": "requested_by_customer",
		"customer_request":      "requested_by_customer",
		"anything_else":         "requested_by_customer",
		"":                      "requested_by_customer",
	}
	for in, want := range cases {
		assert.Equal(t, want, refundReasonToStripe(in), in)
	}
}

func TestNormalizeStripeErrorNil(t *testing.T) {
	assert.Nil(t, normalizeStripeError(nil, "Authorize"))
}

func TestNormalizeStripeErrorDeadline(t *testing.T) {
	nerr := normalizeStripeError(fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "Authorize")
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryTimeout, nerr.Category)
	assert.False(t, nerr.Retryable)
	assert.Contains(t, nerr.Message, "timed out")
	assert.Contains(t, nerr.Message, "Authorize")
}

func TestNormalizeStripeErrorNonStripe(t *testing.T) {
	nerr := normalizeStripeError(errors.New("connection reset"), "Capture")
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryUnknown, nerr.Category)
	assert.False(t, nerr.Retryable)
	assert.Contains(t, nerr.Message, "non-stripe error")
}

func TestNormalizeStripeErrorByType(t *testing.T) {
	cases := []struct {
		typ       stripego.ErrorType
		category  ports.ErrorCategory
		retryable bool
	}{
		{stripego.ErrorTypeCard, ports.ErrCategoryDeclined, false},
		{"rate_limit_error", ports.ErrCategoryRateLimited, true},
		{stripego.ErrorTypeInvalidRequest, ports.ErrCategoryPermanent, false},
		{"authentication_error", ports.ErrCategoryAuthError, false},
		{stripego.ErrorTypeAPI, ports.ErrCategoryTemporary, true},
		{"api_connection_error", ports.ErrCategoryTemporary, true},
	}
	for _, tc := range cases {
		nerr := normalizeStripeError(&stripego.Error{Type: tc.typ}, "Authorize")
		require.NotNil(t, nerr, tc.typ)
		assert.Equal(t, tc.category, nerr.Category, tc.typ)
		assert.Equal(t, tc.retryable, nerr.Retryable, tc.typ)
		assert.Contains(t, nerr.Message, "type=")
	}
}

func TestNormalizeStripeErrorFallbackCodes(t *testing.T) {
	codes := []stripego.ErrorCode{
		stripego.ErrorCodeCardDeclined,
		stripego.ErrorCodeExpiredCard,
		stripego.ErrorCodeIncorrectCVC,
		stripego.ErrorCodeIncorrectNumber,
		stripego.ErrorCodeIncorrectZip,
		stripego.ErrorCodeInsufficientFunds,
	}
	for _, code := range codes {
		nerr := normalizeStripeError(&stripego.Error{Type: "some_new_type", Code: code}, "Refund")
		require.NotNil(t, nerr, code)
		assert.Equal(t, ports.ErrCategoryDeclined, nerr.Category, code)
		assert.False(t, nerr.Retryable, code)
	}

	nerr := normalizeStripeError(&stripego.Error{Type: "some_new_type", Code: "unknown_code"}, "Refund")
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryUnknown, nerr.Category)
	assert.False(t, nerr.Retryable)
}

func TestAuthorizeCreatesCheckoutSession(t *testing.T) {
	var hitSession bool
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents":
			writeJSON(w, http.StatusOK, `{"id":"pi_abc","status":"requires_payment_method","client_secret":"sec_abc"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
			hitSession = true
			writeJSON(w, http.StatusOK, `{"id":"cs_1","url":"https://checkout.stripe.com/pay/cs_1","payment_intent":{"id":"pi_abc"}}`)
		default:
			writeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing")
		}
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{
		PaymentID:      "pay_1",
		AmountMinor:    5000,
		Currency:       "USD",
		IdempotencyKey: "",
	})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.True(t, hitSession)
	assert.Equal(t, "https://checkout.stripe.com/pay/cs_1", res.CheckoutURL)
	assert.Equal(t, "PROCESSING", res.Status)
	assert.Equal(t, "pi_abc", res.ProviderTransactionID)
	assert.Equal(t, "pi_abc", res.ProviderPaymentID)
	assert.Equal(t, "sec_abc", res.ClientSecret)
}

func TestAuthorizeWithExplicitURLsAndIdempotencyKey(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents":
			assert.Equal(t, "Bearer sk_test_fake", r.Header.Get("Authorization"))
			writeJSON(w, http.StatusOK, `{"id":"pi_2","status":"requires_payment_method"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
			_ = r.ParseForm()
			assert.Equal(t, "https://merchant.test/ok", r.FormValue("success_url"))
			assert.Equal(t, "https://merchant.test/bad", r.FormValue("cancel_url"))
			writeJSON(w, http.StatusOK, `{"id":"cs_2","url":"https://checkout.stripe.com/pay/cs_2"}`)
		default:
			writeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing")
		}
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{
		PaymentID:      "pay_2",
		AmountMinor:    100,
		Currency:       "eur",
		IdempotencyKey: "idem-1",
		SuccessURL:     "https://merchant.test/ok",
		CancelURL:      "https://merchant.test/bad",
	})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Equal(t, "https://checkout.stripe.com/pay/cs_2", res.CheckoutURL)
	assert.Equal(t, "PROCESSING", res.Status)
	assert.Equal(t, "pi_2", res.ProviderTransactionID)
}

func TestAuthorizeSessionFailureFallsBackToNormalizedStatus(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents":
			writeJSON(w, http.StatusOK, `{"id":"pi_3","status":"requires_capture"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
			writeError(w, http.StatusInternalServerError, "api_error", "internal")
		default:
			writeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing")
		}
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{PaymentID: "pay_3", AmountMinor: 1, Currency: "USD"})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Empty(t, res.CheckoutURL)
	assert.Equal(t, "AUTHORIZED", res.Status)
	assert.Equal(t, "pi_3", res.ProviderTransactionID)
}

func TestAuthorizeSkipsSessionWhenRedirectURLPresent(t *testing.T) {
	var sessionHit bool
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents":
			writeJSON(w, http.StatusOK, `{"id":"pi_4","status":"requires_action","next_action":{"type":"redirect_to_url","redirect_to_url":{"url":"https://stripe.test/auth/pi_4"}}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
			sessionHit = true
			writeJSON(w, http.StatusOK, `{"id":"cs_4","url":"https://checkout.stripe.com/pay/cs_4"}`)
		default:
			writeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing")
		}
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{PaymentID: "pay_4", AmountMinor: 1, Currency: "USD"})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.False(t, sessionHit)
	assert.Equal(t, "https://stripe.test/auth/pi_4", res.CheckoutURL)
	assert.Equal(t, "PROCESSING", res.Status)
}

func TestAuthorizeReturnsNormalizedAPIError(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusPaymentRequired, "card_error", "card_declined")
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{PaymentID: "pay_5", AmountMinor: 1, Currency: "USD"})
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryDeclined, nerr.Category)
	assert.False(t, nerr.Retryable)
}

func TestAuthorizeTimeoutProducesTimeoutCategory(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		writeJSON(w, http.StatusOK, `{"id":"pi_slow","status":"requires_capture"}`)
	}, 50*time.Millisecond)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{PaymentID: "pay_6", AmountMinor: 1, Currency: "USD"})
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryTimeout, nerr.Category)
}

func TestAuthorizeMalformedResponseProducesUnknownCategory(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this-is-not-json"))
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Authorize(context.Background(), ports.AuthorizeRequest{PaymentID: "pay_7", AmountMinor: 1, Currency: "USD"})
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryUnknown, nerr.Category)
	assert.Contains(t, nerr.Message, "non-stripe error")
}

func TestCaptureSuccessAndError(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/capture"))
		writeJSON(w, http.StatusOK, `{"id":"pi_cap","status":"succeeded"}`)
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Capture(context.Background(), ports.CaptureRequest{ProviderPaymentID: "pi_cap", AmountMinor: 500})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Equal(t, "SUCCEEDED", res.Status)
	assert.Equal(t, "pi_cap", res.ProviderTransactionID)

	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "parameter_missing")
	}, 0)

	res, nerr = p.Capture(context.Background(), ports.CaptureRequest{ProviderPaymentID: "pi_cap", AmountMinor: 500, IdempotencyKey: "k1"})
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryPermanent, nerr.Category)
}

func TestVoidSuccessAndError(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/cancel"))
		writeJSON(w, http.StatusOK, `{"id":"pi_void","status":"canceled"}`)
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Void(context.Background(), ports.VoidRequest{ProviderPaymentID: "pi_void", IdempotencyKey: "k2"})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Equal(t, "CANCELLED", res.Status)

	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusInternalServerError, "api_error", "internal")
	}, 0)

	res, nerr = p.Void(context.Background(), ports.VoidRequest{ProviderPaymentID: "pi_void"})
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryTemporary, nerr.Category)
	assert.True(t, nerr.Retryable)
}

func TestRefundSuccessWithAndWithoutPaymentIntent(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/refunds", r.URL.Path)
		writeJSON(w, http.StatusOK, `{"id":"re_1","status":"succeeded","payment_intent":"pi_ref"}`)
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.Refund(context.Background(), ports.RefundRequest{
		ProviderPaymentID: "pi_ref",
		AmountMinor:       250,
		Reason:            "duplicate",
		IdempotencyKey:    "k3",
	})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Equal(t, "SUCCEEDED", res.Status)
	assert.Equal(t, "re_1", res.ProviderTransactionID)
	assert.Equal(t, "pi_ref", res.ProviderPaymentID)

	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"id":"re_2","status":"failed","payment_intent":null}`)
	}, 0)

	res, nerr = p.Refund(context.Background(), ports.RefundRequest{ProviderPaymentID: "pi_ref", AmountMinor: 1, Reason: "other"})
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Equal(t, "FAILED", res.Status)
	assert.Equal(t, "", res.ProviderPaymentID)

	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusInternalServerError, "rate_limit_error", "slow_down")
	}, 0)

	res, nerr = p.Refund(context.Background(), ports.RefundRequest{ProviderPaymentID: "pi_ref", AmountMinor: 1})
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryRateLimited, nerr.Category)
	assert.True(t, nerr.Retryable)
}

func TestGetStatusSuccessAndError(t *testing.T) {
	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		writeJSON(w, http.StatusOK, `{"id":"pi_st","status":"processing","client_secret":"sec_st"}`)
	}, 0)

	p := NewStripeProvider("sk_test_fake")
	res, nerr := p.GetStatus(context.Background(), "pi_st")
	require.Nil(t, nerr)
	require.NotNil(t, res)
	assert.Equal(t, "PROCESSING", res.Status)
	assert.Equal(t, "sec_st", res.ClientSecret)
	assert.Equal(t, "pi_st", res.ProviderTransactionID)

	serveStripe(t, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing")
	}, 0)

	res, nerr = p.GetStatus(context.Background(), "pi_missing")
	assert.Nil(t, res)
	require.NotNil(t, nerr)
	assert.Equal(t, ports.ErrCategoryPermanent, nerr.Category)
}
