package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterMediaMetricsIdempotent(t *testing.T) {
	RegisterMediaMetrics()
	RegisterMediaMetrics()

	if UploadStartedTotal == nil ||
		UploadCompletedTotal == nil ||
		UploadFailedTotal == nil ||
		UploadBytesTotal == nil ||
		ActiveUploads == nil ||
		ProcessingStartedTotal == nil ||
		ProcessingCompletedTotal == nil ||
		ProcessingFailedTotal == nil ||
		ProcessingDuration == nil ||
		S3ErrorsTotal == nil ||
		KafkaPublishFailures == nil ||
		IdempotencyHitsTotal == nil ||
		ReconciliationTotal == nil ||
		DeadLetterQueueTotal == nil ||
		KafkaConsumerLag == nil ||
		QuotaUsageBytes == nil {
		t.Fatal("all media metric vars must be non-nil after RegisterMediaMetrics")
	}
}

func TestRegisterMediaMetricsRegistersCollectors(t *testing.T) {
	RegisterMediaMetrics()

	UploadStartedTotal.WithLabelValues("probe")
	UploadCompletedTotal.WithLabelValues("probe")
	UploadFailedTotal.WithLabelValues("probe")
	UploadBytesTotal.WithLabelValues("probe")
	ProcessingStartedTotal.WithLabelValues("probe")
	ProcessingCompletedTotal.WithLabelValues("probe")
	ProcessingFailedTotal.WithLabelValues("probe")
	ProcessingDuration.WithLabelValues("probe")
	S3ErrorsTotal.WithLabelValues("probe")
	KafkaPublishFailures.WithLabelValues("probe")
	IdempotencyHitsTotal.WithLabelValues("probe")
	ReconciliationTotal.WithLabelValues("probe")
	DeadLetterQueueTotal.WithLabelValues("probe")
	KafkaConsumerLag.WithLabelValues("probe", "probe")
	QuotaUsageBytes.WithLabelValues("probe")

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	names := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	want := []string{
		"media_upload_started_total",
		"media_upload_completed_total",
		"media_upload_failed_total",
		"media_upload_bytes_total",
		"media_active_uploads",
		"media_processing_started_total",
		"media_processing_completed_total",
		"media_processing_failed_total",
		"media_processing_duration_seconds",
		"media_s3_errors_total",
		"media_kafka_publish_failures_total",
		"media_idempotency_hits_total",
		"media_reconciliation_total",
		"media_dlq_total",
		"media_kafka_consumer_lag",
		"media_quota_usage_bytes",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("metric family %q not registered", name)
		}
	}
}
