package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the payment service.
type Metrics struct {
	// Payment operations
	PaymentsCreatedTotal     *prometheus.CounterVec
	PaymentsAuthorizedTotal  *prometheus.CounterVec
	PaymentsCapturedTotal    *prometheus.CounterVec
	PaymentsCancelledTotal   *prometheus.CounterVec
	PaymentsRefundedTotal    *prometheus.CounterVec
	PaymentsFailedTotal      *prometheus.CounterVec

	// Payment operations by status
	PaymentStatusChanges     *prometheus.CounterVec

	// Payment amounts
	PaymentAmountMinor       *prometheus.HistogramVec

	// Payment latency
	PaymentDurationSeconds   *prometheus.HistogramVec

	// Provider calls
	ProviderCallDuration     *prometheus.HistogramVec
	ProviderCallTotal        *prometheus.CounterVec
	ProviderCallErrors       *prometheus.CounterVec

	// Webhook events
	WebhookEventsReceived    *prometheus.CounterVec
	WebhookProcessingDuration *prometheus.HistogramVec

	// Outbox processing
	OutboxMessagesPublished  *prometheus.CounterVec
	OutboxProcessingDuration *prometheus.HistogramVec

	// Reconciliation
	ReconciliationRuns       *prometheus.CounterVec
	ReconciliationDuration   *prometheus.HistogramVec

	// Idempotency
	IdempotencyKeyHits       *prometheus.CounterVec
	IdempotencyKeyMisses     *prometheus.CounterVec
}

// NewMetrics creates and registers all payment service metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		PaymentsCreatedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payments_created_total",
			Help: "Total number of payments created",
		}, []string{"currency", "method"}),

		PaymentsAuthorizedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payments_authorized_total",
			Help: "Total number of payments authorized",
		}, []string{"currency", "method"}),

		PaymentsCapturedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payments_captured_total",
			Help: "Total number of payments captured",
		}, []string{"currency", "method"}),

		PaymentsCancelledTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payments_cancelled_total",
			Help: "Total number of payments cancelled",
		}, []string{"currency", "method"}),

		PaymentsRefundedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payments_refunded_total",
			Help: "Total number of payments refunded",
		}, []string{"currency", "method"}),

		PaymentsFailedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payments_failed_total",
			Help: "Total number of payments failed",
		}, []string{"currency", "method", "error_type"}),

		PaymentStatusChanges: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "payment_status_changes_total",
			Help: "Total number of payment status changes",
		}, []string{"from_status", "to_status"}),

		PaymentAmountMinor: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "payment_amount_minor",
			Help:    "Payment amount in minor units",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		}, []string{"currency", "type"}),

		PaymentDurationSeconds: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "payment_duration_seconds",
			Help:    "Payment operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation", "status"}),

		ProviderCallDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "provider_call_duration_seconds",
			Help:    "Provider call duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider", "operation", "status"}),

		ProviderCallTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "provider_call_total",
			Help: "Total number of provider calls",
		}, []string{"provider", "operation", "status"}),

		ProviderCallErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "provider_call_errors_total",
			Help: "Total number of provider call errors",
		}, []string{"provider", "operation", "error_type"}),

		WebhookEventsReceived: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "webhook_events_received_total",
			Help: "Total number of webhook events received",
		}, []string{"event_type", "status"}),

		WebhookProcessingDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "webhook_processing_duration_seconds",
			Help:    "Webhook processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"event_type", "status"}),

		OutboxMessagesPublished: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "outbox_messages_published_total",
			Help: "Total number of outbox messages published",
		}, []string{"topic", "status"}),

		OutboxProcessingDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "outbox_processing_duration_seconds",
			Help:    "Outbox processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"topic", "status"}),

		ReconciliationRuns: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "reconciliation_runs_total",
			Help: "Total number of reconciliation runs",
		}, []string{"status"}),

		ReconciliationDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "reconciliation_duration_seconds",
			Help:    "Reconciliation run duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"status"}),

		IdempotencyKeyHits: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "idempotency_key_hits_total",
			Help: "Total number of idempotency key cache hits",
		}, []string{"key_type"}),

		IdempotencyKeyMisses: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "idempotency_key_misses_total",
			Help: "Total number of idempotency key cache misses",
		}, []string{"key_type"}),
	}
}

// RecordPaymentCreated records a payment creation.
func (m *Metrics) RecordPaymentCreated(currency string, method string) {
	m.PaymentsCreatedTotal.WithLabelValues(currency, method).Inc()
}

// RecordPaymentAuthorized records a payment authorization.
func (m *Metrics) RecordPaymentAuthorized(currency string, method string) {
	m.PaymentsAuthorizedTotal.WithLabelValues(currency, method).Inc()
}

// RecordPaymentCaptured records a payment capture.
func (m *Metrics) RecordPaymentCaptured(currency string, method string) {
	m.PaymentsCapturedTotal.WithLabelValues(currency, method).Inc()
}

// RecordPaymentCancelled records a payment cancellation.
func (m *Metrics) RecordPaymentCancelled(currency string, method string) {
	m.PaymentsCancelledTotal.WithLabelValues(currency, method).Inc()
}

// RecordPaymentRefunded records a payment refund.
func (m *Metrics) RecordPaymentRefunded(currency string, method string) {
	m.PaymentsRefundedTotal.WithLabelValues(currency, method).Inc()
}

// RecordPaymentFailed records a payment failure.
func (m *Metrics) RecordPaymentFailed(currency string, method string, errorType string) {
	m.PaymentsFailedTotal.WithLabelValues(currency, method, errorType).Inc()
}

// RecordPaymentStatusChange records a payment status change.
func (m *Metrics) RecordPaymentStatusChange(fromStatus, toStatus string) {
	m.PaymentStatusChanges.WithLabelValues(fromStatus, toStatus).Inc()
}

// RecordPaymentAmount records the payment amount.
func (m *Metrics) RecordPaymentAmount(amount int64, currency, payType string) {
	m.PaymentAmountMinor.WithLabelValues(currency, payType).Observe(float64(amount))
}

// RecordPaymentDuration records the payment operation duration.
func (m *Metrics) RecordPaymentDuration(duration float64, operation, status string) {
	m.PaymentDurationSeconds.WithLabelValues(operation, status).Observe(duration)
}

// RecordProviderCallDuration records the provider call duration.
func (m *Metrics) RecordProviderCallDuration(duration float64, provider, operation, status string) {
	m.ProviderCallDuration.WithLabelValues(provider, operation, status).Observe(duration)
}

// RecordProviderCall records a provider call.
func (m *Metrics) RecordProviderCall(provider, operation, status string) {
	m.ProviderCallTotal.WithLabelValues(provider, operation, status).Inc()
}

// RecordProviderError records a provider error.
func (m *Metrics) RecordProviderError(provider, operation, errorType string) {
	m.ProviderCallErrors.WithLabelValues(provider, operation, errorType).Inc()
}

// RecordWebhookEvent records a webhook event.
func (m *Metrics) RecordWebhookEvent(eventType, status string) {
	m.WebhookEventsReceived.WithLabelValues(eventType, status).Inc()
}

// RecordWebhookProcessingDuration records the webhook processing duration.
func (m *Metrics) RecordWebhookProcessingDuration(duration float64, eventType, status string) {
	m.WebhookProcessingDuration.WithLabelValues(eventType, status).Observe(duration)
}

// RecordOutboxMessagePublished records an outbox message publication.
func (m *Metrics) RecordOutboxMessagePublished(topic, status string) {
	m.OutboxMessagesPublished.WithLabelValues(topic, status).Inc()
}

// RecordOutboxProcessingDuration records the outbox processing duration.
func (m *Metrics) RecordOutboxProcessingDuration(duration float64, topic, status string) {
	m.OutboxProcessingDuration.WithLabelValues(topic, status).Observe(duration)
}

// RecordReconciliationRun records a reconciliation run.
func (m *Metrics) RecordReconciliationRun(status string) {
	m.ReconciliationRuns.WithLabelValues(status).Inc()
}

// RecordReconciliationDuration records the reconciliation duration.
func (m *Metrics) RecordReconciliationDuration(duration float64, status string) {
	m.ReconciliationDuration.WithLabelValues(status).Observe(duration)
}

// RecordIdempotencyHit records an idempotency key cache hit.
func (m *Metrics) RecordIdempotencyHit(keyType string) {
	m.IdempotencyKeyHits.WithLabelValues(keyType).Inc()
}

// RecordIdempotencyMiss records an idempotency key cache miss.
func (m *Metrics) RecordIdempotencyMiss(keyType string) {
	m.IdempotencyKeyMisses.WithLabelValues(keyType).Inc()
}