package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
)

const testSecret = "whsec_test_secret"

func init() {
	gin.SetMode(gin.TestMode)
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("body read failed") }
func (failingBody) Close() error              { return nil }

// sign builds a Stripe-Signature header the way stripe-go verifies it.
func sign(body []byte) string {
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(testSecret))
	_, _ = fmt.Fprintf(mac, "%d.%s", ts, string(body))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func eventJSON(eventID, eventType, objectJSON string) []byte {
	return []byte(fmt.Sprintf(
		`{"id":%q,"object":"event","type":%q,"data":{"object":%s}}`,
		eventID, eventType, objectJSON,
	))
}

func paymentCols() []string {
	return []string{
		"id", "delivery_id", "user_id", "amount_minor", "currency", "status",
		"provider", "provider_payment_id", "gateway_session_id",
		"authorized_amount_minor", "captured_amount_minor", "refunded_amount_minor", "pending_refund_minor",
		"version", "correlation_id", "causation_id",
		"created_at", "updated_at", "authorized_at", "captured_at", "cancelled_at", "failed_at",
	}
}

func paymentRows(status string, version int64) *sqlmock.Rows {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ts := now
	return sqlmock.NewRows(paymentCols()).AddRow(
		"pay_1", "del_1", "usr_1", int64(5000), "USD", status,
		"stripe", "pi_1", "cs_1",
		int64(5000), int64(0), int64(0), int64(0),
		version, "corr_1", "cause_1",
		now, now, &ts, nil, nil, nil,
	)
}

func newTestHandler(t *testing.T) (*Handler, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewHandler(testSecret, postgres.NewPaymentRepository(db), postgres.NewOutboxRepository(db)), mock
}

func callHandle(h *Handler, body io.Reader, signature string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/webhooks/stripe", body)
	c.Request.Header.Set("Stripe-Signature", signature)
	h.Handle(c)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestHandleBodyReadFailure(t *testing.T) {
	h, mock := newTestHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	req.Body = failingBody{}
	c.Request = req
	h.Handle(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "failed to read body", decodeBody(t, w)["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleInvalidSignature(t *testing.T) {
	h, mock := newTestHandler(t)

	w := callHandle(h, bytes.NewReader(eventJSON("evt_bad", "charge.refunded", `{}`)), "t=1,v1=deadbeef")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = callHandle(h, bytes.NewReader(eventJSON("evt_bad2", "charge.refunded", `{}`)), "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleDuplicateEventShortCircuits(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").
		WithArgs("stripe", "evt_dup", "charge.refunded").
		WillReturnResult(sqlmock.NewResult(0, 0))

	body := eventJSON("evt_dup", "charge.refunded", `{}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["duplicate"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleDedupFailureReturns500(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").
		WillReturnError(errors.New("dedup table missing"))

	body := eventJSON("evt_err", "charge.refunded", `{}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUnhandledEventType(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_other", "invoice.paid", `{}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleRequiresActionEvent(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_ra", "payment_intent.requires_action", `{"id":"pi_ra"}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleProcessingFailureReturnsProcessedFalse(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_badobj", "payment_intent.succeeded", `{"id":123}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleMalformedObjectsForTerminalEvents(t *testing.T) {
	h, mock := newTestHandler(t)
	for _, tc := range []struct{ id, typ string }{
		{"evt_f", "payment_intent.payment_failed"},
		{"evt_c", "payment_intent.canceled"},
		{"evt_r", "charge.refunded"},
	} {
		mock.ExpectExec("INSERT INTO processed_provider_events").
			WillReturnResult(sqlmock.NewResult(1, 1))
		body := eventJSON(tc.id, tc.typ, `{"id":123}`)
		w := callHandle(h, bytes.NewReader(body), sign(body))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, false, decodeBody(t, w)["processed"], tc.typ)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleTerminalEventsSuccess(t *testing.T) {
	h, mock := newTestHandler(t)
	for _, tc := range []struct{ id, typ, object string }{
		{"evt_f2", "payment_intent.payment_failed", `{"id":"pi_f","status":"requires_payment_method"}`},
		{"evt_c2", "payment_intent.canceled", `{"id":"pi_c","status":"canceled"}`},
		{"evt_r2", "charge.refunded", `{"id":"ch_r"}`},
	} {
		mock.ExpectExec("INSERT INTO processed_provider_events").
			WillReturnResult(sqlmock.NewResult(1, 1))
		body := eventJSON(tc.id, tc.typ, tc.object)
		w := callHandle(h, bytes.NewReader(body), sign(body))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, true, decodeBody(t, w)["processed"], tc.typ)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleMalformedCheckoutSessionObject(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_cs_bad", "checkout.session.completed", `{"id":123}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckoutSessionCompletedFindsByPaymentID(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE id = \$1`).
		WithArgs("pay_1").
		WillReturnRows(paymentRows("authorized", 3))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO payment_events_outbox").WillReturnResult(sqlmock.NewResult(1, 1))

	object := `{"id":"cs_1","metadata":{"payment_id":"pay_1"}}`
	body := eventJSON("evt_cs1", "checkout.session.completed", object)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckoutSessionCompletedFallsBackToProviderPaymentID(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE id = \$1`).
		WithArgs("pay_unknown").
		WillReturnError(fmt.Errorf("no such payment"))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).
		WithArgs("pi_9").
		WillReturnRows(paymentRows("authorized", 1))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO payment_events_outbox").WillReturnResult(sqlmock.NewResult(1, 1))

	object := `{"id":"cs_9","metadata":{"payment_id":"pay_unknown"},"payment_intent":{"id":"pi_9"}}`
	body := eventJSON("evt_cs2", "checkout.session.completed", object)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckoutSessionCompletedNoMetadataNoPaymentIntent(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_cs3", "checkout.session.completed", `{"id":"cs_3"}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckoutSessionCompletedNotFoundInStore(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE id = \$1`).WithArgs("pay_x").WillReturnRows(sqlmock.NewRows(paymentCols()))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).WithArgs("pi_x").WillReturnRows(sqlmock.NewRows(paymentCols()))

	object := `{"id":"cs_x","metadata":{"payment_id":"pay_x"},"payment_intent":{"id":"pi_x"}}`
	body := eventJSON("evt_cs4", "checkout.session.completed", object)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentIntentSucceededFindsByProviderIDWhenNoMetadata(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).
		WithArgs("pi_only").
		WillReturnRows(paymentRows("authorized", 2))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO payment_events_outbox").WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_pi1", "payment_intent.succeeded", `{"id":"pi_only"}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentIntentSucceededPaymentAlreadyCaptured(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE id = \$1`).
		WithArgs("pay_done").
		WillReturnRows(paymentRows("CAPTURED", 5))

	body := eventJSON("evt_pi2", "payment_intent.succeeded", `{"id":"pi_done","metadata":{"payment_id":"pay_done"}}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentIntentSucceededUpdateConflict(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).
		WithArgs("pi_conflict").
		WillReturnRows(paymentRows("authorized", 7))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 0))

	body := eventJSON("evt_pi3", "payment_intent.succeeded", `{"id":"pi_conflict"}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentIntentSucceededUpdateError(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).
		WithArgs("pi_dberr").
		WillReturnRows(paymentRows("authorized", 1))
	mock.ExpectExec(`version=\$15`).WillReturnError(errors.New("db write failed"))

	body := eventJSON("evt_pi4", "payment_intent.succeeded", `{"id":"pi_dberr"}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckoutSessionCompletedNoMetadataUsesProviderPaymentID(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).
		WithArgs("pi_nometa").
		WillReturnRows(paymentRows("AUTHORIZED", 2))
	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO payment_events_outbox").WillReturnResult(sqlmock.NewResult(1, 1))

	body := eventJSON("evt_cs5", "checkout.session.completed", `{"id":"cs_5","payment_intent":{"id":"pi_nometa"}}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentIntentSucceededLookupErrors(t *testing.T) {
	h, mock := newTestHandler(t)
	mock.ExpectExec("INSERT INTO processed_provider_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`WHERE id = \$1`).WithArgs("pay_err").WillReturnError(errors.New("db read failed"))
	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).WithArgs("pi_err").WillReturnError(errors.New("db read failed 2"))

	body := eventJSON("evt_pi5", "payment_intent.succeeded", `{"id":"pi_err","metadata":{"payment_id":"pay_err"}}`)
	w := callHandle(h, bytes.NewReader(body), sign(body))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["processed"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleMalformedEventObjectJSON(t *testing.T) {
	h, mock := newTestHandler(t)

	w := callHandle(h, bytes.NewReader([]byte("not-json-at-all")), sign([]byte("not-json-at-all")))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewHandlerKeepsSecret(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close(); _ = mock }()

	h := NewHandler("whsec_abc", postgres.NewPaymentRepository(db), postgres.NewOutboxRepository(db))
	require.NotNil(t, h)
	assert.Equal(t, "whsec_abc", h.webhookSecret)
	require.NotNil(t, h.paymentRepo)
	require.NotNil(t, h.outboxRepo)
}
