package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func millisecondBuckets() []float64 {
	return prometheus.ExponentialBuckets(1, 2, 12)
}

type Metrics struct {
	EventsConsumedTotal     *prometheus.CounterVec
	EventsProcessedTotal    *prometheus.CounterVec
	EventsFailedTotal       *prometheus.CounterVec
	EventsDuplicateTotal    *prometheus.CounterVec
	EventsDLQTotal          *prometheus.CounterVec
	ProcessingLatencyMs     *prometheus.HistogramVec
	BatchSize               *prometheus.HistogramVec
	BatchFlushTotal         *prometheus.CounterVec
	QueryLatencyMs          *prometheus.HistogramVec
	QueryErrorsTotal        *prometheus.CounterVec
	ReplayEventsTotal       *prometheus.CounterVec
	DataQualityIssuesTotal  *prometheus.CounterVec
	FreshnessSeconds        prometheus.Gauge
	KafkaConsumerLag        *prometheus.GaugeVec
	ClickHouseInsertLatency *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		EventsConsumedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_consumed_total",
			Help: "Total Kafka events consumed per topic",
		}, []string{"topic"}),

		EventsProcessedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_processed_total",
			Help: "Total events successfully transformed and buffered",
		}, []string{"event_type"}),

		EventsFailedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_failed_total",
			Help: "Total events that failed processing (retryable or permanent)",
		}, []string{"event_type"}),

		EventsDuplicateTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_duplicate_total",
			Help: "Total redelivered events collapsed by idempotency",
		}, []string{"topic"}),

		EventsDLQTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_dlq_total",
			Help: "Total events routed to the dead-letter topic",
		}, []string{"topic"}),

		ProcessingLatencyMs: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "analytics_processing_latency_ms",
			Help:    "Per-event pipeline latency in milliseconds",
			Buckets: millisecondBuckets(),
		}, []string{"event_type", "outcome"}),

		BatchSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "analytics_batch_size",
			Help:    "Rows per ClickHouse batch flush",
			Buckets: prometheus.ExponentialBuckets(1, 2, 12),
		}, []string{"table"}),

		BatchFlushTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_batch_flush_total",
			Help: "Total ClickHouse batch flushes by table and status",
		}, []string{"table", "status"}),

		QueryLatencyMs: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "analytics_query_latency_ms",
			Help:    "GraphQL query latency in milliseconds",
			Buckets: millisecondBuckets(),
		}, []string{"query"}),

		QueryErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_query_errors_total",
			Help: "Total failed GraphQL queries",
		}, []string{"query"}),

		ReplayEventsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_replay_events_total",
			Help: "Total events reprocessed by replay jobs",
		}, []string{"topic", "outcome"}),

		DataQualityIssuesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_data_quality_issues_total",
			Help: "Total data quality issues detected",
		}, []string{"issue_type", "severity"}),

		FreshnessSeconds: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_freshness_seconds",
			Help: "Now minus latest successfully processed event time",
		}),

		KafkaConsumerLag: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Kafka consumer lag by topic and partition",
		}, []string{"topic", "partition"}),

		ClickHouseInsertLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "clickhouse_insert_latency",
			Help:    "ClickHouse insert latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"table", "status"}),
	}
}

// RecordConsumed records a consumed Kafka message.
func (m *Metrics) RecordConsumed(topic string) {
	m.EventsConsumedTotal.WithLabelValues(topic).Inc()
}

// RecordProcessed records a successfully transformed event with its latency.
func (m *Metrics) RecordProcessed(eventType, outcome string, latencyMs float64) {
	m.EventsProcessedTotal.WithLabelValues(eventType).Inc()
	m.ProcessingLatencyMs.WithLabelValues(eventType, outcome).Observe(latencyMs)
}

// RecordFailed records a failed event.
func (m *Metrics) RecordFailed(eventType string) {
	m.EventsFailedTotal.WithLabelValues(eventType).Inc()
}

// RecordDuplicate records an idempotency-collapsed redelivery.
func (m *Metrics) RecordDuplicate(topic string) {
	m.EventsDuplicateTotal.WithLabelValues(topic).Inc()
}

// RecordDLQ records a DLQ routing.
func (m *Metrics) RecordDLQ(topic string) {
	m.EventsDLQTotal.WithLabelValues(topic).Inc()
}

// RecordBatchFlush records a table flush with row count and insert latency.
func (m *Metrics) RecordBatchFlush(table, status string, rows int, insertSeconds float64) {
	m.BatchFlushTotal.WithLabelValues(table, status).Inc()
	m.BatchSize.WithLabelValues(table).Observe(float64(rows))
	m.ClickHouseInsertLatency.WithLabelValues(table, status).Observe(insertSeconds)
}

// RecordQuery records a GraphQL query outcome with latency.
func (m *Metrics) RecordQuery(query, status string, latencyMs float64) {
	if status != "success" {
		m.QueryErrorsTotal.WithLabelValues(query).Inc()
	}
	m.QueryLatencyMs.WithLabelValues(query).Observe(latencyMs)
}

// RecordReplay records a replayed event outcome.
func (m *Metrics) RecordReplay(topic, outcome string) {
	m.ReplayEventsTotal.WithLabelValues(topic, outcome).Inc()
}

// RecordDataQualityIssue records a detected data quality issue.
func (m *Metrics) RecordDataQualityIssue(issueType, severity string) {
	m.DataQualityIssuesTotal.WithLabelValues(issueType, severity).Inc()
}

// SetFreshness sets the current pipeline freshness in seconds.
func (m *Metrics) SetFreshness(seconds float64) {
	m.FreshnessSeconds.Set(seconds)
}

// SetConsumerLag sets the lag gauge for a topic-partition.
func (m *Metrics) SetConsumerLag(topic, partition string, lag float64) {
	m.KafkaConsumerLag.WithLabelValues(topic, partition).Set(lag)
}
