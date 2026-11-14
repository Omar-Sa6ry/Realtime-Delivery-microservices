package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordHelpersAllMetrics(t *testing.T) {
	m := M()

	m.RecordPaymentAuthorized("usd", "card")
	m.RecordPaymentCaptured("usd", "card")
	m.RecordPaymentCancelled("usd", "card")
	m.RecordPaymentRefunded("usd", "card")
	m.RecordPaymentStatusChange("PENDING", "AUTHORIZED")
	m.RecordPaymentAmount(1500, "USD", "payment")
	m.RecordPaymentDuration(0.25, "create", "ok")
	m.RecordProviderCallDuration(0.5, "stripe", "authorize", "ok")
	m.RecordProviderCall("stripe", "authorize", "ok")
	m.RecordProviderError("stripe", "authorize", "timeout")
	m.RecordWebhookEvent("payment.captured", "processed")
	m.RecordWebhookProcessingDuration(0.02, "payment.captured", "processed")
	m.RecordOutboxMessagePublished("payment-events", "ok")
	m.RecordOutboxProcessingDuration(0.1, "payment-events", "ok")
	m.RecordReconciliationRun("ok")
	m.RecordReconciliationDuration(1.5, "ok")
	m.RecordIdempotencyHit("payment.create")
	m.RecordIdempotencyMiss("payment.create")

	assert.Equal(t, float64(1), testutil.ToFloat64(m.PaymentsAuthorizedTotal.WithLabelValues("usd", "card")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.PaymentsCapturedTotal.WithLabelValues("usd", "card")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.PaymentsCancelledTotal.WithLabelValues("usd", "card")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.PaymentsRefundedTotal.WithLabelValues("usd", "card")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.PaymentStatusChanges.WithLabelValues("PENDING", "AUTHORIZED")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.ProviderCallTotal.WithLabelValues("stripe", "authorize", "ok")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.ProviderCallErrors.WithLabelValues("stripe", "authorize", "timeout")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.WebhookEventsReceived.WithLabelValues("payment.captured", "processed")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.OutboxMessagesPublished.WithLabelValues("payment-events", "ok")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.ReconciliationRuns.WithLabelValues("ok")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.IdempotencyKeyHits.WithLabelValues("payment.create")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.IdempotencyKeyMisses.WithLabelValues("payment.create")))

	// histograms only need their Observe path executed; series existence is
	// verified through the collector's own Describe/Collect contract
	assert.Positive(t, testutil.CollectAndCount(m.PaymentAmountMinor))
	assert.Positive(t, testutil.CollectAndCount(m.PaymentDurationSeconds))
	assert.Positive(t, testutil.CollectAndCount(m.ProviderCallDuration))
	assert.Positive(t, testutil.CollectAndCount(m.WebhookProcessingDuration))
	assert.Positive(t, testutil.CollectAndCount(m.OutboxProcessingDuration))
	assert.Positive(t, testutil.CollectAndCount(m.ReconciliationDuration))
}

func TestNewMetricsHasAllCollectors(t *testing.T) {
	m := M()
	assert.NotNil(t, m.PaymentsCreatedTotal)
	assert.NotNil(t, m.PaymentsAuthorizedTotal)
	assert.NotNil(t, m.PaymentsCapturedTotal)
	assert.NotNil(t, m.PaymentsCancelledTotal)
	assert.NotNil(t, m.PaymentsRefundedTotal)
	assert.NotNil(t, m.PaymentsFailedTotal)
	assert.NotNil(t, m.PaymentStatusChanges)
	assert.NotNil(t, m.PaymentAmountMinor)
	assert.NotNil(t, m.PaymentDurationSeconds)
	assert.NotNil(t, m.ProviderCallDuration)
	assert.NotNil(t, m.ProviderCallTotal)
	assert.NotNil(t, m.ProviderCallErrors)
	assert.NotNil(t, m.WebhookEventsReceived)
	assert.NotNil(t, m.WebhookProcessingDuration)
	assert.NotNil(t, m.OutboxMessagesPublished)
	assert.NotNil(t, m.OutboxProcessingDuration)
	assert.NotNil(t, m.ReconciliationRuns)
	assert.NotNil(t, m.ReconciliationDuration)
	assert.NotNil(t, m.IdempotencyKeyHits)
	assert.NotNil(t, m.IdempotencyKeyMisses)
}
