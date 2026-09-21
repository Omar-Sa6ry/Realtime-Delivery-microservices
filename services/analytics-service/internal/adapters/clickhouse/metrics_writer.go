package clickhouse

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type MetricsWriter struct {
	inner   ports.ClickHouseWriter
	metrics *observability.Metrics
}

func NewMetricsWriter(inner ports.ClickHouseWriter, metrics *observability.Metrics) *MetricsWriter {
	return &MetricsWriter{inner: inner, metrics: metrics}
}

var _ ports.ClickHouseWriter = (*MetricsWriter)(nil)

func (w *MetricsWriter) record(table string, rows int, start time.Time, err error) {
	if w.metrics == nil {
		return
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	w.metrics.RecordBatchFlush(table, status, rows, time.Since(start).Seconds())
}

func (w *MetricsWriter) WriteRawEvents(ctx context.Context, rows []*domain.RawEventLanding) error {
	start := time.Now()
	err := w.inner.WriteRawEvents(ctx, rows)
	w.record("raw_events", len(rows), start, err)
	return err
}

func (w *MetricsWriter) WriteDeliveryEvents(ctx context.Context, rows []*domain.FactDeliveryEvent) error {
	start := time.Now()
	err := w.inner.WriteDeliveryEvents(ctx, rows)
	w.record("fact_delivery_events", len(rows), start, err)
	return err
}

func (w *MetricsWriter) WriteDeliveryCompleted(ctx context.Context, rows []*domain.FactDeliveryCompleted) error {
	start := time.Now()
	err := w.inner.WriteDeliveryCompleted(ctx, rows)
	w.record("fact_delivery_completed", len(rows), start, err)
	return err
}

func (w *MetricsWriter) WriteDriverAssignments(ctx context.Context, rows []*domain.FactDriverAssignment) error {
	start := time.Now()
	err := w.inner.WriteDriverAssignments(ctx, rows)
	w.record("fact_driver_assignments", len(rows), start, err)
	return err
}

func (w *MetricsWriter) WritePaymentTransactions(ctx context.Context, rows []*domain.FactPaymentTransaction) error {
	start := time.Now()
	err := w.inner.WritePaymentTransactions(ctx, rows)
	w.record("fact_payment_transactions", len(rows), start, err)
	return err
}

func (w *MetricsWriter) WriteNotificationEvents(ctx context.Context, rows []*domain.FactNotificationEvent) error {
	start := time.Now()
	err := w.inner.WriteNotificationEvents(ctx, rows)
	w.record("fact_notification_events", len(rows), start, err)
	return err
}

func (w *MetricsWriter) WriteDataQualityIssues(ctx context.Context, rows []*domain.DataQualityIssue) error {
	start := time.Now()
	err := w.inner.WriteDataQualityIssues(ctx, rows)
	w.record("analytics_data_quality_issues", len(rows), start, err)
	return err
}

func (w *MetricsWriter) Close() error {
	return w.inner.Close()
}
