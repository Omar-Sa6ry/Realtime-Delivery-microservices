package observability

import "time"

func RecordUploadStarted(contentType string) {
	if UploadStartedTotal != nil {
		UploadStartedTotal.WithLabelValues(contentType).Inc()
	}
}

func RecordUploadCompleted(contentType string) {
	if UploadCompletedTotal != nil {
		UploadCompletedTotal.WithLabelValues(contentType).Inc()
	}
}

func RecordUploadFailed(reason string) {
	if UploadFailedTotal != nil {
		UploadFailedTotal.WithLabelValues(reason).Inc()
	}
}

func RecordUploadBytes(mediaType string, bytes int64) {
	if UploadBytesTotal != nil {
		UploadBytesTotal.WithLabelValues(mediaType).Add(float64(bytes))
	}
}

func IncActiveUploads() {
	if ActiveUploads != nil {
		ActiveUploads.Inc()
	}
}

func DecActiveUploads() {
	if ActiveUploads != nil {
		ActiveUploads.Dec()
	}
}

func SetQuotaUsage(userID string, usedBytes int64) {
	if QuotaUsageBytes != nil {
		QuotaUsageBytes.WithLabelValues(userID).Set(float64(usedBytes))
	}
}

func RecordProcessingStarted(jobType string) {
	if ProcessingStartedTotal != nil {
		ProcessingStartedTotal.WithLabelValues(jobType).Inc()
	}
}

func RecordProcessingCompleted(jobType string) {
	if ProcessingCompletedTotal != nil {
		ProcessingCompletedTotal.WithLabelValues(jobType).Inc()
	}
}

func RecordProcessingFailed(jobType string) {
	if ProcessingFailedTotal != nil {
		ProcessingFailedTotal.WithLabelValues(jobType).Inc()
	}
}

func ObserveProcessingDuration(jobType string, d time.Duration) {
	if ProcessingDuration != nil {
		ProcessingDuration.WithLabelValues(jobType).Observe(d.Seconds())
	}
}

func RecordS3Error(operation string) {
	if S3ErrorsTotal != nil {
		S3ErrorsTotal.WithLabelValues(operation).Inc()
	}
}

func RecordKafkaPublishFailure(topic string) {
	if KafkaPublishFailures != nil {
		KafkaPublishFailures.WithLabelValues(topic).Inc()
	}
}

func RecordIdempotencyHit(operation string) {
	if IdempotencyHitsTotal != nil {
		IdempotencyHitsTotal.WithLabelValues(operation).Inc()
	}
}

func RecordReconciliation(action string) {
	if ReconciliationTotal != nil {
		ReconciliationTotal.WithLabelValues(action).Inc()
	}
}
