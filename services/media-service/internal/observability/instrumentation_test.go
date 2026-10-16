package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func readCounter(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		t.Fatalf("write counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

func readGauge(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := g.Write(m); err != nil {
		t.Fatalf("write gauge: %v", err)
	}
	return m.GetGauge().GetValue()
}

func readHistogram(t *testing.T, h prometheus.Histogram) (uint64, float64) {
	t.Helper()
	m := &dto.Metric{}
	if err := h.Write(m); err != nil {
		t.Fatalf("write histogram: %v", err)
	}
	return m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
}

func TestRecordUploadStarted(t *testing.T) {
	RegisterMediaMetrics()
	c := UploadStartedTotal.WithLabelValues("video/mp4")
	before := readCounter(t, c)
	RecordUploadStarted("video/mp4")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordUploadCompleted(t *testing.T) {
	RegisterMediaMetrics()
	c := UploadCompletedTotal.WithLabelValues("video/mp4")
	before := readCounter(t, c)
	RecordUploadCompleted("video/mp4")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordUploadFailed(t *testing.T) {
	RegisterMediaMetrics()
	c := UploadFailedTotal.WithLabelValues("quota_exceeded")
	before := readCounter(t, c)
	RecordUploadFailed("quota_exceeded")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordUploadBytes(t *testing.T) {
	RegisterMediaMetrics()
	c := UploadBytesTotal.WithLabelValues("video")
	before := readCounter(t, c)
	RecordUploadBytes("video", 1024)
	RecordUploadBytes("video", 256)
	if got := readCounter(t, c); got != before+1280 {
		t.Fatalf("counter = %v, want %v", got, before+1280)
	}
}

func TestIncDecActiveUploads(t *testing.T) {
	RegisterMediaMetrics()
	before := readGauge(t, ActiveUploads)
	IncActiveUploads()
	IncActiveUploads()
	DecActiveUploads()
	if got := readGauge(t, ActiveUploads); got != before+1 {
		t.Fatalf("gauge = %v, want %v", got, before+1)
	}
}

func TestSetQuotaUsage(t *testing.T) {
	RegisterMediaMetrics()
	g := QuotaUsageBytes.WithLabelValues("user-1")
	SetQuotaUsage("user-1", 4096)
	if got := readGauge(t, g); got != 4096 {
		t.Fatalf("gauge = %v, want 4096", got)
	}
}

func TestRecordProcessingStarted(t *testing.T) {
	RegisterMediaMetrics()
	c := ProcessingStartedTotal.WithLabelValues("transcode")
	before := readCounter(t, c)
	RecordProcessingStarted("transcode")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordProcessingCompleted(t *testing.T) {
	RegisterMediaMetrics()
	c := ProcessingCompletedTotal.WithLabelValues("transcode")
	before := readCounter(t, c)
	RecordProcessingCompleted("transcode")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordProcessingFailed(t *testing.T) {
	RegisterMediaMetrics()
	c := ProcessingFailedTotal.WithLabelValues("transcode")
	before := readCounter(t, c)
	RecordProcessingFailed("transcode")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestObserveProcessingDuration(t *testing.T) {
	RegisterMediaMetrics()
	h, ok := ProcessingDuration.WithLabelValues("transcode").(prometheus.Histogram)
	if !ok {
		t.Fatal("ProcessingDuration label value must implement prometheus.Histogram")
	}
	beforeCount, beforeSum := readHistogram(t, h)
	ObserveProcessingDuration("transcode", 2500*time.Millisecond)
	count, sum := readHistogram(t, h)
	if count != beforeCount+1 {
		t.Fatalf("sample count = %d, want %d", count, beforeCount+1)
	}
	if sum != beforeSum+2.5 {
		t.Fatalf("sample sum = %v, want %v", sum, beforeSum+2.5)
	}
}

func TestRecordS3Error(t *testing.T) {
	RegisterMediaMetrics()
	c := S3ErrorsTotal.WithLabelValues("PutObject")
	before := readCounter(t, c)
	RecordS3Error("PutObject")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordKafkaPublishFailure(t *testing.T) {
	RegisterMediaMetrics()
	c := KafkaPublishFailures.WithLabelValues("media.ready")
	before := readCounter(t, c)
	RecordKafkaPublishFailure("media.ready")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordIdempotencyHit(t *testing.T) {
	RegisterMediaMetrics()
	c := IdempotencyHitsTotal.WithLabelValues("upload")
	before := readCounter(t, c)
	RecordIdempotencyHit("upload")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestRecordReconciliation(t *testing.T) {
	RegisterMediaMetrics()
	c := ReconciliationTotal.WithLabelValues("requeue")
	before := readCounter(t, c)
	RecordReconciliation("requeue")
	if got := readCounter(t, c); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

func TestInstrumentationWithNilMetricsIsSafe(t *testing.T) {
	RegisterMediaMetrics()

	sUS, sUC, sUF, sUB := UploadStartedTotal, UploadCompletedTotal, UploadFailedTotal, UploadBytesTotal
	sAU := ActiveUploads
	sPS, sPC, sPF := ProcessingStartedTotal, ProcessingCompletedTotal, ProcessingFailedTotal
	sPD := ProcessingDuration
	sS3, sKP, sIH, sRL, sDLQ, sLag, sQU := S3ErrorsTotal, KafkaPublishFailures, IdempotencyHitsTotal,
		ReconciliationTotal, DeadLetterQueueTotal, KafkaConsumerLag, QuotaUsageBytes

	UploadStartedTotal, UploadCompletedTotal, UploadFailedTotal, UploadBytesTotal = nil, nil, nil, nil
	ActiveUploads = nil
	ProcessingStartedTotal, ProcessingCompletedTotal, ProcessingFailedTotal = nil, nil, nil
	ProcessingDuration = nil
	S3ErrorsTotal, KafkaPublishFailures, IdempotencyHitsTotal = nil, nil, nil
	ReconciliationTotal, DeadLetterQueueTotal, KafkaConsumerLag, QuotaUsageBytes = nil, nil, nil, nil

	t.Cleanup(func() {
		UploadStartedTotal, UploadCompletedTotal, UploadFailedTotal, UploadBytesTotal = sUS, sUC, sUF, sUB
		ActiveUploads = sAU
		ProcessingStartedTotal, ProcessingCompletedTotal, ProcessingFailedTotal = sPS, sPC, sPF
		ProcessingDuration = sPD
		S3ErrorsTotal, KafkaPublishFailures, IdempotencyHitsTotal = sS3, sKP, sIH
		ReconciliationTotal, DeadLetterQueueTotal, KafkaConsumerLag, QuotaUsageBytes = sRL, sDLQ, sLag, sQU
	})

	RecordUploadStarted("video/mp4")
	RecordUploadCompleted("video/mp4")
	RecordUploadFailed("quota_exceeded")
	RecordUploadBytes("video", 100)
	IncActiveUploads()
	DecActiveUploads()
	SetQuotaUsage("user-1", 1)
	RecordProcessingStarted("transcode")
	RecordProcessingCompleted("transcode")
	RecordProcessingFailed("transcode")
	ObserveProcessingDuration("transcode", time.Second)
	RecordS3Error("PutObject")
	RecordKafkaPublishFailure("media.ready")
	RecordIdempotencyHit("upload")
	RecordReconciliation("requeue")
}
