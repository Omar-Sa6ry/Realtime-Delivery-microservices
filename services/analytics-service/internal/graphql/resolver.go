package graphql

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/analytics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type Resolver struct {
	platform *analytics.PlatformOverviewService
	delivery *analytics.DeliveryAnalyticsService
	driver   *analytics.DriverAnalyticsService
	payment  *analytics.PaymentAnalyticsService
	repo     ports.AnalyticsQueryRepository
}

func NewResolver(
	platform *analytics.PlatformOverviewService,
	delivery *analytics.DeliveryAnalyticsService,
	driver *analytics.DriverAnalyticsService,
	payment *analytics.PaymentAnalyticsService,
	repo ports.AnalyticsQueryRepository,
) *Resolver {
	return &Resolver{platform: platform, delivery: delivery, driver: driver, payment: payment, repo: repo}
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func okEnvelope(lang, msgKey string, data any) map[string]any {
	return map[string]any{
		"success":    true,
		"statusCode": 200,
		"message":    i18n.T(lang, msgKey),
		"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		"data":       data,
	}
}

func errEnvelope(lang string, statusCode int, err error) map[string]any {
	msg := i18n.T(lang, "error.internal")
	if err != nil && err.Error() != "" {
		msg = err.Error()
	}
	return map[string]any{
		"success":    false,
		"statusCode": statusCode,
		"message":    msg,
		"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		"data":       nil,
	}
}

func (r *Resolver) ServiceInfo(ctx context.Context) map[string]any {
	lang := i18n.FromContext(ctx)
	return okEnvelope(lang, "server.healthy", map[string]any{
		"name": "analytics-service", "version": "1.0.0", "status": "healthy",
	})
}

func (r *Resolver) PlatformOverview(ctx context.Context, tr ports.TimeRange, scope string) map[string]any {
	lang := i18n.FromContext(ctx)
	out, err := r.platform.GetPlatformOverview(ctx, tr, scope)
	if err != nil {
		return errEnvelope(lang, 400, err)
	}
	return okEnvelope(lang, "platform.overview.found", map[string]any{
		"totalDeliveries": out.TotalDeliveries, "completedDeliveries": out.CompletedDeliveries,
		"cancelledDeliveries": out.CancelledDeliveries, "failedDeliveries": out.FailedDeliveries,
		"completionRate": out.CompletionRate, "averageDeliveryDurationSeconds": out.AverageDeliveryDurationS,
		"paymentCapturedAmount": out.PaymentCapturedAmount, "refundedAmount": out.RefundedAmount,
		"driverAcceptanceRate": out.DriverAcceptanceRate, "dataAsOf": rfc3339(out.DataAsOf),
	})
}

func (r *Resolver) DeliveryAnalytics(ctx context.Context, f ports.DeliveryAnalyticsFilter, scope string) map[string]any {
	lang := i18n.FromContext(ctx)
	out, err := r.delivery.GetDeliveryAnalytics(ctx, f, scope)
	if err != nil {
		return errEnvelope(lang, 400, err)
	}
	buckets := make([]any, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		buckets = append(buckets, map[string]any{
			"bucket": b.Bucket.UTC().Format(time.RFC3339), "total": b.Total,
			"completed": b.Completed, "cancelled": b.Cancelled, "failed": b.Failed,
			"avgDurationSeconds": b.AvgDurationSeconds,
		})
	}
	return okEnvelope(lang, "delivery.analytics.found", map[string]any{
		"total": out.Total, "completed": out.Completed, "cancelled": out.Cancelled, "failed": out.Failed,
		"completionRate": out.CompletionRate, "averageDurationSeconds": out.AverageDurationSeconds,
		"p50DurationSeconds": out.P50DurationSeconds, "p95DurationSeconds": out.P95DurationSeconds,
		"p99DurationSeconds": out.P99DurationSeconds, "averageAssignmentTimeSeconds": out.AverageAssignmentTimeS,
		"buckets": buckets, "dataAsOf": rfc3339(out.DataAsOf),
	})
}

func (r *Resolver) driverShape(ctx context.Context, d ports.DriverAnalytics) map[string]any {
	var driver any
	if d.DriverID != "" {
		if stub, err := LoadDriver(ctx, d.DriverID); err == nil {
			driver = map[string]any{"id": stub.ID}
		} else {
			driver = map[string]any{"id": d.DriverID}
		}
	}
	return map[string]any{
		"driverId": d.DriverID, "driver": driver,
		"offers": d.Offers, "accepted": d.Accepted, "rejected": d.Rejected, "expired": d.Expired,
		"acceptanceRate": d.AcceptanceRate, "averageResponseTimeMs": d.AverageResponseTimeMs,
		"completedDeliveries": d.CompletedDeliveries, "dataAsOf": rfc3339(d.DataAsOf),
	}
}

func (r *Resolver) DriverAnalytics(ctx context.Context, f ports.DriverAnalyticsFilter, scope string) map[string]any {
	lang := i18n.FromContext(ctx)
	out, err := r.driver.GetDriverAnalytics(ctx, f, scope)
	if err != nil {
		return errEnvelope(lang, 400, err)
	}
	return okEnvelope(lang, "driver.analytics.found", r.driverShape(ctx, *out))
}

func (r *Resolver) TopDrivers(ctx context.Context, tr ports.TimeRange, limit int, scope string) map[string]any {
	lang := i18n.FromContext(ctx)
	page, err := r.driver.GetTopDrivers(ctx, tr, limit, scope)
	if err != nil {
		return errEnvelope(lang, 400, err)
	}
	items := make([]any, 0, len(page.Items))
	for _, d := range page.Items {
		items = append(items, r.driverShape(ctx, d))
	}
	return okEnvelope(lang, "driver.top.found", map[string]any{
		"paginationInfo": map[string]any{
			"totalItems": page.Pagination.TotalItems, "currentPage": page.Pagination.CurrentPage, "nextPage": page.Pagination.NextPage,
		},
		"items": items,
	})
}

func (r *Resolver) PaymentAnalytics(ctx context.Context, f ports.PaymentAnalyticsFilter, scope string) map[string]any {
	lang := i18n.FromContext(ctx)
	out, err := r.payment.GetPaymentAnalytics(ctx, f, scope)
	if err != nil {
		return errEnvelope(lang, 400, err)
	}
	return okEnvelope(lang, "payment.analytics.found", map[string]any{
		"authorizationCount": out.AuthorizationCount, "authorizationSuccessRate": out.AuthorizationSuccessRate,
		"captureCount": out.CaptureCount, "capturedAmount": out.CapturedAmount,
		"refundCount": out.RefundCount, "refundedAmount": out.RefundedAmount, "refundRate": out.RefundRate,
		"averageProviderLatencyMs": out.AverageProviderLatencyMs, "dataAsOf": rfc3339(out.DataAsOf),
	})
}

func (r *Resolver) RawAnalyticsEvents(ctx context.Context, page, limit int, eventType string, from, to *time.Time) map[string]any {
	lang := i18n.FromContext(ctx)
	p, err := r.repo.RawEvents(ctx, page, limit, eventType, from, to)
	if err != nil {
		return errEnvelope(lang, 500, err)
	}
	items := make([]any, 0, len(p.Items))
	for _, e := range p.Items {
		items = append(items, map[string]any{
			"eventId": e.EventID, "eventType": e.EventType, "eventVersion": int(e.EventVersion),
			"aggregateType": e.AggregateType, "aggregateId": e.AggregateID, "producer": e.Producer,
			"occurredAt": rfc3339(e.OccurredAt), "ingestedAt": rfc3339(e.IngestedAt),
			"correlationId": e.CorrelationID, "sourceTopic": e.SourceTopic,
		})
	}
	return okEnvelope(lang, "raw.events.found", map[string]any{
		"paginationInfo": map[string]any{
			"totalItems": p.Pagination.TotalItems, "currentPage": p.Pagination.CurrentPage, "nextPage": p.Pagination.NextPage,
		},
		"items": items,
	})
}

func (r *Resolver) DataQualityIssues(ctx context.Context, page, limit int, severity string) map[string]any {
	lang := i18n.FromContext(ctx)
	p, err := r.repo.DataQualityIssues(ctx, page, limit, severity)
	if err != nil {
		return errEnvelope(lang, 500, err)
	}
	items := make([]any, 0, len(p.Items))
	for _, d := range p.Items {
		items = append(items, map[string]any{
			"issueId": d.IssueID, "eventId": d.EventID, "issueType": d.IssueType,
			"aggregateType": d.AggregateType, "aggregateId": d.AggregateID,
			"detectedAt": rfc3339(d.DetectedAt), "severity": string(d.Severity), "details": d.Details,
		})
	}
	return okEnvelope(lang, "data.quality.issues.found", map[string]any{
		"paginationInfo": map[string]any{
			"totalItems": p.Pagination.TotalItems, "currentPage": p.Pagination.CurrentPage, "nextPage": p.Pagination.NextPage,
		},
		"items": items,
	})
}
