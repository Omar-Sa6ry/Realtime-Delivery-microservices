package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/assert"
)

func newTestMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)
	return &Metrics{
		EventsConsumedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_consumed_total",
			Help: "Total Kafka events consumed per topic",
		}, []string{"topic"}),

		EventsProcessedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_processed_total",
			Help: "Total events successfully transformed and buffered",
		}, []string{"event_type"}),

		EventsFailedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_failed_total",
			Help: "Total events that failed processing (retryable or permanent)",
		}, []string{"event_type"}),

		EventsDuplicateTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_duplicate_total",
			Help: "Total redelivered events collapsed by idempotency",
		}, []string{"topic"}),

		EventsDLQTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_dlq_total",
			Help: "Total events routed to the dead-letter topic",
		}, []string{"topic"}),

		ProcessingLatencyMs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "analytics_processing_latency_ms",
			Help:    "Per-event pipeline latency in milliseconds",
			Buckets: millisecondBuckets(),
		}, []string{"event_type", "outcome"}),

		BatchSize: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "analytics_batch_size",
			Help:    "Rows per ClickHouse batch flush",
			Buckets: prometheus.ExponentialBuckets(1, 2, 12),
		}, []string{"table"}),

		BatchFlushTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_batch_flush_total",
			Help: "Total ClickHouse batch flushes by table and status",
		}, []string{"table", "status"}),

		QueryLatencyMs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "analytics_query_latency_ms",
			Help:    "GraphQL query latency in milliseconds",
			Buckets: millisecondBuckets(),
		}, []string{"query"}),

		QueryErrorsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_query_errors_total",
			Help: "Total failed GraphQL queries",
		}, []string{"query"}),

		ReplayEventsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_replay_events_total",
			Help: "Total events reprocessed by replay jobs",
		}, []string{"topic", "outcome"}),

		DataQualityIssuesTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_data_quality_issues_total",
			Help: "Total data quality issues detected",
		}, []string{"issue_type", "severity"}),

		FreshnessSeconds: factory.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_freshness_seconds",
			Help: "Now minus latest successfully processed event time",
		}),

		KafkaConsumerLag: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Kafka consumer lag by topic and partition",
		}, []string{"topic", "partition"}),

		ClickHouseInsertLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "clickhouse_insert_latency",
			Help:    "ClickHouse insert latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"table", "status"}),
	}
}

func TestNewMetrics(t *testing.T) {
	m := newTestMetrics()
	assert.NotNil(t, m)
	assert.NotNil(t, m.EventsConsumedTotal)
	assert.NotNil(t, m.EventsProcessedTotal)
	assert.NotNil(t, m.EventsFailedTotal)
	assert.NotNil(t, m.EventsDuplicateTotal)
	assert.NotNil(t, m.EventsDLQTotal)
	assert.NotNil(t, m.ProcessingLatencyMs)
	assert.NotNil(t, m.BatchSize)
	assert.NotNil(t, m.BatchFlushTotal)
	assert.NotNil(t, m.QueryLatencyMs)
	assert.NotNil(t, m.QueryErrorsTotal)
	assert.NotNil(t, m.ReplayEventsTotal)
	assert.NotNil(t, m.DataQualityIssuesTotal)
	assert.NotNil(t, m.FreshnessSeconds)
	assert.NotNil(t, m.KafkaConsumerLag)
	assert.NotNil(t, m.ClickHouseInsertLatency)
}

func TestMetrics_RecordConsumed(t *testing.T) {
	m := newTestMetrics()
	m.RecordConsumed("test-topic")
	// Verify metric was recorded (no panic)
}

func TestMetrics_RecordProcessed(t *testing.T) {
	m := newTestMetrics()
	m.RecordProcessed("delivery.created", "success", 10.5)
	m.RecordProcessed("delivery.created", "duplicate", 5.0)
}

func TestMetrics_RecordFailed(t *testing.T) {
	m := newTestMetrics()
	m.RecordFailed("payment.created")
}

func TestMetrics_RecordDuplicate(t *testing.T) {
	m := newTestMetrics()
	m.RecordDuplicate("delivery.events")
}

func TestMetrics_RecordDLQ(t *testing.T) {
	m := newTestMetrics()
	m.RecordDLQ("delivery.events")
}

func TestMetrics_RecordBatchFlush(t *testing.T) {
	m := newTestMetrics()
	m.RecordBatchFlush("raw_events", "success", 100, 0.05)
	m.RecordBatchFlush("fact_delivery_events", "error", 50, 0.1)
}

func TestMetrics_RecordQuery(t *testing.T) {
	m := newTestMetrics()
	m.RecordQuery("platformOverview", "success", 25.0)
	m.RecordQuery("deliveryAnalytics", "error", 100.0)
}

func TestMetrics_RecordReplay(t *testing.T) {
	m := newTestMetrics()
	m.RecordReplay("delivery.events", "success")
	m.RecordReplay("delivery.events", "failed")
}

func TestMetrics_RecordDataQualityIssue(t *testing.T) {
	m := newTestMetrics()
	m.RecordDataQualityIssue("duplicate_event", "warning")
	m.RecordDataQualityIssue("out_of_order", "error")
}

func TestMetrics_SetFreshness(t *testing.T) {
	m := newTestMetrics()
	m.SetFreshness(30.5)
}

func TestMetrics_GetFreshnessSeconds(t *testing.T) {
	m := newTestMetrics()
	assert.Equal(t, 0.0, m.GetFreshnessSeconds())
}

func TestMetrics_SetConsumerLag(t *testing.T) {
	m := newTestMetrics()
	m.SetConsumerLag("delivery.events", "0", 100.0)
}

func TestMillisecondBuckets(t *testing.T) {
	buckets := millisecondBuckets()
	assert.NotEmpty(t, buckets)
	// Verify it's exponential buckets starting at 1 with factor 2, 12 buckets
	assert.Len(t, buckets, 12)
	assert.Equal(t, 1.0, buckets[0])
	assert.Equal(t, 2.0, buckets[1])
	assert.Equal(t, 4.0, buckets[2])
}

func TestMetrics_PrometheusRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		EventsConsumedTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "test_events_consumed_total",
			Help: "Test counter",
		}, []string{"topic"}),
	}

	m.RecordConsumed("test-topic")

	// Verify metric is registered
	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)
	assert.Len(t, metricFamilies, 1)
}