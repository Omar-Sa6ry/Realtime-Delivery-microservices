package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type QueryRepository struct {
	db *sql.DB
}

func NewQueryRepository(client *Client) *QueryRepository {
	return &QueryRepository{db: client.DB()}
}

var _ ports.AnalyticsQueryRepository = (*QueryRepository)(nil)

func bucketFn(g ports.AnalyticsGranularity) string {
	switch g {
	case ports.GranularityHour:
		return "toStartOfHour"
	case ports.GranularityWeek:
		return "toStartOfWeek"
	case ports.GranularityMonth:
		return "toStartOfMonth"
	default:
		return "toStartOfDay"
	}
}

func paginate(total, page, limit int) ports.Pagination {
	if page < 1 {
		page = 1
	}
	p := ports.Pagination{TotalItems: total, CurrentPage: page}
	if page*limit < total {
		np := page + 1
		p.NextPage = &np
	}
	return p
}

func (r *QueryRepository) PlatformOverview(ctx context.Context, tr ports.TimeRange) (*ports.PlatformOverview, error) {
	if err := tr.Validate(); err != nil {
		return nil, err
	}
	out := &ports.PlatformOverview{DataAsOf: time.Now().UTC()}

	const deliveries = `SELECT countIf(event_type = 'delivery.created'), countIf(event_type = 'delivery.completed'), countIf(event_type = 'delivery.cancelled'), countIf(event_type = 'delivery.failed') FROM fact_delivery_events WHERE occurred_at BETWEEN ? AND ?`
	if err := r.db.QueryRowContext(ctx, deliveries, tr.From, tr.To).Scan(&out.TotalDeliveries, &out.CompletedDeliveries, &out.CancelledDeliveries, &out.FailedDeliveries); err != nil {
		return nil, fmt.Errorf("platform deliveries: %w", err)
	}
	if out.TotalDeliveries > 0 {
		out.CompletionRate = float64(out.CompletedDeliveries) / float64(out.TotalDeliveries)
	}

	const durations = `SELECT avg(total_duration_seconds) FROM fact_delivery_completed FINAL WHERE completed_at BETWEEN ? AND ?`
	var avgDur sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, durations, tr.From, tr.To).Scan(&avgDur); err != nil {
		return nil, fmt.Errorf("platform durations: %w", err)
	}
	out.AverageDeliveryDurationS = avgDur.Float64

	const money = `SELECT toString(sumIf(amount, transaction_type = 'CAPTURE')), toString(sumIf(amount, transaction_type = 'REFUND')) FROM fact_payment_transactions WHERE occurred_at BETWEEN ? AND ?`
	var captured, refunded sql.NullString
	if err := r.db.QueryRowContext(ctx, money, tr.From, tr.To).Scan(&captured, &refunded); err != nil {
		return nil, fmt.Errorf("platform payments: %w", err)
	}
	out.PaymentCapturedAmount = captured.String
	out.RefundedAmount = refunded.String

	const acceptance = `SELECT countIf(assignment_result = 'ACCEPTED') / nullIf(count(), 0) FROM fact_driver_assignments WHERE offered_at BETWEEN ? AND ?`
	var rate sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, acceptance, tr.From, tr.To).Scan(&rate); err != nil {
		return nil, fmt.Errorf("platform acceptance: %w", err)
	}
	out.DriverAcceptanceRate = rate.Float64
	return out, nil
}

func (r *QueryRepository) DeliveryAnalytics(ctx context.Context, f ports.DeliveryAnalyticsFilter) (*ports.DeliveryAnalytics, error) {
	if err := f.Range.Validate(); err != nil {
		return nil, err
	}
	out := &ports.DeliveryAnalytics{DataAsOf: time.Now().UTC()}
	cityFilter := ""
	args := []any{f.Range.From, f.Range.To}
	if f.CityID != "" {
		cityFilter = " AND city_id = ?"
		args = append(args, f.CityID)
	}
	if f.DriverID != "" {
		cityFilter += " AND driver_id = ?"
		args = append(args, f.DriverID)
	}

	totals := `SELECT countIf(event_type = 'delivery.created'), countIf(event_type = 'delivery.completed'), countIf(event_type = 'delivery.cancelled'), countIf(event_type = 'delivery.failed') FROM fact_delivery_events WHERE occurred_at BETWEEN ? AND ?` + cityFilter
	if err := r.db.QueryRowContext(ctx, totals, args...).Scan(&out.Total, &out.Completed, &out.Cancelled, &out.Failed); err != nil {
		return nil, fmt.Errorf("delivery totals: %w", err)
	}
	if out.Total > 0 {
		out.CompletionRate = float64(out.Completed) / float64(out.Total)
	}

	stats := `SELECT avg(total_duration_seconds), quantile(0.5)(total_duration_seconds), quantile(0.95)(total_duration_seconds), quantile(0.99)(total_duration_seconds), avg(assignment_duration_s) FROM fact_delivery_completed FINAL WHERE created_at BETWEEN ? AND ?` + cityFilter
	var avg, p50, p95, p99, avgAssign sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, stats, args...).Scan(&avg, &p50, &p95, &p99, &avgAssign); err != nil {
		return nil, fmt.Errorf("delivery stats: %w", err)
	}
	out.AverageDurationSeconds = avg.Float64
	out.P50DurationSeconds = p50.Float64
	out.P95DurationSeconds = p95.Float64
	out.P99DurationSeconds = p99.Float64
	out.AverageAssignmentTimeS = avgAssign.Float64

	buckets := fmt.Sprintf(`SELECT %[1]s(occurred_at) AS bucket, countIf(event_type = 'delivery.created'), countIf(event_type = 'delivery.completed'), countIf(event_type = 'delivery.cancelled'), countIf(event_type = 'delivery.failed') FROM fact_delivery_events WHERE occurred_at BETWEEN ? AND ?%[2]s GROUP BY bucket ORDER BY bucket`, bucketFn(f.Range.Granularity), cityFilter)
	rows, err := r.db.QueryContext(ctx, buckets, args...)
	if err != nil {
		return nil, fmt.Errorf("delivery buckets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b ports.DeliveryMetricBucket
		if err := rows.Scan(&b.Bucket, &b.Total, &b.Completed, &b.Cancelled, &b.Failed); err != nil {
			return nil, fmt.Errorf("scan delivery bucket: %w", err)
		}
		out.Buckets = append(out.Buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery buckets: %w", err)
	}

	durBuckets := fmt.Sprintf(`SELECT %s(created_at) AS bucket, avg(total_duration_seconds) FROM fact_delivery_completed FINAL WHERE created_at BETWEEN ? AND ?%s GROUP BY bucket`, bucketFn(f.Range.Granularity), cityFilter)
	avgByBucket := map[time.Time]float64{}
	drows, err := r.db.QueryContext(ctx, durBuckets, args...)
	if err != nil {
		return nil, fmt.Errorf("delivery duration buckets: %w", err)
	}
	for drows.Next() {
		var bucket time.Time
		var v sql.NullFloat64
		if err := drows.Scan(&bucket, &v); err != nil {
			drows.Close()
			return nil, fmt.Errorf("scan duration bucket: %w", err)
		}
		avgByBucket[bucket] = v.Float64
	}
	drows.Close()
	if err := drows.Err(); err != nil {
		return nil, fmt.Errorf("iterate duration buckets: %w", err)
	}
	for i := range out.Buckets {
		out.Buckets[i].AvgDurationSeconds = avgByBucket[out.Buckets[i].Bucket]
	}
	return out, nil
}

func scanDriverAnalytics(row *sql.Row, driverID string) (*ports.DriverAnalytics, error) {
	out := &ports.DriverAnalytics{DriverID: driverID, DataAsOf: time.Now().UTC()}
	if err := row.Scan(&out.Offers, &out.Accepted, &out.Rejected, &out.Expired, &out.AverageResponseTimeMs); err != nil {
		return nil, err
	}
	if out.Offers > 0 {
		out.AcceptanceRate = float64(out.Accepted) / float64(out.Offers)
	}
	return out, nil
}

func (r *QueryRepository) DriverAnalytics(ctx context.Context, f ports.DriverAnalyticsFilter) (*ports.DriverAnalytics, error) {
	if err := f.Range.Validate(); err != nil {
		return nil, err
	}
	const q = `SELECT count(), countIf(assignment_result = 'ACCEPTED'), countIf(assignment_result = 'REJECTED'), countIf(assignment_result = 'EXPIRED'), avg(response_time_ms) FROM fact_driver_assignments WHERE offered_at BETWEEN ? AND ? AND (? = '' OR driver_id = ?)`
	out, err := scanDriverAnalytics(r.db.QueryRowContext(ctx, q, f.Range.From, f.Range.To, f.DriverID, f.DriverID), f.DriverID)
	if err != nil {
		return nil, fmt.Errorf("driver analytics: %w", err)
	}
	const completed = `SELECT count() FROM fact_delivery_completed FINAL WHERE driver_id = ? AND completed_at BETWEEN ? AND ?`
	if err := r.db.QueryRowContext(ctx, completed, f.DriverID, f.Range.From, f.Range.To).Scan(&out.CompletedDeliveries); err != nil {
		return nil, fmt.Errorf("driver completed: %w", err)
	}
	return out, nil
}

func (r *QueryRepository) TopDrivers(ctx context.Context, tr ports.TimeRange, limit int) (*ports.DriverAnalyticsPage, error) {
	if err := tr.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	const q = `SELECT driver_id, count(), countIf(assignment_result = 'ACCEPTED'), countIf(assignment_result = 'REJECTED'), countIf(assignment_result = 'EXPIRED'), avg(response_time_ms) FROM fact_driver_assignments WHERE offered_at BETWEEN ? AND ? GROUP BY driver_id HAVING count() > 0 ORDER BY countIf(assignment_result = 'ACCEPTED') / count() DESC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, q, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("top drivers: %w", err)
	}
	defer rows.Close()
	page := &ports.DriverAnalyticsPage{}
	now := time.Now().UTC()
	driverIDs := make([]string, 0, limit)
	for rows.Next() {
		var d ports.DriverAnalytics
		if err := rows.Scan(&d.DriverID, &d.Offers, &d.Accepted, &d.Rejected, &d.Expired, &d.AverageResponseTimeMs); err != nil {
			return nil, fmt.Errorf("scan top driver: %w", err)
		}
		if d.Offers > 0 {
			d.AcceptanceRate = float64(d.Accepted) / float64(d.Offers)
		}
		d.DataAsOf = now
		page.Items = append(page.Items, d)
		driverIDs = append(driverIDs, d.DriverID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top drivers: %w", err)
	}
	const total = `SELECT count(DISTINCT driver_id) FROM fact_driver_assignments WHERE offered_at BETWEEN ? AND ?`
	var totalDrivers int
	if err := r.db.QueryRowContext(ctx, total, tr.From, tr.To).Scan(&totalDrivers); err != nil {
		return nil, fmt.Errorf("top drivers total: %w", err)
	}
	page.Pagination = paginate(totalDrivers, 1, limit)

	// Single batch query for completed deliveries — avoids N+1 per driver.
	if len(driverIDs) > 0 {
		completedMap, err := r.batchCompletedByDriver(ctx, driverIDs, tr)
		if err != nil {
			return nil, err
		}
		for i := range page.Items {
			page.Items[i].CompletedDeliveries = completedMap[page.Items[i].DriverID]
		}
	}
	return page, nil
}

func (r *QueryRepository) batchCompletedByDriver(ctx context.Context, driverIDs []string, tr ports.TimeRange) (map[string]int64, error) {
	if len(driverIDs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(driverIDs)+2)
	args = append(args, tr.From, tr.To)
	placeholders := make([]byte, 0, len(driverIDs)*2)
	for i, id := range driverIDs {
		args = append(args, id)
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
	}
	q := `SELECT driver_id, count() FROM fact_delivery_completed FINAL WHERE completed_at BETWEEN ? AND ? AND driver_id IN (` + string(placeholders) + `) GROUP BY driver_id`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("batch completed by driver: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int64, len(driverIDs))
	for rows.Next() {
		var driverID string
		var n int64
		if err := rows.Scan(&driverID, &n); err != nil {
			return nil, fmt.Errorf("scan batch completed: %w", err)
		}
		out[driverID] = n
	}
	return out, rows.Err()
}

func (r *QueryRepository) PaymentAnalytics(ctx context.Context, f ports.PaymentAnalyticsFilter) (*ports.PaymentAnalytics, error) {
	if err := f.Range.Validate(); err != nil {
		return nil, err
	}
	out := &ports.PaymentAnalytics{DataAsOf: time.Now().UTC()}
	providerFilter := ""
	args := []any{f.Range.From, f.Range.To}
	if f.Provider != "" {
		providerFilter = " AND provider = ?"
		args = append(args, f.Provider)
	}
	q := `SELECT countIf(transaction_type = 'AUTHORIZATION'), countIf(transaction_type = 'AUTHORIZATION' AND status IN ('AUTHORIZED', 'CAPTURED')) / nullIf(countIf(transaction_type = 'AUTHORIZATION'), 0), countIf(transaction_type = 'CAPTURE'), toString(sumIf(amount, transaction_type = 'CAPTURE')), countIf(transaction_type = 'REFUND'), toString(sumIf(amount, transaction_type = 'REFUND')), avg(provider_latency_ms) FROM fact_payment_transactions WHERE occurred_at BETWEEN ? AND ?` + providerFilter
	var authCount sql.NullInt64
	var authRate, avgLatency sql.NullFloat64
	var captured, refunded sql.NullString
	var captureCount, refundCount sql.NullInt64
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&authCount, &authRate, &captureCount, &captured, &refundCount, &refunded, &avgLatency); err != nil {
		return nil, fmt.Errorf("payment analytics: %w", err)
	}
	out.AuthorizationCount = authCount.Int64
	out.AuthorizationSuccessRate = authRate.Float64
	out.CaptureCount = captureCount.Int64
	out.CapturedAmount = captured.String
	out.RefundCount = refundCount.Int64
	out.RefundedAmount = refunded.String
	out.AverageProviderLatencyMs = avgLatency.Float64
	if out.CaptureCount > 0 {
		out.RefundRate = float64(out.RefundCount) / float64(out.CaptureCount)
	}
	return out, nil
}

func (r *QueryRepository) RawEvents(ctx context.Context, page, limit int, eventType string, from, to *time.Time) (*ports.RawEventsPage, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	where := "WHERE 1 = 1"
	args := []any{}
	if eventType != "" {
		where += " AND event_type = ?"
		args = append(args, eventType)
	}
	if from != nil {
		where += " AND occurred_at >= ?"
		args = append(args, *from)
	}
	if to != nil {
		where += " AND occurred_at <= ?"
		args = append(args, *to)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT count() FROM raw_events "+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("raw events total: %w", err)
	}
	offset := (page - 1) * limit
	q := `SELECT event_id, event_type, event_version, aggregate_type, aggregate_id, producer, occurred_at, ingested_at, correlation_id, causation_id, source_topic, source_partition, source_offset, payload_json FROM raw_events ` + where + ` ORDER BY occurred_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, q, append(args, limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("raw events: %w", err)
	}
	defer rows.Close()
	out := &ports.RawEventsPage{Pagination: paginate(total, page, limit)}
	for rows.Next() {
		e := &domain.RawEventLanding{}
		var version uint16
		var partition uint32
		var offset64 uint64
		if err := rows.Scan(&e.EventID, &e.EventType, &version, &e.AggregateType, &e.AggregateID, &e.Producer, &e.OccurredAt, &e.IngestedAt, &e.CorrelationID, &e.CausationID, &e.SourceTopic, &partition, &offset64, &e.PayloadJSON); err != nil {
			return nil, fmt.Errorf("scan raw event: %w", err)
		}
		e.EventVersion = domain.EventVersion(version)
		e.SourcePartition = int32(partition)
		e.SourceOffset = int64(offset64)
		out.Items = append(out.Items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate raw events: %w", err)
	}
	return out, nil
}

func (r *QueryRepository) DataQualityIssues(ctx context.Context, page, limit int, severity string) (*ports.DataQualityIssuesPage, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	where := "WHERE 1 = 1"
	args := []any{}
	if severity != "" {
		where += " AND severity = ?"
		args = append(args, severity)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT count() FROM analytics_data_quality_issues "+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("data quality total: %w", err)
	}
	offset := (page - 1) * limit
	q := `SELECT issue_id, event_id, issue_type, aggregate_type, aggregate_id, detected_at, severity, details, resolved_at FROM analytics_data_quality_issues ` + where + ` ORDER BY detected_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, q, append(args, limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("data quality issues: %w", err)
	}
	defer rows.Close()
	out := &ports.DataQualityIssuesPage{Pagination: paginate(total, page, limit)}
	for rows.Next() {
		var d domain.DataQualityIssue
		if err := rows.Scan(&d.IssueID, &d.EventID, &d.IssueType, &d.AggregateType, &d.AggregateID, &d.DetectedAt, &d.Severity, &d.Details, &d.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan data quality issue: %w", err)
		}
		out.Items = append(out.Items, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate data quality issues: %w", err)
	}
	return out, nil
}

func (r *QueryRepository) Close() error { return nil }

var _ ports.ReconciliationQueries = (*QueryRepository)(nil)

func (r *QueryRepository) MaxRawOccurredAt(ctx context.Context) (time.Time, error) {
	var max time.Time
	if err := r.db.QueryRowContext(ctx, "SELECT max(occurred_at) FROM raw_events").Scan(&max); err != nil {
		return time.Time{}, fmt.Errorf("max occurred_at: %w", err)
	}
	return max, nil
}

func (r *QueryRepository) CountRawEvents(ctx context.Context, from, to time.Time) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT count() FROM raw_events WHERE occurred_at BETWEEN ? AND ?", from, to).Scan(&n); err != nil {
		return 0, fmt.Errorf("count raw events: %w", err)
	}
	return n, nil
}

func (r *QueryRepository) CountFactDeliveryEvents(ctx context.Context, from, to time.Time) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT count() FROM fact_delivery_events WHERE occurred_at BETWEEN ? AND ?", from, to).Scan(&n); err != nil {
		return 0, fmt.Errorf("count delivery facts: %w", err)
	}
	return n, nil
}

func (r *QueryRepository) CountDataQualityByType(ctx context.Context, issueType string, from, to time.Time) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT count() FROM analytics_data_quality_issues WHERE issue_type = ? AND detected_at BETWEEN ? AND ?",
		issueType, from, to).Scan(&n); err != nil {
		return 0, fmt.Errorf("count data quality: %w", err)
	}
	return n, nil
}

func (r *QueryRepository) DistinctRawEventTypes(ctx context.Context, from, to time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT DISTINCT event_type FROM raw_events WHERE occurred_at BETWEEN ? AND ?", from, to)
	if err != nil {
		return nil, fmt.Errorf("distinct event types: %w", err)
	}
	defer rows.Close()
	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan event type: %w", err)
		}
		types = append(types, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event types: %w", err)
	}
	return types, nil
}

func (r *QueryRepository) CountOpenIssuesBySeverity(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT severity, count() FROM analytics_data_quality_issues WHERE isNull(resolved_at) GROUP BY severity")
	if err != nil {
		return nil, fmt.Errorf("open issues: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var severity string
		var n int64
		if err := rows.Scan(&severity, &n); err != nil {
			return nil, fmt.Errorf("scan open issues: %w", err)
		}
		out[severity] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate open issues: %w", err)
	}
	return out, nil
}
