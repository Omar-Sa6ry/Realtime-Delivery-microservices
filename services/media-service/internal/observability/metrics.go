package observability

import (
	"sync"

	sharedmetrics "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	UploadStartedTotal   *prometheus.CounterVec
	UploadCompletedTotal *prometheus.CounterVec
	UploadFailedTotal    *prometheus.CounterVec
	UploadBytesTotal     *prometheus.CounterVec
	ActiveUploads        prometheus.Gauge

	ProcessingStartedTotal   *prometheus.CounterVec
	ProcessingCompletedTotal *prometheus.CounterVec
	ProcessingFailedTotal    *prometheus.CounterVec
	ProcessingDuration       *prometheus.HistogramVec

	S3ErrorsTotal        *prometheus.CounterVec
	KafkaPublishFailures *prometheus.CounterVec
	IdempotencyHitsTotal *prometheus.CounterVec
	ReconciliationTotal  *prometheus.CounterVec
	DeadLetterQueueTotal *prometheus.CounterVec
	KafkaConsumerLag     *prometheus.GaugeVec

	QuotaUsageBytes *prometheus.GaugeVec

	once sync.Once
)

func RegisterMediaMetrics() {
	once.Do(func() {
		// Ensure shared metrics are registered first.
		sharedmetrics.RegisterMetrics()

		UploadStartedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_upload_started_total",
			Help: "Total number of upload sessions created.",
		}, []string{"content_type"})

		UploadCompletedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_upload_completed_total",
			Help: "Total number of uploads completed successfully.",
		}, []string{"content_type"})

		UploadFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_upload_failed_total",
			Help: "Total number of upload failures.",
		}, []string{"reason"})

		UploadBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_upload_bytes_total",
			Help: "Total bytes uploaded (completed uploads only).",
		}, []string{"media_type"})

		ActiveUploads = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "media_active_uploads",
			Help: "Number of currently in-progress upload sessions.",
		})

		ProcessingStartedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_processing_started_total",
			Help: "Total number of media processing jobs started.",
		}, []string{"job_type"})

		ProcessingCompletedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_processing_completed_total",
			Help: "Total number of media processing jobs completed.",
		}, []string{"job_type"})

		ProcessingFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_processing_failed_total",
			Help: "Total number of media processing jobs failed.",
		}, []string{"job_type"})

		ProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "media_processing_duration_seconds",
			Help:    "Duration of media processing jobs.",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		}, []string{"job_type"})

		S3ErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_s3_errors_total",
			Help: "Total number of S3 operation errors.",
		}, []string{"operation"})

		KafkaPublishFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_kafka_publish_failures_total",
			Help: "Total number of Kafka publish failures.",
		}, []string{"topic"})

		IdempotencyHitsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_idempotency_hits_total",
			Help: "Total number of duplicate requests detected via idempotency keys.",
		}, []string{"operation"})

		ReconciliationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_reconciliation_total",
			Help: "Total number of reconciliation actions taken.",
		}, []string{"action"})

		DeadLetterQueueTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "media_dlq_total",
			Help: "Total number of messages routed to dead letter queues.",
		}, []string{"topic"})

		KafkaConsumerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "media_kafka_consumer_lag",
			Help: "Current Kafka consumer lag by topic and consumer group.",
		}, []string{"topic", "group"})

		QuotaUsageBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "media_quota_usage_bytes",
			Help: "Current storage quota usage per user.",
		}, []string{"user_id"})

		prometheus.MustRegister(
			UploadStartedTotal,
			UploadCompletedTotal,
			UploadFailedTotal,
			UploadBytesTotal,
			ActiveUploads,
			ProcessingStartedTotal,
			ProcessingCompletedTotal,
			ProcessingFailedTotal,
			ProcessingDuration,
			S3ErrorsTotal,
			KafkaPublishFailures,
			IdempotencyHitsTotal,
			ReconciliationTotal,
			DeadLetterQueueTotal,
			KafkaConsumerLag,
			QuotaUsageBytes,
		)
	})
}
